package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"github.com/ebogdum/keramos/v2/internal/release"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// newMetricsCommand exposes `keramos metrics <release>` — sample CPU/memory
// usage of every pod in the release at `--interval` for `--duration`, then
// emit min/avg/p50/p95/max plus a recommendation for requests and limits.
//
// Reads from the metrics.k8s.io/v1beta1 API (same source kubectl top uses).
// If metrics-server is not installed the first sample errors with a clear
// message rather than producing meaningless zeros.
func newMetricsCommand() *cobra.Command {
	var (
		duration  time.Duration
		interval  time.Duration
		output    string
		recommend bool
	)
	cmd := &cobra.Command{
		Use:   "metrics <release-name>",
		Short: "Sample CPU/memory usage over time and recommend requests/limits",
		Long: `Sample every pod in a release through the metrics.k8s.io API at
--interval for --duration, then print per-container statistics:

  CONTAINER       SAMPLES   CPU(min/avg/p50/p95/max)   MEM(min/avg/p50/p95/max)

With --recommend keramos also prints a values-yaml-shaped block of suggested
resources.requests and resources.limits, computed as:

  requests.cpu    = round-up(p50 * 1.1)
  requests.memory = round-up(p50 * 1.2)
  limits.cpu      = round-up(p95 * 1.5)
  limits.memory   = round-up(p95 * 1.5)

These are starting points — verify against your application's failure
modes before applying.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			ns := namespace
			if "" == ns {
				ns = client.Namespace()
			}
			storage := release.NewSecretStorage(client.Clientset(), ns)
			rel, err := storage.Last(args[0])
			if nil != err {
				return err
			}

			// Resource names from the release manifest become the
			// pod-name prefix filter — kubernetes' built-in pod-naming
			// rules ensure every pod managed by a Deployment/StatefulSet/
			// DaemonSet/Job/CronJob has a name starting with the parent
			// resource's name. We don't rely on app.kubernetes.io/instance
			// because keramos packages don't always set it.
			prefixes := extractPodControllerNames(rel.Manifest)
			fmt.Fprintf(cmd.OutOrStdout(),
				"Sampling %s pods every %s for %s (matching prefixes: %v)\n",
				rel.Name, interval, duration, prefixes)

			ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Second)
			defer cancel()
			samples, err := sampleMetrics(ctx, client, ns, prefixes, interval, duration)
			if nil != err {
				return err
			}
			if 0 == len(samples) {
				fmt.Fprintln(cmd.OutOrStdout(), "no samples collected (no pods matching the release labels?)")
				return nil
			}
			stats := computeStats(samples)

			if "json" == output {
				data, _ := json.MarshalIndent(stats, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
			} else {
				fmt.Fprint(cmd.OutOrStdout(), formatStatsTable(stats))
			}
			if recommend {
				fmt.Fprint(cmd.OutOrStdout(), formatRecommendation(stats))
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&duration, "duration", 30*time.Second, "total sampling window")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "interval between samples")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json")
	cmd.Flags().BoolVar(&recommend, "recommend", false, "also print suggested resources.requests/limits values-block")
	return cmd
}

// extractPodControllerNames walks the rendered manifest and returns the
// metadata.name of every kind that produces pods. Pods in a release are
// named with one of these as their prefix.
func extractPodControllerNames(manifest string) []string {
	docs, err := splitYAMLForMetrics(manifest)
	if nil != err {
		return nil
	}
	out := make([]string, 0, 4)
	seen := map[string]bool{}
	podControllers := map[string]bool{
		"Pod": true, "Deployment": true, "StatefulSet": true,
		"DaemonSet": true, "Job": true, "CronJob": true, "ReplicaSet": true,
	}
	for _, d := range docs {
		kind, _ := d["kind"].(string)
		if !podControllers[kind] {
			continue
		}
		meta, _ := d["metadata"].(map[string]any)
		name, _ := meta["name"].(string)
		if "" != name && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// metricSample is a single CPU/memory measurement for one container.
type metricSample struct {
	Pod        string
	Container  string
	CPU_mCores int64 // millicores
	Mem_Bytes  int64
	When       time.Time
}

func sampleMetrics(ctx context.Context, client kube.KubeClient, namespace string, prefixes []string, interval, duration time.Duration) ([]metricSample, error) {
	samples := make([]metricSample, 0, 64)
	deadline := time.Now().Add(duration)
	tick := time.NewTicker(interval)
	defer tick.Stop()

	if err := pollOnce(ctx, client, namespace, prefixes, &samples); nil != err {
		return nil, err
	}
	consecutiveFailures := 0
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return samples, nil
		case <-tick.C:
			if err := pollOnce(ctx, client, namespace, prefixes, &samples); nil != err {
				consecutiveFailures++
				logger.Warn("metrics poll failed (%d consecutive): %v", consecutiveFailures, err)
				if consecutiveFailures >= 5 {
					return samples, keramoserr.WrapErrorf(keramoserr.ErrKube, err,
						"metrics sampling aborted after %d consecutive poll failures", consecutiveFailures)
				}
			} else {
				consecutiveFailures = 0
			}
		}
	}
	return samples, nil
}

// pollOnce queries the metrics.k8s.io API and keeps every pod whose name
// begins with one of the supplied prefixes. metrics-server returns one
// usage point per pod per query — there is no built-in history. Keramos
// builds history by polling at `--interval`.
func pollOnce(ctx context.Context, client kube.KubeClient, namespace string, prefixes []string, out *[]metricSample) error {
	cs := client.Clientset()
	if nil == cs {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "no Kubernetes client available")
	}
	rest := cs.CoreV1().RESTClient()
	raw, err := rest.Get().
		AbsPath("/apis/metrics.k8s.io/v1beta1/namespaces/" + namespace + "/pods").
		DoRaw(ctx)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrKube, "metrics API not available (is metrics-server installed?)", err)
	}
	var doc struct {
		Items []struct {
			Metadata   metav1.ObjectMeta `json:"metadata"`
			Containers []struct {
				Name  string                         `json:"name"`
				Usage map[corev1.ResourceName]string `json:"usage"`
			} `json:"containers"`
			Timestamp metav1.Time `json:"timestamp"`
		} `json:"items"`
	}
	if jErr := json.Unmarshal(raw, &doc); nil != jErr {
		return keramoserr.WrapError(keramoserr.ErrKube, "parse metrics response", jErr)
	}
	for _, p := range doc.Items {
		if !podMatches(p.Metadata.Name, prefixes) {
			continue
		}
		for _, c := range p.Containers {
			cpuStr := c.Usage[corev1.ResourceCPU]
			memStr := c.Usage[corev1.ResourceMemory]
			cpu, _ := parseCPU(cpuStr)
			mem, _ := parseMemory(memStr)
			*out = append(*out, metricSample{
				Pod: p.Metadata.Name, Container: c.Name,
				CPU_mCores: cpu, Mem_Bytes: mem,
				When: p.Timestamp.Time,
			})
		}
	}
	return nil
}

func podMatches(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if name == p {
			return true
		}
		if len(name) > len(p) && name[:len(p)] == p && name[len(p)] == '-' {
			return true
		}
	}
	return false
}

func splitYAMLForMetrics(manifest string) ([]map[string]any, error) {
	dec := yaml.NewDecoder(strings.NewReader(manifest))
	out := make([]map[string]any, 0)
	for {
		var d map[string]any
		err := dec.Decode(&d)
		if nil != err {
			break
		}
		if 0 < len(d) {
			out = append(out, d)
		}
	}
	return out, nil
}

// parseCPU returns millicores from strings like "12m", "0.5", "1500u" (1500 microcores).
func parseCPU(s string) (int64, error) {
	q, err := resource.ParseQuantity(s)
	if nil != err {
		return 0, err
	}
	// MilliValue: returns the quantity in milli-units (1 core = 1000m).
	return q.MilliValue(), nil
}

// parseMemory returns bytes.
func parseMemory(s string) (int64, error) {
	q, err := resource.ParseQuantity(s)
	if nil != err {
		return 0, err
	}
	return q.Value(), nil
}

type containerStats struct {
	Container string  `json:"container"`
	Samples   int     `json:"samples"`
	CPUMin    int64   `json:"cpuMin"`
	CPUAvg    float64 `json:"cpuAvg"`
	CPUP50    int64   `json:"cpuP50"`
	CPUP95    int64   `json:"cpuP95"`
	CPUMax    int64   `json:"cpuMax"`
	MemMin    int64   `json:"memMin"`
	MemAvg    float64 `json:"memAvg"`
	MemP50    int64   `json:"memP50"`
	MemP95    int64   `json:"memP95"`
	MemMax    int64   `json:"memMax"`
}

func computeStats(samples []metricSample) []containerStats {
	byContainer := make(map[string][]metricSample)
	for _, s := range samples {
		byContainer[s.Container] = append(byContainer[s.Container], s)
	}
	out := make([]containerStats, 0, len(byContainer))
	for c, ss := range byContainer {
		cpus := make([]int64, len(ss))
		mems := make([]int64, len(ss))
		var cpuSum, memSum int64
		for i, s := range ss {
			cpus[i] = s.CPU_mCores
			mems[i] = s.Mem_Bytes
			cpuSum += s.CPU_mCores
			memSum += s.Mem_Bytes
		}
		sort.Slice(cpus, func(i, j int) bool { return cpus[i] < cpus[j] })
		sort.Slice(mems, func(i, j int) bool { return mems[i] < mems[j] })
		n := len(ss)
		stat := containerStats{
			Container: c, Samples: n,
			CPUMin: cpus[0], CPUMax: cpus[n-1],
			CPUAvg: float64(cpuSum) / float64(n),
			CPUP50: cpus[n*50/100], CPUP95: cpus[min(n-1, n*95/100)],
			MemMin: mems[0], MemMax: mems[n-1],
			MemAvg: float64(memSum) / float64(n),
			MemP50: mems[n*50/100], MemP95: mems[min(n-1, n*95/100)],
		}
		out = append(out, stat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Container < out[j].Container })
	return out
}

func formatStatsTable(stats []containerStats) string {
	var buf []byte
	buf = append(buf, "\nCONTAINER                          SAMPLES    CPU(min/avg/p50/p95/max, m)              MEM(min/avg/p50/p95/max)\n"...)
	for _, s := range stats {
		buf = append(buf, fmt.Sprintf("%-32s   %5d      %5d / %6.0f / %5d / %5d / %5d        %s / %s / %s / %s / %s\n",
			truncCol(s.Container, 32), s.Samples,
			s.CPUMin, s.CPUAvg, s.CPUP50, s.CPUP95, s.CPUMax,
			humanBytes(s.MemMin), humanBytesFloat(s.MemAvg),
			humanBytes(s.MemP50), humanBytes(s.MemP95), humanBytes(s.MemMax))...)
	}
	return string(buf)
}

func formatRecommendation(stats []containerStats) string {
	var buf []byte
	buf = append(buf, "\n# suggested resources block (paste into values.yaml):\n"...)
	buf = append(buf, "resources:\n"...)
	for _, s := range stats {
		// Round up to a nicer increment. Helps avoid bid-rounding.
		reqCPU := roundCPUMillis(int64(float64(s.CPUP50) * 1.1))
		limCPU := roundCPUMillis(int64(float64(s.CPUP95) * 1.5))
		reqMem := roundMemBytes(int64(float64(s.MemP50) * 1.2))
		limMem := roundMemBytes(int64(float64(s.MemP95) * 1.5))
		buf = append(buf, fmt.Sprintf("  # container: %s (over %d samples)\n", s.Container, s.Samples)...)
		buf = append(buf, fmt.Sprintf("  requests: {cpu: %dm, memory: %s}\n", reqCPU, humanBytes(reqMem))...)
		buf = append(buf, fmt.Sprintf("  limits:   {cpu: %dm, memory: %s}\n", limCPU, humanBytes(limMem))...)
	}
	return string(buf)
}

func roundCPUMillis(m int64) int64 {
	if 10 > m {
		return 10
	}
	// Round up to the nearest 25m.
	return ((m + 24) / 25) * 25
}

func roundMemBytes(b int64) int64 {
	const Mi = 1024 * 1024
	if Mi > b {
		return Mi
	}
	// Round up to the nearest 16Mi.
	return ((b + 16*Mi - 1) / (16 * Mi)) * 16 * Mi
}

func humanBytes(b int64) string {
	const (
		Ki = 1024
		Mi = 1024 * Ki
		Gi = 1024 * Mi
	)
	switch {
	case Gi <= b:
		return strconv.FormatInt(b/Gi, 10) + "Gi"
	case Mi <= b:
		return strconv.FormatInt(b/Mi, 10) + "Mi"
	case Ki <= b:
		return strconv.FormatInt(b/Ki, 10) + "Ki"
	}
	return strconv.FormatInt(b, 10) + "B"
}

func humanBytesFloat(b float64) string { return humanBytes(int64(b)) }

func truncCol(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
