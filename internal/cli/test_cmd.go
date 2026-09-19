package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

func newTestCommand() *cobra.Command {
	var (
		timeout  time.Duration
		logs     bool
		filter   []string
		parallel int
		retries  int
		output   string
	)

	cmd := &cobra.Command{
		Use:   "test <release-name>",
		Short: "Run tests for a release",
		Long:  "Run test manifests for a deployed release. Tests are stored at install/upgrade time from the package's tests/ directory.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(cmd, args[0], timeout, logs, filter, parallel, retries, output)
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout waiting for test pods")
	cmd.Flags().BoolVar(&logs, "logs", false, "show pod logs after test completes")
	cmd.Flags().StringArrayVar(&filter, "filter", nil, "only run tests whose filename contains the substring (repeatable)")
	cmd.Flags().IntVar(&parallel, "parallel", 1, "number of tests to run concurrently (1 = sequential)")
	cmd.Flags().IntVar(&retries, "retries", 0, "number of retry attempts per test on failure")
	cmd.Flags().StringVarP(&output, "output", "o", "human", "output format: human, junit, json")

	return cmd
}

func runTest(cmd *cobra.Command, releaseName string, timeout time.Duration, showLogs bool, filter []string, parallel, retries int, output string) error {
	if parallel < 1 {
		parallel = 1
	}
	if retries < 0 {
		retries = 0
	}
	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return err
	}

	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
	rel, err := storage.Last(releaseName)
	if nil != err {
		return err
	}
	if release.StatusDeployed != rel.Status {
		return keramoserr.NewErrorf(keramoserr.ErrRelease, "release %s is not deployed (status: %s)", releaseName, rel.Status)
	}

	if 0 == len(rel.Tests) {
		fmt.Fprintln(cmd.OutOrStdout(), "No tests stored for this release.")
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Running tests for release %s (revision %d)...\n", rel.Name, rel.Revision)

	names := make([]string, 0, len(rel.Tests))
	for name := range rel.Tests {
		if 0 < len(filter) && !matchesAnyFilter(name, filter) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if 0 == len(names) {
		fmt.Fprintln(w, "No tests matched the filter.")
		return nil
	}

	ns := rel.Namespace
	clientset := client.Clientset()

	// Run a single test (with retries) and return whether it passed plus any
	// log output to print after the worker pool drains.
	runOne := func(name string) (bool, string) {
		manifest := rel.Tests[name]
		var report strings.Builder
		fmt.Fprintf(&report, "  TEST: %s\n", name)

		var lastErr error
		for attempt := 0; attempt <= retries; attempt++ {
			if attempt > 0 {
				fmt.Fprintf(&report, "    retry %d/%d\n", attempt, retries)
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			passed, logTxt, err := executeOneTest(client, clientset, ns, manifest, timeout, showLogs)
			if nil != err {
				lastErr = err
				continue
			}
			if "" != logTxt {
				fmt.Fprintf(&report, "    LOGS:\n")
				for _, line := range splitLogLines(logTxt) {
					fmt.Fprintf(&report, "      %s\n", line)
				}
			}
			if passed {
				fmt.Fprintf(&report, "    PASS\n")
				logger.Debug("cleaning up test resources for %s", name)
				if delErr := client.DeleteManifests(manifest); nil != delErr {
					logger.Warn("failed to clean up test resources for %s: %v", name, delErr)
				}
				return true, report.String()
			}
			fmt.Fprintf(&report, "    FAIL\n")
		}
		if nil != lastErr {
			fmt.Fprintf(&report, "    ERROR: %v\n", lastErr)
		}
		logger.Debug("cleaning up failed test resources for %s", name)
		if delErr := client.DeleteManifests(manifest); nil != delErr {
			logger.Warn("failed to clean up test resources for %s: %v", name, delErr)
		}
		return false, report.String()
	}

	type result struct {
		name     string
		passed   bool
		report   string
		duration time.Duration
	}
	results := make(chan result, len(names))
	work := make(chan string, len(names))
	for _, name := range names {
		work <- name
	}
	close(work)

	// runOneTimed wraps runOne to capture per-test duration for JUnit output.
	runOneTimed := func(name string) result {
		start := time.Now()
		passed, report := runOne(name)
		return result{
			name:     name,
			passed:   passed,
			report:   report,
			duration: time.Since(start),
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for name := range work {
				results <- runOneTimed(name)
			}
		}()
	}
	wg.Wait()
	close(results)

	collected := make([]result, 0, len(names))
	allPassed := true
	for r := range results {
		collected = append(collected, r)
		if !r.passed {
			allPassed = false
		}
	}

	switch output {
	case "junit":
		junitResults := make([]TestResult, 0, len(collected))
		for _, r := range collected {
			tr := TestResult{
				Name:     r.name,
				Passed:   r.passed,
				Duration: r.duration,
				Logs:     r.report,
			}
			if !r.passed {
				tr.Error = r.report
			}
			junitResults = append(junitResults, tr)
		}
		if err := writeJUnit(w, releaseName, junitResults); nil != err {
			return err
		}
	case "json":
		structured := make([]map[string]any, 0, len(collected))
		for _, r := range collected {
			structured = append(structured, map[string]any{
				"name":     r.name,
				"passed":   r.passed,
				"duration": r.duration.String(),
				"output":   r.report,
			})
		}
		out, fmtErr := FormatJSON(structured)
		if nil != fmtErr {
			return fmtErr
		}
		fmt.Fprint(w, out)
	default:
		for _, r := range collected {
			fmt.Fprint(w, r.report)
		}
		if allPassed {
			fmt.Fprintln(w, "All tests passed.")
		}
	}

	if !allPassed {
		return keramoserr.NewErrorf(keramoserr.ErrRelease, "one or more tests failed for release %s", releaseName)
	}
	return nil
}

// executeOneTest applies the test manifest, waits for any Job/Pod to finish,
// and returns (passed, optionalLogs, err). The caller owns cleanup.
func executeOneTest(client kube.KubeClient, clientset kubernetes.Interface, ns, manifest string, timeout time.Duration, showLogs bool) (bool, string, error) {
	if applyErr := client.ApplyManifests(manifest); nil != applyErr {
		return false, "", applyErr
	}
	resources, parseErr := kube.ParseManifests(manifest)
	if nil != parseErr {
		return false, "", parseErr
	}
	passed := true
	var logBuf strings.Builder
	for _, res := range resources {
		kind := res.GetKind()
		name := res.GetName()
		if "Pod" != kind && "Job" != kind {
			continue
		}
		if "Job" == kind {
			if waitErr := client.WaitForJob(ns, name, timeout); nil != waitErr {
				passed = false
			}
			continue
		}
		ok, waitErr := waitForTestPod(clientset, ns, name, timeout)
		if nil != waitErr || !ok {
			passed = false
			continue
		}
		if showLogs {
			if l, _ := getPodLogs(clientset, ns, name); "" != l {
				logBuf.WriteString(l)
				if !strings.HasSuffix(l, "\n") {
					logBuf.WriteByte('\n')
				}
			}
		}
	}
	return passed, logBuf.String(), nil
}

func matchesAnyFilter(name string, filter []string) bool {
	for _, f := range filter {
		if strings.Contains(name, f) {
			return true
		}
	}
	return false
}

func waitForTestPod(clientset kubernetes.Interface, ns, podName string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return waitForPodCompletion(ctx, clientset, ns, podName)
}

func waitForPodCompletion(ctx context.Context, clientset kubernetes.Interface, ns, podName string) (bool, error) {
	var finalPhase corev1.PodPhase

	pollErr := wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		finalPhase = pod.Status.Phase
		return corev1.PodSucceeded == finalPhase || corev1.PodFailed == finalPhase, nil
	})

	if nil != pollErr {
		return false, keramoserr.WrapError(keramoserr.ErrKube, "timeout waiting for test pod to complete", pollErr)
	}

	return corev1.PodSucceeded == finalPhase, nil
}

func getPodLogs(clientset kubernetes.Interface, ns, podName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req := clientset.CoreV1().Pods(ns).GetLogs(podName, &corev1.PodLogOptions{})
	result := req.Do(ctx)
	rawBody, err := result.Raw()
	if nil != err {
		return "", err
	}
	return string(rawBody), nil
}

func splitLogLines(logs string) []string {
	if "" == logs {
		return nil
	}
	out := make([]string, 0)
	for _, line := range strings.Split(logs, "\n") {
		if "" != line {
			out = append(out, line)
		}
	}
	return out
}

// Legacy helpers retained for tests. New code should not depend on these.
func containsTest(name string) bool  { return strings.Contains(strings.ToLower(name), "test") }
func contains(s, substr string) bool { return strings.Contains(s, substr) }
func buildTestPodManifest(name, ns string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
  labels:
    keramos/test: "true"
spec:
  restartPolicy: Never
  containers:
  - name: test
    image: busybox
    command: ["sh", "-c", "echo test"]
`, name, ns)
}

type testHook struct {
	Name     string
	Manifest string
}

func findTestHooks(rel *release.Release) []testHook {
	results := make([]testHook, 0, len(rel.Hooks))
	for _, h := range rel.Hooks {
		if containsTest(h.Name) {
			results = append(results, testHook{Name: h.Name, Manifest: buildTestPodManifest(h.Name, rel.Namespace)})
		}
	}
	return results
}
