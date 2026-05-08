package action

import (
	"time"

	"github.com/ebogdum/keramos/internal/audit"
	"github.com/ebogdum/keramos/internal/engine"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/hooks"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/layer"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/maputil"
	"github.com/ebogdum/keramos/internal/otel"
	"github.com/ebogdum/keramos/internal/policy"
	"github.com/ebogdum/keramos/internal/release"
	"github.com/ebogdum/keramos/internal/values"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// UpgradeOptions configures an upgrade operation.
type UpgradeOptions struct {
	ReleaseName         string
	Namespace           string
	ValueFiles          []string
	Sets                []string
	SetStrings          []string
	SetFiles            []string
	SetJSON             []string
	Profile             string
	Wait                bool
	Timeout             time.Duration
	DryRun              string
	ReuseValues         bool
	ResetValues         bool
	ResetThenReuseValues bool
	Install             bool
	Description         string
	Atomic              bool
	NoHooks             bool
	CreateNamespace     bool
	IncludeCRDs         bool
	Labels              map[string]string
	APIVersions         []string
	KubeVersion         string
	PostRenderer        string
	PostRenderers       []string
	PostRendererTimeout time.Duration
	HistoryMax          int
	Force               bool
	CleanupOnFail       bool
	RecreatePods        bool
	WaitForJobs         bool
	HookTimeout         time.Duration
	Environment         string
	// Only restricts upgrade to a list of dotted value paths: every key
	// outside the list reverts to its previous-revision value before render.
	// Equivalent to surgical patching: `keramos upgrade --only image.tag,replicas`.
	Only []string
}

// Upgrade upgrades an existing release or installs if --install is set.
func Upgrade(client kube.KubeClient, packagePath string, opts *UpgradeOptions) (rel *release.Release, retErr error) {
	span := otel.Start("upgrade")
	span.SetAttr("release", opts.ReleaseName)
	span.SetAttr("packagePath", packagePath)
	defer func() { span.EndWithError(retErr) }()

	if err := ValidateReleaseName(opts.ReleaseName); nil != err {
		return nil, err
	}

	ns := opts.Namespace
	if "" == ns {
		ns = client.Namespace()
	}
	if "" == ns {
		ns = "default"
	}

	storage := release.NewSecretStorage(client.Clientset(), ns)

	// Step 1: Get latest release
	current, err := storage.Last(opts.ReleaseName)
	if nil != err {
		he, isKeramosErr := err.(*keramoserr.KeramosError)
		isNotFound := isKeramosErr && keramoserr.ErrReleaseNotFound == he.Type
		if opts.Install && isNotFound {
			logger.Debug("release %s not found, installing", opts.ReleaseName)
			return Install(client, packagePath, &InstallOptions{
				ReleaseName:     opts.ReleaseName,
				Namespace:       opts.Namespace,
				ValueFiles:      opts.ValueFiles,
				Sets:            opts.Sets,
				SetStrings:      opts.SetStrings,
				SetFiles:        opts.SetFiles,
				SetJSON:         opts.SetJSON,
				Profile:         opts.Profile,
				Wait:            opts.Wait,
				Timeout:         opts.Timeout,
				DryRun:          opts.DryRun,
				Description:     opts.Description,
				Atomic:          opts.Atomic,
				NoHooks:         opts.NoHooks,
				CreateNamespace: opts.CreateNamespace,
				IncludeCRDs:     opts.IncludeCRDs,
				Labels:          opts.Labels,
				APIVersions:     opts.APIVersions,
				KubeVersion:     opts.KubeVersion,
			})
		}
		if isNotFound {
			return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found (use --install to create)", opts.ReleaseName)
		}
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to retrieve release %s", opts.ReleaseName)
	}

	logger.Debug("upgrading release %s from revision %d", opts.ReleaseName, current.Revision)

	if "" != opts.Environment {
		if envErr := applyEnvironmentToUpgrade(packagePath, opts); nil != envErr {
			return nil, envErr
		}
	}

	// Step 2a: Pre-resolve user-supplied values so sub-chart conditions /
	// tags can be evaluated against --set / -f overrides during layer filtering.
	userValues, err := values.ResolveAll(map[string]any{}, opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
	if nil != err {
		return nil, err
	}

	// Step 2b: Resolve package (filtering disabled layers using userValues).
	resolved, resolveErr := layer.ResolveWithOverrides(packagePath, opts.Profile, userValues)
	if nil != resolveErr {
		return nil, resolveErr
	}

	var mergedValues map[string]any
	switch {
	case opts.ResetThenReuseValues:
		// Apply package defaults first, then merge previous values, then overlays.
		base := layer.DeepMerge(map[string]any(resolved.Values), current.Values)
		mergedValues, err = values.ResolveAll(base, opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
	case opts.ReuseValues && !opts.ResetValues:
		mergedValues, err = values.ResolveAll(current.Values, opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
	default:
		mergedValues, err = values.ResolveAll(map[string]any(resolved.Values), opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
	}
	if nil != err {
		return nil, err
	}

	// Validate the fully-merged map against the schema BEFORE applying
	// --only restriction. The schema describes the package's expectations
	// for the new values universe; restricting to a subset would produce
	// false-negative validation when the user's surgical change is fine
	// but a previously-stored value violates the new schema.
	if schemaErr := ValidateValuesAgainstSchema(packagePath, mergedValues); nil != schemaErr {
		return nil, schemaErr
	}

	// Incremental upgrade: keep every key in current.Values except those
	// listed in opts.Only, which take their value from the freshly-merged
	// computation above.
	if 0 < len(opts.Only) {
		mergedValues = maputil.RestrictToPaths(current.Values, mergedValues, opts.Only)
	}

	// Re-validate the restricted map so the deployed values still satisfy
	// the schema. Failure here is a real schema violation (the previous
	// revision's stored values are now incompatible with the new schema).
	if schemaErr := ValidateValuesAgainstSchema(packagePath, mergedValues); nil != schemaErr {
		return nil, schemaErr
	}

	newRevision := current.Revision + 1

	// Step 3: Build render context
	var capabilities map[string]any
	if "" == opts.DryRun {
		caps, capErr := client.GetCapabilities()
		if nil != capErr {
			logger.Warn("failed to get cluster capabilities: %v", capErr)
			capabilities = map[string]any{}
		} else {
			capabilities = caps
		}
	} else {
		capabilities = map[string]any{}
	}
	overrideCapabilities(capabilities, opts.APIVersions, opts.KubeVersion)

	var lookupFn engine.LookupFn
	if nil != client && "" == opts.DryRun {
		lookupFn = client.Lookup
	}
	ctx := &engine.RenderContext{
		Lookup: lookupFn,
		Values: mergedValues,
		Package: map[string]any{
			"name":       resolved.Metadata.Name,
			"version":    resolved.Metadata.Version,
			"appVersion": resolved.Metadata.AppVersion,
		},
		Release: map[string]any{
			"name":      opts.ReleaseName,
			"namespace": ns,
			"revision":  newRevision,
			"isUpgrade": true,
			"isInstall": false,
		},
		Capabilities: capabilities,
		Files:        resolved.Files,
	}

	// Step 4: Render templates
	eng := engine.New()
	manifest, renderErr := eng.Render(resolved.Templates, resolved.Partials, ctx)
	if nil != renderErr {
		return nil, renderErr
	}

	// Render hooks
	renderedHooks, hookErr := renderHooks(eng, resolved.Hooks, resolved.Partials, ctx)
	if nil != hookErr {
		return nil, hookErr
	}

	parsedHooks, hookErr := hooks.ParseHooks(renderedHooks)
	if nil != hookErr {
		return nil, hookErr
	}

	renderedTests, testsErr := renderHooks(eng, resolved.Tests, resolved.Partials, ctx)
	if nil != testsErr {
		return nil, testsErr
	}

	manifest, notes, notesErr := extractNotes(manifest)
	if nil != notesErr {
		return nil, notesErr
	}

	rules, polErr := policy.LoadRules(packagePath)
	if nil != polErr {
		return nil, polErr
	}
	if 0 < len(rules) {
		violations, evalErr := policy.Evaluate(rules, manifest)
		if nil != evalErr {
			return nil, evalErr
		}
		for _, v := range violations {
			if policy.SeverityWarn == v.Severity {
				logger.Warn("policy %s: %s", v.Rule, v.Detail)
			}
		}
		if policy.HasDeny(violations) {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"policy violations:\n%s", policy.FormatHuman(violations))
		}
	}

	if "" != opts.PostRenderer || 0 < len(opts.PostRenderers) {
		chain := opts.PostRenderers
		if 0 == len(chain) && "" != opts.PostRenderer {
			chain = []string{opts.PostRenderer}
		}
		out, prErr := runPostRenderers(chain, manifest, opts.PostRendererTimeout)
		if nil != prErr {
			return nil, prErr
		}
		manifest = out
	}

	var crdManifest string
	if opts.IncludeCRDs {
		loaded, crdErr := loadCRDs(packagePath)
		if nil != crdErr {
			return nil, crdErr
		}
		crdManifest = loaded
		if "" != crdManifest {
			manifest = crdManifest + "---\n" + manifest
		}
	}

	now := time.Now().UTC()
	labels := opts.Labels
	if nil == labels {
		labels = current.Labels
	}
	rel = &release.Release{
		Name:      opts.ReleaseName,
		Namespace: ns,
		Revision:  newRevision,
		Status:    release.StatusPendingUpgrade,
		Package: release.PackageRef{
			Name:       resolved.Metadata.Name,
			Version:    resolved.Metadata.Version,
			AppVersion: resolved.Metadata.AppVersion,
		},
		Values:     mergedValues,
		UserValues: userValues,
		Manifest:      manifest,
		Notes:         notes,
		Tests:         renderedTests,
		HookTemplates: renderedHooks,
		Labels:        labels,
		Audit:         audit.Capture("upgrade", current.Revision),
		Info: release.ReleaseInfo{
			FirstDeployed: current.Info.FirstDeployed,
			LastDeployed:  now,
			Description:   opts.Description,
		},
	}

	if "client" == opts.DryRun {
		rel.Status = release.StatusDeployed
		return rel, nil
	}
	if "server" == opts.DryRun {
		rel.Status = release.StatusDeployed
		if dryRunErr := client.DryRunApply(manifest); nil != dryRunErr {
			return rel, dryRunErr
		}
		return rel, nil
	}

	// Step 5: Create new revision
	if storeErr := storage.Create(rel); nil != storeErr {
		return nil, storeErr
	}

	// Step 6: Execute pre-upgrade hooks
	var preResults []release.HookResult
	var preErr error
	if !opts.NoHooks {
		preResults, preErr = hooks.ExecuteHooksWithTimeout(client, parsedHooks, hooks.PreUpgrade, opts.HookTimeout)
	}
	if nil != preErr {
		rel.Status = release.StatusFailed
		rel.Hooks = preResults
		_ = storage.Update(rel)
		if opts.Atomic {
			atomicRollbackUpgrade(client, storage, current, rel)
		}
		return rel, preErr
	}
	rel.Hooks = append(rel.Hooks, preResults...)

	// Step 7a: Apply CRDs first and wait for Established.
	if "" != crdManifest {
		crdTimeout := opts.Timeout
		if 0 == crdTimeout {
			crdTimeout = 2 * time.Minute
		}
		if crdErr := client.ApplyCRDs(crdManifest, crdTimeout); nil != crdErr {
			rel.Status = release.StatusFailed
			_ = storage.Update(rel)
			return rel, crdErr
		}
	}

	// Step 7b: Apply manifests (server-side apply handles 3-way merge).
	// Snapshot which resources already existed so --cleanup-on-fail only
	// removes those introduced by this upgrade.
	var preExisting map[string]bool
	if opts.CleanupOnFail {
		snap, snapErr := client.SnapshotResources(manifest)
		if nil != snapErr {
			logger.Warn("cleanup-on-fail: snapshot failed (will fall back to full delete): %v", snapErr)
		} else {
			preExisting = snap
		}
	}
	// --force: pre-delete only resources whose immutable fields differ from
	// the desired manifest, so the rest are upgraded via normal apply.
	if opts.Force {
		need, needErr := client.ResourcesNeedingForce(manifest)
		if nil != needErr {
			logger.Warn("force upgrade: divergence check failed: %v (falling back to full pre-delete)", needErr)
			if delErr := client.DeleteManifests(current.Manifest); nil != delErr {
				logger.Warn("force upgrade: pre-delete of previous manifests reported error: %v", delErr)
			}
		} else if 0 < len(need) {
			if delErr := client.DeleteResources(current.Manifest, need); nil != delErr {
				logger.Warn("force upgrade: targeted pre-delete reported error: %v", delErr)
			}
		}
	}
	if applyErr := client.ApplyManifests(manifest); nil != applyErr {
		rel.Status = release.StatusFailed
		_ = storage.Update(rel)
		if opts.CleanupOnFail {
			// Delete only the resources we introduced (those not in preExisting).
			introduced := newResourcesOnly(client, manifest, preExisting)
			if 0 < len(introduced) {
				if cleanErr := client.DeleteResources(manifest, introduced); nil != cleanErr {
					logger.Warn("cleanup-on-fail: failed to delete partially-applied resources: %v", cleanErr)
				}
			}
		}
		if opts.Atomic {
			atomicRollbackUpgrade(client, storage, current, rel)
		}
		return rel, applyErr
	}

	// --recreate-pods: rolling restart of Deployments/StatefulSets/DaemonSets
	// that this manifest manages. Best effort; ignore failures.
	if opts.RecreatePods {
		if rrErr := recreatePodsForManifest(client, manifest); nil != rrErr {
			logger.Warn("recreate-pods: %v", rrErr)
		}
	}

	// Step 8: Wait if requested
	if opts.Wait && opts.WaitForJobs {
		jt := opts.Timeout
		if 0 == jt {
			jt = 5 * time.Minute
		}
		if jobErr := waitForJobsInManifest(client, manifest, jt); nil != jobErr {
			rel.Status = release.StatusFailed
			_ = storage.Update(rel)
			if opts.Atomic {
				atomicRollbackUpgrade(client, storage, current, rel)
			}
			return rel, jobErr
		}
	}
	if opts.Wait {
		timeout := opts.Timeout
		if 0 == timeout {
			timeout = 5 * time.Minute
		}
		if waitErr := client.WaitForReady(manifest, timeout); nil != waitErr {
			rel.Status = release.StatusFailed
			_ = storage.Update(rel)
			// CleanupOnFail: roll back the partial upgrade by re-applying the
			// previous revision's manifest. Resources introduced by this
			// upgrade and not in the previous revision are deleted.
			if opts.CleanupOnFail {
				if "" != current.Manifest {
					if reapplyErr := client.ApplyManifests(current.Manifest); nil != reapplyErr {
						logger.Warn("cleanup-on-fail upgrade: re-apply previous manifest: %v", reapplyErr)
					}
				}
			}
			if opts.Atomic {
				atomicRollbackUpgrade(client, storage, current, rel)
			}
			return rel, waitErr
		}
	}

	// Step 9: Execute post-upgrade hooks
	var postResults []release.HookResult
	var postErr error
	if !opts.NoHooks {
		postResults, postErr = hooks.ExecuteHooksWithTimeout(client, parsedHooks, hooks.PostUpgrade, opts.HookTimeout)
	}
	rel.Hooks = append(rel.Hooks, postResults...)
	if nil != postErr {
		rel.Status = release.StatusFailed
		_ = storage.Update(rel)
		if opts.Atomic {
			atomicRollbackUpgrade(client, storage, current, rel)
		}
		return rel, postErr
	}

	// Step 10: Mark new revision deployed, old superseded
	rel.Status = release.StatusDeployed
	if updateErr := storage.Update(rel); nil != updateErr {
		return rel, updateErr
	}

	current.Status = release.StatusSuperseded
	if updateErr := storage.Update(current); nil != updateErr {
		logger.Warn("failed to mark previous revision as superseded: %v", updateErr)
	}

	// Step 11: Optional history pruning.
	if 0 < opts.HistoryMax {
		pruneReleaseHistory(storage, opts.ReleaseName, opts.HistoryMax)
	}

	return rel, nil
}

// pruneReleaseHistory keeps the most recent maxKeep revisions and deletes
// older entries. The currently-deployed revision is always preserved even if
// older than the cutoff.
func pruneReleaseHistory(storage release.Storage, name string, maxKeep int) {
	history, err := storage.History(name)
	if nil != err {
		logger.Warn("history pruning skipped: %v", err)
		return
	}
	if len(history) <= maxKeep {
		return
	}
	// History is sorted ascending by revision; oldest at index 0.
	cutoff := len(history) - maxKeep
	for i := 0; i < cutoff; i++ {
		rel := history[i]
		if release.StatusDeployed == rel.Status {
			continue
		}
		if delErr := storage.Delete(rel.Name, rel.Revision); nil != delErr {
			logger.Warn("failed to prune revision %d of %s: %v", rel.Revision, rel.Name, delErr)
		}
	}
}

// atomicRollbackUpgrade re-applies the previous revision's manifests when
// atomic mode is enabled and an upgrade fails. It also deletes resources
// that only exist in the failed manifest (not in the previous one).
func atomicRollbackUpgrade(client kube.KubeClient, storage release.Storage, previous *release.Release, failed *release.Release) {
	logger.Warn("atomic upgrade failed, rolling back to revision %d for %s", previous.Revision, previous.Name)

	// Delete resources that exist only in the failed manifest (new resources from the upgrade).
	orphaned, orphanErr := computeOrphanedManifest(failed.Manifest, previous.Manifest)
	if nil != orphanErr {
		logger.Warn("atomic rollback: failed to compute orphaned resources: %v", orphanErr)
	} else if "" != orphaned {
		if delErr := client.DeleteManifests(orphaned); nil != delErr {
			logger.Warn("atomic rollback: failed to delete orphaned resources: %v", delErr)
		}
	}

	if applyErr := client.ApplyManifests(previous.Manifest); nil != applyErr {
		logger.Warn("atomic rollback: failed to re-apply previous manifests: %v", applyErr)
	}
	failed.Status = release.StatusFailed
	_ = storage.Update(failed)

	previous.Status = release.StatusDeployed
	_ = storage.Update(previous)
}

// resourceKey returns a unique identifier for a Kubernetes resource.
func resourceKey(obj *unstructured.Unstructured) string {
	return obj.GroupVersionKind().String() + "/" + obj.GetNamespace() + "/" + obj.GetName()
}

// computeOrphanedManifest returns the YAML for resources present in failedManifest
// but absent from previousManifest. These are resources created by the failed upgrade
// that must be cleaned up during rollback.
func computeOrphanedManifest(failedManifest, previousManifest string) (string, error) {
	failedResources, err := kube.ParseManifests(failedManifest)
	if nil != err {
		return "", err
	}

	previousResources, err := kube.ParseManifests(previousManifest)
	if nil != err {
		return "", err
	}

	previousKeys := make(map[string]bool, len(previousResources))
	for _, obj := range previousResources {
		previousKeys[resourceKey(obj)] = true
	}

	var orphanedParts []string
	for _, obj := range failedResources {
		if previousKeys[resourceKey(obj)] {
			continue
		}
		data, marshalErr := obj.MarshalJSON()
		if nil != marshalErr {
			return "", keramoserr.WrapError(keramoserr.ErrKube, "failed to marshal orphaned resource", marshalErr)
		}
		orphanedParts = append(orphanedParts, string(data))
	}

	if 0 == len(orphanedParts) {
		return "", nil
	}

	result := ""
	for i, part := range orphanedParts {
		if i > 0 {
			result += "\n---\n"
		}
		result += part
	}
	return result, nil
}
