package action

import (
	"os"
	"strings"
	"time"

	"github.com/ebogdum/keramos/internal/audit"
	"github.com/ebogdum/keramos/internal/deptree"
	"github.com/ebogdum/keramos/internal/engine"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/hooks"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/layer"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/otel"
	"github.com/ebogdum/keramos/internal/policy"
	"github.com/ebogdum/keramos/internal/release"
	"github.com/ebogdum/keramos/internal/values"
	"gopkg.in/yaml.v3"
)

// InstallOptions configures an install operation.
type InstallOptions struct {
	ReleaseName     string
	Namespace       string
	ValueFiles      []string
	Sets            []string
	SetStrings      []string
	SetFiles        []string
	SetJSON         []string
	Profile         string
	Wait            bool
	Timeout         time.Duration
	DryRun          string
	Description     string
	Atomic          bool
	NoHooks         bool
	CreateNamespace bool
	IncludeCRDs     bool
	Labels          map[string]string
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

	// Step 2: Resolve full merged values (defaults + overrides).
	mergedValues, err := values.ResolveAll(map[string]any(resolved.Values), opts.ValueFiles, opts.Sets, opts.SetStrings, opts.SetFiles, opts.SetJSON)
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
		Values:     mergedValues,
		UserValues: userValues,
		Manifest:      manifest,
		Notes:         notes,
		Tests:         renderedTests,
		HookTemplates: renderedHooks,
		Labels:        opts.Labels,
		Audit:         audit.Capture("install", 0),
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
		rel.Status = release.StatusFailed
		rel.Hooks = preResults
		_ = storage.Update(rel)
		if opts.Atomic {
			atomicCleanupInstall(client, storage, rel, manifest)
		}
		return rel, preErr
	}
	rel.Hooks = append(rel.Hooks, preResults...)

	// Step 9a: Apply CRDs first and wait for Established.
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

	// Step 9b: Apply remaining manifests.
	// For --cleanup-on-fail, fresh installs always introduce every resource,
	// so a per-resource set isn't needed — the whole manifest is "ours".
	if applyErr := client.ApplyManifests(manifest); nil != applyErr {
		rel.Status = release.StatusFailed
		_ = storage.Update(rel)
		if opts.CleanupOnFail {
			if cleanErr := client.DeleteManifests(manifest); nil != cleanErr {
				logger.Warn("cleanup-on-fail: %v", cleanErr)
			}
		}
		if opts.Atomic {
			atomicCleanupInstall(client, storage, rel, manifest)
		}
		return rel, applyErr
	}

	// Step 10: Wait for readiness
	if opts.Wait {
		timeout := opts.Timeout
		if 0 == timeout {
			timeout = 5 * time.Minute
		}
		if opts.WaitForJobs {
			if jobErr := waitForJobsInManifest(client, manifest, timeout); nil != jobErr {
				rel.Status = release.StatusFailed
				_ = storage.Update(rel)
				if opts.CleanupOnFail {
					if cleanErr := client.DeleteManifests(manifest); nil != cleanErr {
						logger.Warn("cleanup-on-fail: %v", cleanErr)
					}
				}
				if opts.Atomic {
					atomicCleanupInstall(client, storage, rel, manifest)
				}
				return rel, jobErr
			}
		}
		if waitErr := client.WaitForReady(manifest, timeout); nil != waitErr {
			rel.Status = release.StatusFailed
			_ = storage.Update(rel)
			if opts.CleanupOnFail {
				if cleanErr := client.DeleteManifests(manifest); nil != cleanErr {
					logger.Warn("cleanup-on-fail: %v", cleanErr)
				}
			}
			if opts.Atomic {
				atomicCleanupInstall(client, storage, rel, manifest)
			}
			return rel, waitErr
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
		rel.Status = release.StatusFailed
		_ = storage.Update(rel)
		if opts.Atomic {
			atomicCleanupInstall(client, storage, rel, manifest)
		}
		return rel, postErr
	}

	// Step 12: Mark deployed
	rel.Status = release.StatusDeployed
	if updateErr := storage.Update(rel); nil != updateErr {
		return rel, updateErr
	}

	// Step 13: Install required co-deployed packages
	if !opts.SkipRequires {
		if reqErr := installRequires(client, packagePath, opts); nil != reqErr {
			logger.Warn("failed to install required packages: %v", reqErr)
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
				// Write scoped values to a temp file and pass as value file
				tmpFile, tmpErr := writeScopedValuesTemp(scoped)
				if nil == tmpErr {
					scopedValueFiles = append(scopedValueFiles, tmpFile)
					defer func(f string) { _ = os.Remove(f) }(tmpFile)
				}
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


// atomicCleanupInstall deletes all applied manifests and marks the release failed
// when atomic mode is enabled and an error occurs.
func atomicCleanupInstall(client kube.KubeClient, storage release.Storage, rel *release.Release, manifest string) {
	logger.Warn("atomic install failed, cleaning up resources for %s", rel.Name)
	if delErr := client.DeleteManifests(manifest); nil != delErr {
		logger.Warn("atomic cleanup: failed to delete manifests: %v", delErr)
	}
	rel.Status = release.StatusFailed
	_ = storage.Update(rel)
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
