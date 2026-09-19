package action

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ebogdum/keramos/v3/internal/audit"
	"github.com/ebogdum/keramos/v3/internal/deptree"
	"github.com/ebogdum/keramos/v3/internal/engine"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/hooks"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/layer"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/otel"
	"github.com/ebogdum/keramos/v3/internal/policy"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/ebogdum/keramos/v3/internal/values"
	"gopkg.in/yaml.v3"
)

// InstallOptions configures an install operation.
type InstallOptions struct {
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
	Description         string
	Atomic              bool
	NoHooks             bool
	CreateNamespace     bool
	IncludeCRDs         bool
	Labels              map[string]string
	APIVersions         []string
	KubeVersion         string
	PostRenderer        string        // single command (legacy)
	PostRenderers       []string      // chained post-renderers (preferred)
	PostRendererTimeout time.Duration // 0 = default
	CleanupOnFail       bool
	WaitForJobs         bool
	HookTimeout         time.Duration
	Environment         string // keramos.yaml `environments:` key (replaces values-{env}.yaml)
	SkipRequires        bool
	HistoryMax          int  // 0 = unlimited; cap retained revisions
	RecreatePods        bool // rolling-restart Deployments/StatefulSets/DaemonSets
	Force               bool // delete-and-recreate immutable resources
}

