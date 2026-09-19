package action

import (
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/hooks"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// UninstallOptions configures an uninstall operation.
type UninstallOptions struct {
	ReleaseName    string
	Namespace      string
	KeepHistory    bool
	Timeout        time.Duration
	NoHooks        bool
	Description    string
	IgnoreNotFound bool
	// Wait blocks until every deleted resource is actually gone from the
	// cluster (bounded by Timeout), instead of returning as soon as the DELETE
	// calls are issued.
	Wait bool
}

// Uninstall removes a release and its resources from the cluster.
// Returns the final release state before deletion for output purposes.
func Uninstall(client kube.KubeClient, opts *UninstallOptions) (*release.Release, error) {
	if err := ValidateReleaseName(opts.ReleaseName); nil != err {
		return nil, err
	}

	ns := opts.Namespace
	if "" == ns {
		ns = client.Namespace()
	}

	storage := release.NewSecretStorage(client.Clientset(), ns)

	// Step 1: Get latest release
	current, err := storage.Last(opts.ReleaseName)
	if nil != err {
		if opts.IgnoreNotFound {
			logger.Debug("release %s not found; --ignore-not-found in effect, exiting cleanly",
				opts.ReleaseName)
			return &release.Release{Name: opts.ReleaseName, Namespace: ns}, nil
		}
		return nil, err
	}
	if "" != opts.Description {
		current.Info.Description = opts.Description
	}

	logger.Debug("uninstalling release %s revision %d", opts.ReleaseName, current.Revision)

	// Step 2: Mark as uninstalling
	current.Status = release.StatusUninstalling
	if updateErr := storage.Update(current); nil != updateErr {
		return nil, updateErr
	}

	// Step 3: Execute pre-delete hooks (no stored hooks for uninstall, use empty)
	parsedHooks, hookParseErr := hooks.ParseHooks(current.HookTemplates)
	if nil != hookParseErr {
		return current, combineFailure(hookParseErr, markFailed(storage, current))
	}

	var preResults []release.HookResult
	var preErr error
	if !opts.NoHooks {
		preResults, preErr = hooks.ExecuteHooks(client, parsedHooks, hooks.PreDelete)
	}
	if nil != preErr {
		current.Hooks = preResults
		return current, combineFailure(preErr, markFailed(storage, current))
	}

	// Step 4: Delete all manifests (reverse install order handled by DeleteManifests)
	if delErr := client.DeleteManifests(current.Manifest); nil != delErr {
		return current, combineFailure(delErr, markFailed(storage, current))
	}

	// Step 4b: Optionally block until the deleted resources are actually gone.
	if opts.Wait {
		timeout := opts.Timeout
		if 0 == timeout {
			timeout = 5 * time.Minute
		}
		if waitErr := waitForDeletion(client, current.Manifest, ns, timeout); nil != waitErr {
			return current, combineFailure(waitErr, markFailed(storage, current))
		}
	}

	// Step 5: Execute post-delete hooks
	var postResults []release.HookResult
	var postErr error
	if !opts.NoHooks {
		postResults, postErr = hooks.ExecuteHooks(client, parsedHooks, hooks.PostDelete)
	}
	current.Hooks = append(current.Hooks, postResults...)
	if nil != postErr {
		return current, combineFailure(postErr, markFailed(storage, current))
	}

	// Step 6: Delete release history unless --keep-history
	if !opts.KeepHistory {
		history, histErr := storage.History(opts.ReleaseName)
		if nil != histErr {
			return current, keramoserr.WrapError(keramoserr.ErrRelease, "failed to get release history for cleanup", histErr)
		}
		for _, rel := range history {
			if delErr := storage.Delete(rel.Name, rel.Revision); nil != delErr {
				logger.Warn("failed to delete release secret for %s v%d: %v", rel.Name, rel.Revision, delErr)
			}
		}
	} else {
		current.Status = release.StatusSuperseded
		if updateErr := storage.Update(current); nil != updateErr {
			return current, keramoserr.WrapError(keramoserr.ErrRelease, "uninstall completed but failed to persist kept-history status", updateErr)
		}
	}

	logger.Debug("uninstall of %s complete", opts.ReleaseName)
	return current, nil
}

// waitForDeletion polls until every resource in the manifest is gone from the
// cluster or the timeout elapses. A resource is considered deleted when Lookup
// returns nil (not found).
func waitForDeletion(client kube.KubeClient, manifest, defaultNS string, timeout time.Duration) error {
	all, err := kube.ParseManifests(manifest)
	if nil != err {
		return err
	}
	// Only wait for resources that DeleteManifests actually deletes. Resources
	// annotated resource-policy: keep are intentionally left behind, so waiting
	// on them would time out forever and report a false uninstall failure.
	resources := make([]*unstructured.Unstructured, 0, len(all))
	for _, r := range all {
		anns := r.GetAnnotations()
		if "keep" == anns["keramos.sh/resource-policy"] || "keep" == anns["helm.sh/resource-policy"] {
			continue
		}
		resources = append(resources, r)
	}
	deadline := time.Now().Add(timeout)
	for {
		remaining := 0
		for _, r := range resources {
			ns := r.GetNamespace()
			if "" == ns {
				ns = defaultNS
			}
			obj, lErr := client.Lookup(r.GetAPIVersion(), r.GetKind(), ns, r.GetName())
			if nil == lErr && nil != obj {
				remaining++
			}
		}
		if 0 == remaining {
			return nil
		}
		if time.Now().After(deadline) {
			return keramoserr.NewErrorf(keramoserr.ErrKube,
				"timed out after %s waiting for %d resource(s) to be deleted", timeout, remaining)
		}
		time.Sleep(2 * time.Second)
	}
}
