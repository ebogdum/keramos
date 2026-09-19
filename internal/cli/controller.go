package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ebogdum/keramos/v3/internal/action"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// validateControllerPackagePath rejects absolute paths, traversal sequences,
// and paths outside the controller's allowlisted root. A namespaced tenant
// with create/KeramosRelease RBAC must NOT be able to point the controller at
// /etc, /proc, or pod-local secret mounts.
func validateControllerPackagePath(p, root string) error {
	if "" == p {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "spec.package is empty")
	}
	if filepath.IsAbs(p) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"spec.package %q must be a relative path under the controller's allowlisted root", p)
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"spec.package %q contains a path traversal sequence", p)
	}
	if "" != root {
		full := filepath.Join(root, clean)
		rel, relErr := filepath.Rel(root, full)
		if nil != relErr || strings.HasPrefix(rel, "..") {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"spec.package %q resolves outside %s", p, root)
		}
	}
	return nil
}

// secretLikePatterns redact common credential shapes before they are
// written into a CR status, where any cluster reader with `get
// keramosreleases` can see them.
var secretLikePatterns = []*regexp.Regexp{
	regexp.MustCompile(`hvs\.[A-Za-z0-9_\-]+`),                                 // Vault service token
	regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+`),                        // Authorization: Bearer
	regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`), // JWT
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                                     // AWS access key id
	regexp.MustCompile(`ya29\.[A-Za-z0-9_\-]+`),                                // Google OAuth access token
	regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`),                               // Google API key
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),                           // GitHub tokens
	regexp.MustCompile(`glpat-[A-Za-z0-9_\-]{20,}`),                            // GitLab PAT
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9\-]{10,}`),                        // Slack token
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),                   // PEM private key header
	// Labelled secrets: capture quoted values (which may contain spaces) as
	// well as bare non-whitespace tokens.
	regexp.MustCompile(`(?i)(?:password|passwd|pwd|token|secret|api[_\-]?key|access[_\-]?key|client[_\-]?secret)\s*[:=]\s*(?:"[^"]*"|'[^']*'|\S+)`),
}

// urlUserinfoSecret matches the password component of a URL userinfo block
// (scheme://user:password@host) so connection strings don't leak via status.
var urlUserinfoSecret = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://[^:@\s/]+:)[^@\s/]+(@)`)

// scrubError redacts secret-shaped substrings, then truncates so a multi-MB
// response body cannot be exfiltrated wholesale through the CR status field.
func scrubError(err error) string {
	if nil == err {
		return ""
	}
	s := err.Error()
	s = urlUserinfoSecret.ReplaceAllString(s, "${1}[REDACTED]${2}")
	for _, re := range secretLikePatterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	const max = 256
	if len(s) > max {
		return s[:max] + "…(truncated)"
	}
	return s
}

// KeramosRelease group/version/kind for the CRD reconciled by `keramos controller`.
var keramosReleaseGVR = schema.GroupVersionResource{
	Group: "keramos.dev", Version: "v1", Resource: "keramosreleases",
}

// newControllerCommand starts an in-process reconciler for KeramosRelease CRs.
// On each tick it lists every KeramosRelease, computes desired state, and runs
// install/upgrade. Status sub-resource is updated with revision + condition.
func newControllerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "controller",
		Short: "Reconcile KeramosRelease CRs declared in the cluster",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newControllerInstallCRDCommand())
	cmd.AddCommand(newControllerRunCommand())
	cmd.AddCommand(newControllerCRDYAMLCommand())
	return cmd
}

func newControllerCRDYAMLCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "crd",
		Short: "Print the KeramosRelease CRD YAML",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), keramosReleaseCRDYAML)
			return nil
		},
	}
}

func newControllerInstallCRDCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-crd",
		Short: "Apply the KeramosRelease CRD to the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			return client.ApplyManifests(keramosReleaseCRDYAML)
		},
	}
	return cmd
}

