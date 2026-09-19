package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ebogdum/keramos/v2/internal/action"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/kube"
	keramoslabels "github.com/ebogdum/keramos/v2/internal/labels"
	"github.com/ebogdum/keramos/v2/internal/release"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// newPurgeCommand removes everything keramos has installed across the cluster.
//
//	keramos purge --dry-run                      # show what would be removed
//	keramos purge --yes                          # actually remove
//	keramos purge --yes --delete-namespaces      # also drop namespaces keramos created
//	keramos purge --yes --delete-crds            # also remove keramos-installed CRDs
//	keramos purge --yes --ns-prefix keramos-r12     # restrict scope by ns prefix
//
// Safety mechanism: every action is gated behind --yes. The dry-run path is
// the default. Keramos only touches resources whose labels mark them as
// keramos-managed (`managedBy=keramos` or legacy `owner=keramos` on release Secrets); resources keramos
// never installed are not touched.
func newPurgeCommand() *cobra.Command {
	var (
		dryRun           bool
		confirm          bool
		force            bool
		nsPrefix         string
		parallel         int
		deleteNamespaces bool
		deleteCRDs       bool
		ignoreFailures   bool
		excludeNs        []string
	)
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Clean up everything keramos has installed (use --yes after a --dry-run)",
		Long: `Find every release keramos has ever created (matched by the managedBy=keramos
or legacy owner=keramos label
on the release-storage Secret), uninstall each one, and optionally remove
the namespaces and CRDs keramos created.

By default this command is dry-run; it prints what would be removed and
exits without touching the cluster. Pass --yes to actually run.

Safety:
  * Only resources keramos labelled managedBy=keramos (or legacy owner=keramos) are touched.
  * kube-system / kube-public / kube-node-lease / default are never deleted
    regardless of --delete-namespaces.
  * --exclude-ns lets you protect specific namespaces.

Examples:

  # Show what would be removed across keramos-r12 namespaces only
  keramos purge --dry-run --ns-prefix keramos-r12

  # Actually purge those namespaces too
  keramos purge --yes --ns-prefix keramos-test --delete-namespaces

  # Full cluster wipe (dangerous)
  keramos purge --yes --delete-namespaces --delete-crds`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !dryRun && !confirm {
				return keramoserr.NewError(keramoserr.ErrCLIValidation,
					"refusing to purge without --yes (run with --dry-run first to preview)")
			}
			client, err := kube.NewClient(kubeconfig, kubeContext, "")
			if nil != err {
				return err
			}
			cs := client.Clientset()
			if nil == cs {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "no Kubernetes client available")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()

			scope, scanErr := scanKeramosReleases(ctx, cs, nsPrefix, excludeNs)
			if nil != scanErr {
				return scanErr
			}
			// Also pick up any namespace keramos labelled at creation time
			// (CreateNamespace stamps managedBy=keramos). This catches
			// namespaces that keramos made but where every release Secret
			// has already been deleted — for example after a partial
			// purge or a node failure that wiped out the rest of the
			// release. We use a label selector, not a name prefix, so
			// keramos never claims a namespace it didn't create.
			added, scanNsErr := scanKeramosManagedNamespaces(ctx, cs, scope, nsPrefix, excludeNs)
			if nil != scanNsErr {
				return scanNsErr
			}
			if 0 < added {
				fmt.Fprintf(cmd.OutOrStdout(),
					"discovered %d keramos-managed namespace(s) without remaining releases\n", added)
			}
			if 0 == len(scope) {
				fmt.Fprintln(cmd.OutOrStdout(), "no keramos-managed releases or namespaces found in scope")
				if force && !dryRun {
					// Even with nothing in scope, --force still sweeps any
					// orphaned pods left in nonexistent namespaces from
					// prior runs. This is the only place that cleanup
					// happens, so we can't early-exit before it.
					cleared := sweepOrphanPods(ctx, cs, cmd)
					if 0 < cleared {
						fmt.Fprintf(cmd.OutOrStdout(), "swept %d orphan pod(s)\n", cleared)
					}
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"found %d release(s) across %d namespace(s) in scope\n",
				countReleases(scope), len(scope))
			if dryRun {
				printPurgePlan(cmd, scope, deleteNamespaces, deleteCRDs)
				return nil
			}

			if 1 > parallel {
				parallel = 4
			}
			if force {
				// Force implies ignore-failures: a force run is a one-shot
				// best-effort wipe, not a transactional uninstall.
				ignoreFailures = true
			}
			failed := purgeReleases(ctx, scope, parallel, ignoreFailures, force, cmd)

			if deleteNamespaces {
				dropNamespaces(ctx, cs, scope, ignoreFailures, force, cmd)
			}
			if force {
				// Mop up any pods whose namespace no longer exists. These
				// would otherwise persist forever — there is no controller
				// in vanilla Kubernetes that garbage-collects pods in a
				// nonexistent namespace.
				cleared := sweepOrphanPods(ctx, cs, cmd)
				if 0 < cleared {
					fmt.Fprintf(cmd.OutOrStdout(), "swept %d orphan pod(s)\n", cleared)
				}
			}
			if deleteCRDs {
				dropKeramosCRDs(ctx, client, ignoreFailures, cmd)
			}

			if 0 < failed && !ignoreFailures {
				return keramoserr.NewErrorf(keramoserr.ErrInternal,
					"%d release(s) failed to uninstall (use --ignore-failures to suppress)", failed)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "purge complete")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview only; print what would be removed")
	cmd.Flags().BoolVar(&confirm, "yes", false, "actually run; required when --dry-run is not set")
	cmd.Flags().StringVar(&nsPrefix, "ns-prefix", "", "restrict scope to namespaces beginning with this prefix")
	cmd.Flags().IntVar(&parallel, "parallel", 4, "number of namespaces to purge concurrently")
	cmd.Flags().BoolVar(&deleteNamespaces, "delete-namespaces", false, "after uninstall, delete every namespace that contained a keramos release (excludes kube-*/default)")
	cmd.Flags().BoolVar(&deleteCRDs, "delete-crds", false, "remove keramos-installed CRDs (currently: keramosreleases.keramos.dev)")
	cmd.Flags().BoolVar(&ignoreFailures, "ignore-failures", false, "keep going past failed uninstalls; report at the end")
	cmd.Flags().BoolVar(&force, "force", false, "skip graceful uninstall: delete storage Secrets directly, force-delete pods, force-finalize stuck namespaces (use after a node failure)")
	cmd.Flags().StringSliceVar(&excludeNs, "exclude-ns", nil, "namespaces to skip (repeatable, comma-separated)")
	return cmd
}

// purgeScope groups release names by namespace.
type purgeScope map[string]map[string]bool

func countReleases(s purgeScope) int {
	n := 0
	for _, m := range s {
		n += len(m)
	}
	return n
}

var reservedNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
	"default":         true,
}