// Install deploys a keramos package as a new release.
func Install(client kube.KubeClient, packagePath string, opts *InstallOptions) (rel *release.Release, retErr error) {
	span := otel.Start("install")
	span.SetAttr("release", opts.ReleaseName)
	span.SetAttr("packagePath", packagePath)
	defer func() { span.EndWithError(retErr) }()

	if err := ValidateReleaseName(opts.ReleaseName); nil != err {
		return nil, err
	}

	logger.Debug("installing release %s from %s", opts.ReleaseName, packagePath)

	// Step 1: If --env was supplied, fold environment-declared overrides
	// into the install options BEFORE values resolution so they participate
	// in layer filtering and merging.
	if "" != opts.Environment {
		envApplyErr := applyEnvironmentToInstall(packagePath, opts)
		if nil != envApplyErr {
			return nil, envApplyErr
		}
	}

	// Step 1a: Pre-resolve user-supplied values so sub-chart conditions /
	// tags can be evaluated against the user's --set / -f overrides during
	// layer filtering.
	userValues, err := values.ResolveAll(map[string]any{}, opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
	if nil != err {
		return nil, err
	}

	// Step 1b: Resolve the package (filtering disabled layers using userValues).
	resolved, err := layer.ResolveWithOverrides(packagePath, opts.Profile, userValues)
	if nil != err {
		return nil, err
	}

	// Step 2: Resolve full merged values (defaults + overrides), keeping the
	// resolution trace so we can record where each value came from in the state.
	mergedValues, valueTrace, err := values.ResolveAllWithTrace(map[string]any(resolved.Values), opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
	if nil != err {
		return nil, err
	}

	// Step 2c: Validate values against the package's JSON schema if present.
	if schemaErr := ValidateValuesAgainstSchema(packagePath, mergedValues); nil != schemaErr {
		return nil, schemaErr
	}

	ns := opts.Namespace
	if "" == ns && nil != client {
		ns = client.Namespace()
	}
	if "" == ns {
		ns = "default"
	}

	// Step 3: Build render context
	var capabilities map[string]any
	if nil != client && "" == opts.DryRun {
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

	ctx := &engine.RenderContext{
		Values: mergedValues,
		Package: map[string]any{
			"name":       resolved.Metadata.Name,
			"version":    resolved.Metadata.Version,
			"appVersion": resolved.Metadata.AppVersion,
		},
		Release: map[string]any{
			"name":      opts.ReleaseName,
			"namespace": ns,
			"revision":  1,
			"isUpgrade": false,
			"isInstall": true,
		},
		Capabilities: capabilities,
		Files:        resolved.Files,
	}
	if nil != client && "" == opts.DryRun {
		ctx.Lookup = client.Lookup
	}

	// Step 4: Render templates
	eng := engine.New()
	manifest, err := eng.Render(resolved.Templates, resolved.Partials, ctx)
	if nil != err {
		return nil, err
	}

	// Step 5: Render and parse hooks
	renderedHooks, err := renderHooks(eng, resolved.Hooks, resolved.Partials, ctx)
	if nil != err {
		return nil, err
	}

	parsedHooks, err := hooks.ParseHooks(renderedHooks)
	if nil != err {
		return nil, err
	}

	// Render tests so `keramos test` can re-apply them later.
	renderedTests, testsErr := renderHooks(eng, resolved.Tests, resolved.Partials, ctx)
	if nil != testsErr {
		return nil, testsErr
	}

	// Extract notes from rendered output
	manifest, notes, notesErr := extractNotes(manifest)
	if nil != notesErr {
		return nil, notesErr
	}

	// Step 5b: Evaluate package policies against the rendered manifest. Deny
	// violations abort; warns log. A load error is a hard failure so a
	// corrupt rule file cannot silently disable the security gate.
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

	// Optionally include CRDs from the crds/ directory. They are tracked on
	// the release record (so uninstall/get sees them) but applied in a
	// separate phase below so they are Established before dependent manifests.
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
	rel = &release.Release{
		Name:      opts.ReleaseName,
		Namespace: ns,
		Revision:  1,
		Status:    release.StatusPendingInstall,
		Package: release.PackageRef{
			Name:       resolved.Metadata.Name,
			Version:    resolved.Metadata.Version,
			AppVersion: resolved.Metadata.AppVersion,
		},
		Values:        mergedValues,
		UserValues:    userValues,
		Provenance:    valueTrace.Provenance(),
		Manifest:      manifest,
		Notes:         notes,
		Tests:         renderedTests,
		HookTemplates: renderedHooks,
		Labels:        opts.Labels,
		Audit:         audit.WithValueFiles(audit.WithFlags(audit.Capture("install", 0), auditFlags(opts)), opts.ValueFiles),
		Info: release.ReleaseInfo{
			FirstDeployed: now,
			LastDeployed:  now,
			Description:   opts.Description,
		},
	}

	// Dry run: return rendered manifests without applying
	if "client" == opts.DryRun {
		rel.Status = release.StatusPendingInstall
		return rel, nil
	}
	if "server" == opts.DryRun {
		rel.Status = release.StatusPendingInstall
		if nil == client {
			return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "server-side dry-run requires a cluster connection")
		}
		if dryRunErr := client.DryRunApply(manifest); nil != dryRunErr {
			return rel, dryRunErr
		}
		return rel, nil
	}

	// Step 6: Ensure namespace exists when --create-namespace is set.
	if opts.CreateNamespace {
		if nsErr := client.CreateNamespace(ns); nil != nsErr {
			return nil, nsErr
		}
	}

	// Step 7: Store release as pending
	storage := release.NewSecretStorage(client.Clientset(), ns)
	if storeErr := storage.Create(rel); nil != storeErr {
		return nil, storeErr
	}

	// Step 8: Execute pre-install hooks
	var preResults []release.HookResult
	var preErr error
	if !opts.NoHooks {
		preResults, preErr = hooks.ExecuteHooksWithTimeout(client, parsedHooks, hooks.PreInstall, opts.HookTimeout)
	}
	if nil != preErr {
		rel.Hooks = preResults
		return failInstall(client, storage, rel, manifest, opts, false, preErr)
	}
	rel.Hooks = append(rel.Hooks, preResults...)

	// Step 9a: Apply CRDs first and wait for Established.
	if "" != crdManifest {
		crdTimeout := opts.Timeout
		if 0 == crdTimeout {
			crdTimeout = 2 * time.Minute
		}
		if crdErr := client.ApplyCRDs(crdManifest, crdTimeout); nil != crdErr {
			return failInstall(client, storage, rel, manifest, opts, false, crdErr)
		}
	}

	// Step 9b: Apply remaining manifests.
	// For --cleanup-on-fail, fresh installs always introduce every resource,
	// so a per-resource set isn't needed — the whole manifest is "ours".
	if applyErr := client.ApplyManifests(manifest); nil != applyErr {
		return failInstall(client, storage, rel, manifest, opts, true, applyErr)
	}

	// Step 10: Wait for readiness
	if opts.Wait {
		timeout := opts.Timeout
		if 0 == timeout {
			timeout = 5 * time.Minute
		}
		if opts.WaitForJobs {
			if jobErr := waitForJobsInManifest(client, manifest, timeout); nil != jobErr {
				return failInstall(client, storage, rel, manifest, opts, true, jobErr)
			}
		}
		if waitErr := client.WaitForReady(manifest, timeout); nil != waitErr {
			return failInstall(client, storage, rel, manifest, opts, true, waitErr)
		}
	}

	// Step 11: Execute post-install hooks
	var postResults []release.HookResult
	var postErr error
	if !opts.NoHooks {
		postResults, postErr = hooks.ExecuteHooksWithTimeout(client, parsedHooks, hooks.PostInstall, opts.HookTimeout)
	}
	rel.Hooks = append(rel.Hooks, postResults...)
	if nil != postErr {
		return failInstall(client, storage, rel, manifest, opts, true, postErr)
	}

	// Step 12: Mark deployed (retry so a transient write error doesn't leave a
	// fully-applied release stuck Pending).
	rel.Status = release.StatusDeployed
	if updateErr := updateReleaseWithRetry(storage, rel); nil != updateErr {
		return rel, updateErr
	}

	if opts.RecreatePods {
		if rrErr := recreatePodsForManifest(client, manifest); nil != rrErr {
			logger.Warn("pod recreation reported: %v", rrErr)
		}
	}

	if 0 < opts.HistoryMax {
		pruneReleaseHistory(storage, opts.ReleaseName, opts.HistoryMax)
	}

	// Step 13: Install required co-deployed packages. A failure here is
	// surfaced (not just logged): the primary release IS deployed — rel is
	// returned — but the caller must know a declared dependency did not install.
	if !opts.SkipRequires {
		if reqErr := installRequires(client, packagePath, opts); nil != reqErr {
			return rel, keramoserr.WrapError(keramoserr.ErrDependency,
				"release deployed but a required package failed to install", reqErr)
		}
	}

	return rel, nil
}

// installRequires installs co-deployed packages declared in the requires field.
// Uses the dependency tree to discover all requires and pass scoped values.
func installRequires(client kube.KubeClient, packagePath string, opts *InstallOptions) error {
	root, buildErr := layer.BuildTree(packagePath)
	if nil != buildErr {
		return buildErr
	}

	// Populate tree to access parent scoped values
	if popErr := deptree.Populate(root); nil != popErr {
		return popErr
	}

	requireNodes := deptree.WalkRequires(root)
	if 0 == len(requireNodes) {
		return nil
	}

	ns := opts.Namespace
	if "" == ns && nil != client {
		ns = client.Namespace()
	}
	if "" == ns {
		ns = "default"
	}

	storage := release.NewSecretStorage(client.Clientset(), ns)

	for _, reqNode := range requireNodes {
		// Check if already installed
		_, lastErr := storage.Last(reqNode.Name)
		if nil == lastErr {
			logger.Log("required package %s already installed, skipping", reqNode.Name)
			continue
		}

		logger.Log("installing required package %s from %s", reqNode.Name, reqNode.Source)

		// Build scoped values from parent's requires.<name> section
		var scopedValueFiles []string
		var scopedSets []string
		if nil != reqNode.Parent {
			parentValues := reqNode.Parent.Values
			scoped := deptree.ScopedRequireValues(parentValues, reqNode.Name)
			if 0 < len(scoped) {
				// Write scoped values to a temp file and pass as value file.
				// A write failure must abort — otherwise the required package is
				// installed WITHOUT its parent's requires.<name> overrides,
				// silently applying wrong/default configuration.
				tmpFile, tmpErr := writeScopedValuesTemp(scoped)
				if nil != tmpErr {
					return keramoserr.WrapErrorf(keramoserr.ErrDependency, tmpErr,
						"failed to write scoped values for required package %s", reqNode.Name)
				}
				scopedValueFiles = append(scopedValueFiles, tmpFile)
				defer func(f string) { _ = os.Remove(f) }(tmpFile)
			}
		}

		reqOpts := &InstallOptions{
			ReleaseName:  reqNode.Name,
			Namespace:    ns,
			ValueFiles:   scopedValueFiles,
			Sets:         scopedSets,
			Wait:         opts.Wait,
			Timeout:      opts.Timeout,
			Atomic:       opts.Atomic,
			SkipRequires: false,
		}

		if _, installErr := Install(client, reqNode.Source, reqOpts); nil != installErr {
			return keramoserr.WrapErrorf(keramoserr.ErrDependency, installErr,
				"failed to install required package %s", reqNode.Name)
		}
	}

	return nil
}

func writeScopedValuesTemp(vals map[string]any) (string, error) {
	tmpFile, err := os.CreateTemp("", "keramos-scoped-values-*.yaml")
	if nil != err {
		return "", err
	}

	data, marshalErr := yaml.Marshal(vals)
	if nil != marshalErr {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", marshalErr
	}

	if _, writeErr := tmpFile.Write(data); nil != writeErr {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", writeErr
	}

	return tmpFile.Name(), tmpFile.Close()
}

// failInstall records an install failure. It marks the release Failed and
// persists that status, optionally removes the applied manifest (when atomic
// or cleanup-on-fail is set and resources were actually applied), and returns
// an error wrapping the original cause TOGETHER WITH any secondary failure (a
// dropped status write or a failed cleanup). This ensures the caller never
// reports a plain failure when the cluster or the release record may have been
// left inconsistent.
// auditFlags renders a REDACTED list of the salient flags for the audit trail.
// It records --set KEYS only (never their values, which routinely carry
// secrets), profile/environment, and enabled boolean flags. Value-file paths
// are recorded separately via WithValueFiles.
func auditFlags(opts *InstallOptions) []string {
	var f []string
	for _, s := range opts.Sets {
		f = append(f, "--set "+redactSetValue(s))
	}
	for _, s := range opts.SetStrings {
		f = append(f, "--set-string "+redactSetValue(s))
	}
	for _, s := range opts.SetJSON {
		f = append(f, "--set-json "+redactSetValue(s))
	}
	for _, s := range opts.SetFiles {
		f = append(f, "--set-file "+s)
	}
	if "" != opts.Profile {
		f = append(f, "--profile "+opts.Profile)
	}
	if "" != opts.Environment {
		f = append(f, "--environment "+opts.Environment)
	}
	for name, on := range map[string]bool{
		"--atomic": opts.Atomic, "--wait": opts.Wait, "--wait-for-jobs": opts.WaitForJobs,
		"--force": opts.Force, "--cleanup-on-fail": opts.CleanupOnFail, "--no-hooks": opts.NoHooks,
		"--create-namespace": opts.CreateNamespace, "--include-crds": opts.IncludeCRDs,
		"--recreate-pods": opts.RecreatePods, "--skip-requires": opts.SkipRequires,
	} {
		if on {
			f = append(f, name)
		}
	}
	sort.Strings(f)
	return f
}

// redactSetValue keeps the key of a `key=value` override but replaces the
// value with [REDACTED] so secrets passed via --set never land in the trail.
func redactSetValue(s string) string {
	if eq := strings.IndexByte(s, '='); -1 != eq {
		return s[:eq] + "=[REDACTED]"
	}
	return s
}

// markFailed sets the release status to Failed and persists it. If the
// persist itself fails it returns a secondary-error description so the caller
// can surface that the release record may not reflect reality.
func markFailed(storage release.Storage, rel *release.Release) []string {
	rel.Status = release.StatusFailed
	if uErr := storage.Update(rel); nil != uErr {
		return []string{"release status not persisted: " + uErr.Error()}
	}
	return nil
}

// combineFailure folds any secondary failures (failed cleanup, dropped status
// write, failed rollback) into the primary cause so a partially-recovered
// operation never reports a clean error.
func combineFailure(cause error, secondary []string) error {
	if 0 == len(secondary) {
		return cause
	}
	return keramoserr.WrapErrorf(keramoserr.ErrRelease, cause, "operation failed and recovery was incomplete: %s", strings.Join(secondary, "; "))
}

func failInstall(client kube.KubeClient, storage release.Storage, rel *release.Release, manifest string, opts *InstallOptions, applied bool, cause error) (*release.Release, error) {
	var secondary []string
	rel.Status = release.StatusFailed
	if uErr := storage.Update(rel); nil != uErr {
		secondary = append(secondary, "release status not persisted: "+uErr.Error())
	}
	if applied && (opts.Atomic || opts.CleanupOnFail) {
		logger.Warn("install failed, removing applied resources for %s", rel.Name)
		if delErr := client.DeleteManifests(manifest); nil != delErr {
			secondary = append(secondary, "cleanup of applied resources failed (orphans may remain): "+delErr.Error())
		}
	}
	if 0 < len(secondary) {
		return rel, keramoserr.WrapErrorf(keramoserr.ErrRelease, cause, "install failed and recovery was incomplete: %s", strings.Join(secondary, "; "))
	}
	return rel, cause
}

func renderHooks(eng *engine.Engine, hookTemplates map[string]string, partials map[string]any, ctx *engine.RenderContext) (map[string]string, error) {
	rendered := make(map[string]string, len(hookTemplates))
	for name, tmpl := range hookTemplates {
		// Hook directives ($hook, $hookWeight, $hookDeletePolicy,
		// $hookTimeout) are stripped by the engine's cleanDollarKeys pass.
		// Capture and re-attach them so ParseHooks can read the directive
		// from the post-render content. Only the leading-document
		// directive lines are preserved; expressions inside them are not
		// expanded (operators don't typically need that).
		head, body := splitHookDirective(tmpl)
		docs, err := eng.RenderFile(name, body, partials, ctx)
		if nil != err {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, err, "failed to render hook %s", name)
		}
		joined := joinDocs(docs)
		if "" != head {
			joined = head + joined
		}
		rendered[name] = joined
	}
	return rendered, nil
}

// splitHookDirective extracts $hook* directive lines from the top of `s`,
// returning them as a directive-only YAML preface and the remaining body
// (which is what the engine renders). Recognises the same set of keys
// hooks.extractDirective consumes.
func splitHookDirective(s string) (head, body string) {
	lines := strings.SplitN(s, "\n", -1)
	headLines := []string{}
	bodyLines := []string{}
	inHead := true
	for _, line := range lines {
		if inHead {
			trimmed := strings.TrimLeft(line, " \t")
			if strings.HasPrefix(trimmed, "$hook") {
				headLines = append(headLines, line)
				continue
			}
			if "" == strings.TrimSpace(trimmed) && 0 == len(headLines) {
				bodyLines = append(bodyLines, line)
				continue
			}
			inHead = false
		}
		bodyLines = append(bodyLines, line)
	}
	if 0 == len(headLines) {
		return "", s
	}
	return strings.Join(headLines, "\n") + "\n", strings.Join(bodyLines, "\n")
}

func joinDocs(docs []string) string {
	if 0 == len(docs) {
		return ""
	}
	result := docs[0]
	for i := 1; i < len(docs); i++ {
		result += "---\n" + docs[i]
	}
	return result
}