func newControllerRunCommand() *cobra.Command {
	var (
		interval    time.Duration
		watchNS     string
		pkgRoot     string
		leaderElect bool
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the KeramosRelease reconciler in the foreground",
		RunE: func(cmd *cobra.Command, args []string) error {
			if 0 >= interval {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"--interval must be greater than zero, got %s", interval)
			}

			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			absRoot := pkgRoot
			if "" != absRoot {
				abs, aErr := filepath.Abs(absRoot)
				if nil != aErr {
					return keramoserr.WrapError(keramoserr.ErrCLIValidation, "resolve --package-root", aErr)
				}
				absRoot = abs
			}
			ctrl := &controllerLoop{
				client:      client,
				watchNS:     watchNS,
				interval:    interval,
				processed:   map[string]string{},
				packageRoot: absRoot,
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if !leaderElect {
				return ctrl.run(ctx, cmd)
			}
			return runWithLeaderElection(ctx, cmd, client, leaseNamespace(watchNS), func(leaderCtx context.Context) error {
				return ctrl.run(leaderCtx, cmd)
			})
		},
	}
	cmd.Flags().BoolVar(&leaderElect, "leader-elect", true, "hold a lease so only one replica reconciles at a time")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "reconcile interval")
	cmd.Flags().StringVar(&watchNS, "watch-namespace", "", "namespace to watch (empty = all)")
	cmd.Flags().StringVar(&pkgRoot, "package-root", "/var/lib/keramos/packages",
		"directory under which CR-supplied package paths must resolve (anti-traversal root)")
	return cmd
}

type controllerLoop struct {
	client      kube.KubeClient
	watchNS     string
	interval    time.Duration
	processed   map[string]string // key=ns/name → resourceVersion last reconciled
	packageRoot string
	mu          sync.Mutex
}