// scanKeramosReleases lists every Secret in the cluster carrying the keramos
// management marker (the new managedBy=keramos label or the legacy owner=keramos)
// and reverse-engineers each release's name from the Secret name. Keramos
// stores release records as `keramos.v<schemaVersion>.<release>.v<revision>`
// in the release namespace.
func scanKeramosReleases(ctx context.Context, cs kubernetes.Interface, nsPrefix string, excludeNs []string) (purgeScope, error) {
	exclude := map[string]bool{}
	for _, n := range excludeNs {
		exclude[n] = true
	}
	// Two passes: managedBy=keramos (canonical) and owner=keramos (legacy).
	// k8s label selectors AND on commas; there's no native OR, so we union
	// the result sets by secret UID.
	seen := map[string]bool{}
	merged := []corev1.Secret{}
	for _, sel := range []string{keramoslabels.Selector(), "owner=keramos"} {
		list, err := cs.CoreV1().Secrets("").List(ctx, metav1.ListOptions{LabelSelector: sel})
		if nil != err {
			return nil, keramoserr.WrapError(keramoserr.ErrKube, "list keramos-managed secrets", err)
		}
		for _, s := range list.Items {
			key := string(s.UID)
			if "" == key {
				key = s.Namespace + "/" + s.Name
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, s)
		}
	}
	scope := purgeScope{}
	for _, s := range merged {
		ns := s.Namespace
		if exclude[ns] {
			continue
		}
		if "" != nsPrefix && !strings.HasPrefix(ns, nsPrefix) {
			continue
		}
		name := s.Name // e.g. keramos.v1.myrel.v3
		releaseName := parseReleaseFromSecretName(name)
		if "" == releaseName {
			continue
		}
		if _, ok := scope[ns]; !ok {
			scope[ns] = map[string]bool{}
		}
		scope[ns][releaseName] = true
	}
	return scope, nil
}

