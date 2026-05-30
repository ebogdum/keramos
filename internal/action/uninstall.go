package action

import (
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/hooks"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/release"
)

// UninstallOptions configures an uninstall operation.
type UninstallOptions struct {
	ReleaseName     string
	Namespace       string
	KeepHistory     bool
	Timeout         time.Duration
	NoHooks         bool
	Description     string
	IgnoreNotFound  bool
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
	var parsedHooks []hooks.Hook
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