func (c *controllerLoop) run(ctx context.Context, cmd *cobra.Command) error {
	t := time.NewTicker(c.interval)
	defer t.Stop()
	for {
		if err := c.reconcileAll(ctx); nil != err {
			logger.Warn("reconcile cycle: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (c *controllerLoop) reconcileAll(ctx context.Context) error {
	dyn, err := c.client.Dynamic()
	if nil != err {
		return err
	}
	list, err := dyn.Resource(keramosReleaseGVR).Namespace(c.watchNS).List(ctx, metav1.ListOptions{})
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrInternal, "list KeramosReleases", err)
	}
	// Track which CRs still exist so we can evict stale entries from the
	// `processed` cache. Without this, the cache grows linearly with the
	// cumulative count of CRs ever observed in the cluster — a long-running
	// controller leaks memory across redeploys.
	live := make(map[string]struct{}, len(list.Items))
	for i := range list.Items {
		// Honour ctx cancellation between items so a long list does not
		// block shutdown.
		if cErr := ctx.Err(); nil != cErr {
			return cErr
		}
		item := &list.Items[i]
		live[item.GetNamespace()+"/"+item.GetName()] = struct{}{}
		if rerr := c.reconcileOne(ctx, item); nil != rerr {
			logger.Warn("KeramosRelease %s/%s: %v", item.GetNamespace(), item.GetName(), rerr)
		}
	}
	c.mu.Lock()
	for k := range c.processed {
		if _, ok := live[k]; !ok {
			delete(c.processed, k)
		}
	}
	c.mu.Unlock()
	return nil
}

type keramosReleaseSpec struct {
	ReleaseName string         `json:"releaseName"`
	Package     string         `json:"package"`
	Version     string         `json:"version,omitempty"`
	Values      map[string]any `json:"values,omitempty"`
	Profile     string         `json:"profile,omitempty"`
}

func (c *controllerLoop) reconcileOne(ctx context.Context, item *unstructured.Unstructured) error {
	key := item.GetNamespace() + "/" + item.GetName()
	c.mu.Lock()
	last := c.processed[key]
	c.mu.Unlock()
	if last == item.GetResourceVersion() && !c.releaseHasDrifted(item) {
		return nil
	}
	rawSpec, _, _ := unstructured.NestedMap(item.Object, "spec")
	specJSON, _ := json.Marshal(rawSpec)
	var spec keramosReleaseSpec
	if err := json.Unmarshal(specJSON, &spec); nil != err {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "decode spec", err)
	}
	if "" == spec.ReleaseName {
		spec.ReleaseName = item.GetName()
	}
	if vErr := validateControllerPackagePath(spec.Package, c.packageRoot); nil != vErr {
		if sErr := c.setStatus(ctx, item, "Failed", scrubError(vErr), 0); nil != sErr {
			logger.Warn("status update failed for %s/%s: %v", item.GetNamespace(), item.GetName(), sErr)
		}
		return vErr
	}
	resolvedPkg := spec.Package
	if "" != c.packageRoot {
		// The controller treats packageRoot as a strict allowlist of
		// pre-provisioned packages: spec.package MUST refer to a path that
		// already exists under the resolved root. Allowing lazy-create paths
		// re-introduces a TOCTOU where a tenant could create a symlink at
		// the not-yet-existing leaf between validation and use.
		realRoot, rrErr := filepath.EvalSymlinks(c.packageRoot)
		if nil != rrErr {
			err := keramoserr.WrapError(keramoserr.ErrCLIValidation, "resolve package root", rrErr)
			if sErr := c.setStatus(ctx, item, "Failed", scrubError(err), 0); nil != sErr {
				logger.Warn("status update failed for %s/%s: %v", item.GetNamespace(), item.GetName(), sErr)
			}
			return err
		}
		joined := filepath.Join(realRoot, filepath.Clean(spec.Package))
		realPath, evalErr := filepath.EvalSymlinks(joined)
		if nil != evalErr {
			err := keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, evalErr,
				"package %q does not exist under allowlisted root", spec.Package)
			if sErr := c.setStatus(ctx, item, "Failed", scrubError(err), 0); nil != sErr {
				logger.Warn("status update failed for %s/%s: %v", item.GetNamespace(), item.GetName(), sErr)
			}
			return err
		}
		rel, relErr := filepath.Rel(realRoot, realPath)
		if nil != relErr || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			esc := keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"package %q resolves outside allowlisted root", spec.Package)
			if sErr := c.setStatus(ctx, item, "Failed", scrubError(esc), 0); nil != sErr {
				logger.Warn("status update failed for %s/%s: %v", item.GetNamespace(), item.GetName(), sErr)
			}
			return esc
		}
		// Use the fully-resolved real path so downstream consumers do not
		// re-traverse symlinks; the trust boundary is enforced once, here.
		resolvedPkg = realPath
	}
	tmp, tmpErr := os.CreateTemp("", "keramos-controller-vals-*.yaml")
	if nil != tmpErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "create values tempfile", tmpErr)
	}
	defer os.Remove(tmp.Name())
	if 0 < len(spec.Values) {
		data, mErr := yaml.Marshal(spec.Values)
		if nil != mErr {
			tmp.Close()
			return keramoserr.WrapError(keramoserr.ErrInternal, "marshal CR values", mErr)
		}
		if _, wErr := tmp.Write(data); nil != wErr {
			tmp.Close()
			return keramoserr.WrapError(keramoserr.ErrInternal, "write values tempfile", wErr)
		}
	}
	if cErr := tmp.Close(); nil != cErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "close values tempfile", cErr)
	}
	// Use a kube client scoped to the CR's namespace so that namespaced
	// resources without an explicit metadata.namespace inherit the CR's
	// namespace (not the controller's running namespace).
	tenantClient, tcErr := kube.NewClient(kubeconfig, kubeContext, item.GetNamespace())
	if nil != tcErr {
		return tcErr
	}
	rel, err := action.Upgrade(tenantClient, resolvedPkg, &action.UpgradeOptions{
		ReleaseName: spec.ReleaseName,
		Namespace:   item.GetNamespace(),
		ValueFiles:  []string{tmp.Name()},
		Profile:     spec.Profile,
		Install:     true,
		Atomic:      true,
		Wait:        true,
		Timeout:     5 * time.Minute,
	})
	if nil != err {
		if sErr := c.setStatus(ctx, item, "Failed", scrubError(err), 0); nil != sErr {
			logger.Warn("status update failed for %s/%s: %v", item.GetNamespace(), item.GetName(), sErr)
		}
		return err
	}
	c.mu.Lock()
	c.processed[key] = item.GetResourceVersion()
	c.mu.Unlock()
	return c.setStatus(ctx, item, "Deployed", "ok", rel.Revision)
}