// scanKeramosManagedNamespaces lists every namespace stamped with the
// canonical keramos-managed label (`managedBy=keramos`) and adds it to the
// purge scope. Namespaces that already have releases tracked are
// untouched; only label-marked-but-orphaned namespaces are added with
// an empty release set, which means dropNamespaces will drain and
// remove them but purgeReleases skips them (nothing to uninstall).
func scanKeramosManagedNamespaces(ctx context.Context, cs kubernetes.Interface, scope purgeScope, nsPrefix string, excludeNs []string) (int, error) {
	exclude := map[string]bool{}
	for _, n := range excludeNs {
		exclude[n] = true
	}
	list, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{LabelSelector: keramoslabels.Selector()})
	if nil != err {
		return 0, keramoserr.WrapError(keramoserr.ErrKube, "list keramos-managed namespaces", err)
	}
	added := 0
	for _, ns := range list.Items {
		if exclude[ns.Name] || reservedNamespaces[ns.Name] {
			continue
		}
		if "" != nsPrefix && !strings.HasPrefix(ns.Name, nsPrefix) {
			continue
		}
		if _, exists := scope[ns.Name]; exists {
			continue
		}
		scope[ns.Name] = map[string]bool{}
		added++
	}
	return added, nil
}

// parseReleaseFromSecretName extracts <release> from `keramos.v<x>.<release>.v<rev>`.
func parseReleaseFromSecretName(name string) string {
	if !strings.HasPrefix(name, "keramos.v") {
		return ""
	}
	rest := strings.TrimPrefix(name, "keramos.v")
	// rest = `<schema>.<release>.v<rev>`. Split off the leading schema int.
	i := strings.IndexByte(rest, '.')
	if i < 0 {
		return ""
	}
	rest = rest[i+1:]
	// Strip trailing `.v<rev>`.
	j := strings.LastIndex(rest, ".v")
	if j < 0 {
		return rest
	}
	return rest[:j]
}

func printPurgePlan(cmd *cobra.Command, scope purgeScope, deleteNS, deleteCRDs bool) {
	namespaces := sortedScopeKeys(scope)
	fmt.Fprintln(cmd.OutOrStdout(), "\n--- DRY RUN ---")
	for _, ns := range namespaces {
		releases := sortedReleaseList(scope[ns])
		marker := ""
		if deleteNS && !reservedNamespaces[ns] {
			marker = "  [namespace will be deleted]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ns/%s%s\n", ns, marker)
		for _, r := range releases {
			fmt.Fprintf(cmd.OutOrStdout(), "    └ release/%s\n", r)
		}
	}
	if deleteCRDs {
		fmt.Fprintln(cmd.OutOrStdout(), "\n  CRDs to remove:")
		fmt.Fprintln(cmd.OutOrStdout(), "    └ customresourcedefinitions.apiextensions.k8s.io/keramosreleases.keramos.dev")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "\n(re-run with --yes to actually purge)")
}