func (c *controllerLoop) setStatus(ctx context.Context, item *unstructured.Unstructured, phase, msg string, rev int) error {
	dyn, err := c.client.Dynamic()
	if nil != err {
		return err
	}
	status := map[string]any{
		"phase":          phase,
		"message":        msg,
		"revision":       int64(rev),
		"lastTransition": time.Now().UTC().Format(time.RFC3339),
	}
	// Reconcile takes minutes; the cached resourceVersion is almost certainly
	// stale by now. Refresh-then-update; retry once on conflict to absorb the
	// usual case where another writer just bumped the object.
	for attempt := 0; attempt < 3; attempt++ {
		fresh, gErr := dyn.Resource(keramosReleaseGVR).Namespace(item.GetNamespace()).
			Get(ctx, item.GetName(), metav1.GetOptions{})
		if nil != gErr {
			return keramoserr.WrapError(keramoserr.ErrInternal, "refresh CR for status update", gErr)
		}
		if sErr := unstructured.SetNestedMap(fresh.Object, status, "status"); nil != sErr {
			return keramoserr.WrapError(keramoserr.ErrInternal, "set status", sErr)
		}
		_, uErr := dyn.Resource(keramosReleaseGVR).Namespace(item.GetNamespace()).
			UpdateStatus(ctx, fresh, metav1.UpdateOptions{})
		if nil == uErr {
			return nil
		}
		// Only conflict errors are worth retrying — for anything else
		// (RBAC denied, validation webhook, deletion races) more attempts
		// only delay surfacing the real problem.
		if !apierrors.IsConflict(uErr) {
			return keramoserr.WrapError(keramoserr.ErrInternal, "update CR status", uErr)
		}
		if attempt == 2 {
			return keramoserr.WrapError(keramoserr.ErrInternal, "update CR status (conflict after retries)", uErr)
		}
	}
	return nil
}

const keramosReleaseCRDYAML = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: keramosreleases.keramos.dev
spec:
  group: keramos.dev
  scope: Namespaced
  names:
    plural: keramosreleases
    singular: keramosrelease
    kind: KeramosRelease
    shortNames: [hr]
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [package]
              properties:
                releaseName: { type: string }
                package:     { type: string }
                version:     { type: string }
                profile:     { type: string }
                values:
                  type: object
                  x-kubernetes-preserve-unknown-fields: true
            status:
              type: object
              properties:
                phase:          { type: string }
                message:        { type: string }
                revision:       { type: integer }
                lastTransition: { type: string }
`

func (c *controllerLoop) releaseHasDrifted(item *unstructured.Unstructured) bool {
	releaseName, _, _ := unstructured.NestedString(item.Object, "spec", "releaseName")
	if "" == releaseName {
		releaseName = item.GetName()
	}

	storage, storageErr := release.SelectStorage(c.client.Clientset(), item.GetNamespace())
	if nil != storageErr {
		return false
	}

	current, lastErr := storage.Last(releaseName)
	if nil != lastErr {
		return false
	}

	drifted, driftErr := action.DriftInNamespace(c.client, current.Manifest, current.Namespace)
	if nil != driftErr {
		logger.Warn("drift check for %s failed: %v", releaseName, driftErr)
		return false
	}

	return 0 < len(drifted)
}

func leaseNamespace(watchNS string) string {
	if "" != watchNS {
		return watchNS
	}
	if fromEnv := os.Getenv("POD_NAMESPACE"); "" != fromEnv {
		return fromEnv
	}
	return "default"
}

func runWithLeaderElection(ctx context.Context, cmd *cobra.Command, client *kube.Client, ns string, run func(context.Context) error) error {
	host, hostErr := os.Hostname()
	if nil != hostErr || "" == host {
		host = "keramos"
	}
	identity := fmt.Sprintf("%s-%d", host, os.Getpid())

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{Name: "keramos-controller", Namespace: ns},
		Client:    client.Clientset().CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: identity,
		},
	}

	runErrCh := make(chan error, 1)
	electionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	leaderelection.RunOrDie(electionCtx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(leaderCtx context.Context) {
				logger.Log("acquired leadership as %s; reconciling", identity)
				runErr := run(leaderCtx)
				runErrCh <- runErr
				cancel()
			},
			OnStoppedLeading: func() {
				logger.Warn("lost leadership as %s; stopping reconcile", identity)
			},
		},
	})

	select {
	case runErr := <-runErrCh:
		return runErr
	default:
		return nil
	}
}