func purgeReleases(ctx context.Context, scope purgeScope, parallel int, ignoreFailures, force bool, cmd *cobra.Command) int {
	type job struct{ ns, name string }
	jobs := make([]job, 0)
	for ns, names := range scope {
		for n := range names {
			jobs = append(jobs, job{ns: ns, name: n})
		}
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].ns != jobs[j].ns {
			return jobs[i].ns < jobs[j].ns
		}
		return jobs[i].name < jobs[j].name
	})

	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := 0
	done := 0
	total := len(jobs)
	for _, j := range jobs {
		j := j
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			client, err := kube.NewClient(kubeconfig, kubeContext, j.ns)
			if nil != err {
				mu.Lock()
				failed++
				mu.Unlock()
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s/%s: client error: %v\n", j.ns, j.name, err)
				return
			}
			cs := client.Clientset()
			if force {
				// Skip the graceful uninstall — it waits on finalizers that
				// dead kubelets will never run. Just delete the storage
				// Secret(s) so keramos forgets about the release.
				if nil != cs {
					forceDeleteReleaseSecrets(ctx, cs, j.ns, j.name)
				}
				mu.Lock()
				done++
				d := done
				mu.Unlock()
				fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s/%s [%d/%d] (force)\n", j.ns, j.name, d, total)
				return
			}
			_, uErr := action.Uninstall(client, &action.UninstallOptions{
				ReleaseName:    j.name,
				Namespace:      j.ns,
				IgnoreNotFound: true,
				NoHooks:        true,
				Timeout:        2 * time.Minute,
				KeepHistory:    false,
			})
			mu.Lock()
			done++
			d := done
			mu.Unlock()
			if nil != uErr {
				mu.Lock()
				failed++
				mu.Unlock()
				if !ignoreFailures {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✗ %s/%s [%d/%d]: %v\n", j.ns, j.name, d, total, uErr)
				}
				return
			}
			// Always also nuke the storage secret so retry never sees it.
			// Match either label key — current installs write both, but
			// older ones may have only `owner=keramos`.
			if nil != cs {
				forceDeleteReleaseSecrets(ctx, cs, j.ns, j.name)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s/%s [%d/%d]\n", j.ns, j.name, d, total)
		}()
	}
	wg.Wait()
	return failed
}

// forceDeleteReleaseSecrets removes every release-storage Secret in `ns`
// whose `name` label matches `release`, under either the canonical
// managedBy=keramos or the legacy owner=keramos marker.
func forceDeleteReleaseSecrets(ctx context.Context, cs kubernetes.Interface, ns, release string) {
	deleted := map[string]bool{}
	for _, sel := range []string{keramoslabels.Selector() + ",name=" + release, "owner=keramos,name=" + release} {
		ss, _ := cs.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{LabelSelector: sel})
		if nil == ss {
			continue
		}
		for _, s := range ss.Items {
			if deleted[s.Name] {
				continue
			}
			deleted[s.Name] = true
			_ = cs.CoreV1().Secrets(ns).Delete(ctx, s.Name, metav1.DeleteOptions{})
		}
	}
}

func dropNamespaces(ctx context.Context, cs kubernetes.Interface, scope purgeScope, ignoreFailures, force bool, cmd *cobra.Command) {
	for _, ns := range sortedScopeKeys(scope) {
		if reservedNamespaces[ns] {
			continue
		}
		if force {
			// Tear down in dependency order so controllers don't respawn
			// pods we just killed and so pods aren't running with their
			// Secrets/ConfigMaps already gone:
			//   1. Pod-spawning controllers (Deployments, StatefulSets,
			//      DaemonSets, Jobs, CronJobs, ReplicaSets).
			//   2. Pods themselves (grace=0).
			//   3. Network/config dependents (Services, Ingresses).
			//   4. Storage (PVCs).
			//   5. Mounts (Secrets, ConfigMaps) — these go LAST so any
			//      still-running pod observed by other tooling doesn't
			//      lose its volume backing before the pod is gone.
			// The namespace controller cascades the rest at finalize time.
			forceDrainNamespace(ctx, cs, ns, cmd)
		}
		dErr := cs.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
		if nil != dErr && !apierrors.IsNotFound(dErr) {
			if !force && !ignoreFailures {
				fmt.Fprintf(cmd.OutOrStdout(), "  ✗ ns/%s: %v\n", ns, dErr)
				continue
			}
		}
		if force {
			// Strip finalizers via the /finalize subresource so namespaces
			// stuck in Terminating actually disappear.
			if fErr := forceFinalizeNamespace(ctx, cs, ns); nil != fErr && !apierrors.IsNotFound(fErr) {
				fmt.Fprintf(cmd.OutOrStdout(), "  ! ns/%s finalize: %v\n", ns, fErr)
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ ns/%s force-deleted\n", ns)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ ns/%s deleted\n", ns)
	}
}

// forceDrainNamespace tears down everything inside a namespace in
// dependency order before the namespace itself is deleted. The
// motivation: a kubelet-orphaned cluster ends up with Pods that won't
// terminate, controllers that respawn them, and finalizers that never
// run. Doing the namespace.Delete in one shot lets the namespace
// controller delete objects in arbitrary order, which means a Pod can
// observe its Secret/ConfigMap disappear before kubelet has actually
// stopped it — leading to crash-loops, mount errors, and stuck
// terminations. Stepping through the order manually avoids that.
func forceDrainNamespace(ctx context.Context, cs kubernetes.Interface, ns string, cmd *cobra.Command) {
	zero := int64(0)
	bg := metav1.DeletePropagationBackground
	delOpts := metav1.DeleteOptions{GracePeriodSeconds: &zero, PropagationPolicy: &bg}
	listOpts := metav1.ListOptions{}

	// 1. Kill pod-spawning controllers first so pods don't get respawned
	//    while we delete them. Use background propagation so the API
	//    server cascades the controller's owned objects.
	if cj, _ := cs.BatchV1().CronJobs(ns).List(ctx, listOpts); nil != cj {
		for _, o := range cj.Items {
			_ = cs.BatchV1().CronJobs(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	if jj, _ := cs.BatchV1().Jobs(ns).List(ctx, listOpts); nil != jj {
		for _, o := range jj.Items {
			_ = cs.BatchV1().Jobs(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	if ds, _ := cs.AppsV1().DaemonSets(ns).List(ctx, listOpts); nil != ds {
		for _, o := range ds.Items {
			_ = cs.AppsV1().DaemonSets(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	if ss, _ := cs.AppsV1().StatefulSets(ns).List(ctx, listOpts); nil != ss {
		for _, o := range ss.Items {
			_ = cs.AppsV1().StatefulSets(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	if dd, _ := cs.AppsV1().Deployments(ns).List(ctx, listOpts); nil != dd {
		for _, o := range dd.Items {
			_ = cs.AppsV1().Deployments(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	if rs, _ := cs.AppsV1().ReplicaSets(ns).List(ctx, listOpts); nil != rs {
		for _, o := range rs.Items {
			_ = cs.AppsV1().ReplicaSets(ns).Delete(ctx, o.Name, delOpts)
		}
	}

	// 2. Now the pods themselves. We do two passes:
	//    (a) issue grace=0 delete — this removes the etcd record IF the
	//        kubelet is alive to ack, otherwise the API server holds the
	//        record waiting for the ack;
	//    (b) wait briefly, then re-issue with GracePeriodSeconds=0 AND
	//        DeletionPropagation=Background which is the kubectl
	//        `--force --grace-period=0` equivalent — this unconditionally
	//        removes the etcd record, even if no kubelet ever acks. We
	//        MUST do this before the namespace finalize step or the pod
	//        records will outlive the namespace and become orphans
	//        (visible in `kubectl get pods -A` with no parent ns).
	forceDeletePods(ctx, cs, ns, delOpts)

	// 3. Network/ingress objects — no harm leaving them for the namespace
	//    cascade, but pre-emptively deleting avoids load-balancer churn.
	if svcs, _ := cs.CoreV1().Services(ns).List(ctx, listOpts); nil != svcs {
		for _, o := range svcs.Items {
			_ = cs.CoreV1().Services(ns).Delete(ctx, o.Name, delOpts)
		}
	}

	// 4. Storage. PVCs may have a `kubernetes.io/pvc-protection` finalizer
	//    that the kube-controller-manager removes once the dependent pods
	//    are gone — they'll be gone after step 2.
	if pvcs, _ := cs.CoreV1().PersistentVolumeClaims(ns).List(ctx, listOpts); nil != pvcs {
		for _, o := range pvcs.Items {
			_ = cs.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, o.Name, delOpts)
		}
	}

	// 5. Configuration objects last. Any pod still mounting them is in
	//    grace=0 deletion already, so the API has erased the pod object.
	if cms, _ := cs.CoreV1().ConfigMaps(ns).List(ctx, listOpts); nil != cms {
		for _, o := range cms.Items {
			_ = cs.CoreV1().ConfigMaps(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	if secrets, _ := cs.CoreV1().Secrets(ns).List(ctx, listOpts); nil != secrets {
		for _, o := range secrets.Items {
			// Skip ServiceAccount tokens; they get cleaned up by the
			// namespace cascade and the SA controller.
			if corev1.SecretTypeServiceAccountToken == o.Type {
				continue
			}
			_ = cs.CoreV1().Secrets(ns).Delete(ctx, o.Name, delOpts)
		}
	}
	_ = cmd // reserved for future verbose progress output
}

// forceDeletePods removes every Pod in `ns` from etcd. Two passes:
// first a normal grace=0 delete (the kubelet may ack and clean up the
// container); second, after a short wait, a re-issue that forces the
// API server to drop the etcd record regardless of whether any kubelet
// ever acked. This is the programmatic equivalent of
// `kubectl delete pod --force --grace-period=0` and it's what prevents
// pods from outliving their namespace as orphans when the namespace is
// later force-finalized.
func forceDeletePods(ctx context.Context, cs kubernetes.Interface, ns string, delOpts metav1.DeleteOptions) {
	first, _ := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if nil == first {
		return
	}
	for _, p := range first.Items {
		_ = cs.CoreV1().Pods(ns).Delete(ctx, p.Name, delOpts)
	}
	if 0 == len(first.Items) {
		return
	}
	// Brief wait so cooperative kubelets can ack.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		remaining, _ := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if nil == remaining || 0 == len(remaining.Items) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Re-issue with grace=0 — at this point the API server unconditionally
	// removes the etcd record, even if the kubelet never acked.
	stragglers, _ := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if nil == stragglers {
		return
	}
	for _, p := range stragglers.Items {
		_ = cs.CoreV1().Pods(ns).Delete(ctx, p.Name, delOpts)
	}
}

// sweepOrphanPods finds pods whose namespace no longer exists in the
// cluster and force-deletes them. Orphan pods can result from prior
// purges that ran before forceDeletePods existed, or from any path
// where a namespace was removed via /finalize while pods on a dead
// kubelet were still present in etcd.
func sweepOrphanPods(ctx context.Context, cs kubernetes.Interface, cmd *cobra.Command) int {
	pods, err := cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if nil != err || nil == pods {
		return 0
	}
	nsList, nsErr := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if nil != nsErr || nil == nsList {
		return 0
	}
	live := make(map[string]bool, len(nsList.Items))
	for _, n := range nsList.Items {
		live[n.Name] = true
	}
	zero := int64(0)
	bg := metav1.DeletePropagationBackground
	delOpts := metav1.DeleteOptions{GracePeriodSeconds: &zero, PropagationPolicy: &bg}
	cleared := 0
	for _, p := range pods.Items {
		if live[p.Namespace] {
			continue
		}
		if dErr := cs.CoreV1().Pods(p.Namespace).Delete(ctx, p.Name, delOpts); nil == dErr || apierrors.IsNotFound(dErr) {
			cleared++
			fmt.Fprintf(cmd.OutOrStdout(), "  ✓ orphan pod %s/%s force-deleted\n", p.Namespace, p.Name)
		}
	}
	return cleared
}

// forceFinalizeNamespace clears `spec.finalizers` on a namespace via the
// /finalize subresource. This is the documented escape hatch when a
// namespace is stuck in Terminating because a controller-owned finalizer
// will never run (e.g. its controller is gone, or the kubelet that hosts
// the dependent pods is down).
func forceFinalizeNamespace(ctx context.Context, cs kubernetes.Interface, name string) error {
	nsObj, getErr := cs.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if nil != getErr {
		return getErr
	}
	if 0 == len(nsObj.Spec.Finalizers) {
		return nil
	}
	nsObj.Spec.Finalizers = nil
	_, updErr := cs.CoreV1().Namespaces().Finalize(ctx, nsObj, metav1.UpdateOptions{})
	return updErr
}

func dropKeramosCRDs(ctx context.Context, client kube.KubeClient, ignoreFailures bool, cmd *cobra.Command) {
	dyn, err := client.Dynamic()
	if nil != err {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✗ keramos-CRDs: dynamic client: %v\n", err)
		return
	}
	gvr := schema.GroupVersionResource{
		Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
	}
	for _, name := range []string{"keramosreleases.keramos.dev"} {
		dErr := dyn.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
		if nil != dErr && !apierrors.IsNotFound(dErr) {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✗ crd/%s: %v\n", name, dErr)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ crd/%s removed\n", name)
	}
}

func sortedScopeKeys(m purgeScope) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedReleaseList(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// _ = release.SecretName silences the import; we keep the import for
// future use when surfacing exact secret names per release/revision.
var _ = release.SecretName
