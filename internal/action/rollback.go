package action

import (
	"fmt"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/hooks"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/release"
)

// RollbackOptions configures a rollback operation.
type RollbackOptions struct {
	ReleaseName   string
	Namespace     string
	Revision      int // target revision (0 = previous)
	Wait          bool
	Timeout       time.Duration
	Description   string
	NoHooks       bool
	HistoryMax    int
	Force         bool
	CleanupOnFail bool
	RecreatePods  bool
}

// Rollback reverts a release to a previous revision.
func Rollback(client kube.KubeClient, opts *RollbackOptions) (*release.Release, error) {
	if err := ValidateReleaseName(opts.ReleaseName); nil != err {
		return nil, err
	}

	ns := opts.Namespace
	if "" == ns {
		ns = client.Namespace()
	}

	storage := release.NewSecretStorage(client.Clientset(), ns)

	// Get current release
	current, err := storage.Last(opts.ReleaseName)
	if nil != err {
		return nil, err
	}

	// Step 1: Determine target revision
	targetRevision := opts.Revision
	if 0 == targetRevision {
		targetRevision = current.Revision - 1
	}
	if 1 > targetRevision {
		return nil, keramoserr.NewError(keramoserr.ErrRelease, "no previous revision to rollback to")
	}

	logger.Debug("rolling back %s from revision %d to %d", opts.ReleaseName, current.Revision, targetRevision)

	target, err := storage.Get(opts.ReleaseName, targetRevision)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "target revision %d not found", targetRevision)
	}

	// Step 2: Create new revision from target's manifest/values
	newRevision := current.Revision + 1
	now := time.Now().UTC()

	description := opts.Description
	if "" == description {
		description = fmt.Sprintf("Rollback to %d", targetRevision)
	}

	rel := &release.Release{
		Name:      opts.ReleaseName,
		Namespace: ns,
		Revision:  newRevision,
		Status:    release.StatusPendingRollback,
		Package:   target.Package,
		Values:    target.Values,
		Manifest:  target.Manifest,
		Notes:     target.Notes,
		Info: release.ReleaseInfo{
			FirstDeployed: current.Info.FirstDeployed,
			LastDeployed:  now,
			Description:   description,
		},
	}

	if storeErr := storage.Create(rel); nil != storeErr {
		return nil, storeErr
	}

	// Re-parse hooks from the target revision's stored templates so
	// pre/post-rollback hooks fire with the correct manifest.
	var parsedHooks []hooks.Hook
	if 0 < len(target.HookTemplates) {
		parsed, parseErr := hooks.ParseHooks(target.HookTemplates)
		if nil != parseErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, parseErr, "failed to parse hooks for revision %d", targetRevision)
		}
		parsedHooks = parsed
	} else if 0 < len(target.Hooks) {
		logger.Warn("target revision %d has no stored hook templates; rollback cannot re-execute hooks (release pre-dates HookTemplates field)", targetRevision)
	}

	// Step 3: Execute pre-rollback hooks
	var preResults []release.HookResult
	var preErr error
	if !opts.NoHooks {
		preResults, preErr = hooks.ExecuteHooks(client, parsedHooks, hooks.PreRollback)
	}
	if nil != preErr {
		rel.Status = release.StatusFailed
		rel.Hooks = preResults
		_ = storage.Update(rel)
		return rel, preErr
	}
	rel.Hooks = append(rel.Hooks, preResults...)

	// Step 4: Apply target revision's manifests
	if opts.Force {
		if delErr := client.DeleteManifests(current.Manifest); nil != delErr {
			logger.Warn("force rollback: pre-delete reported error: %v", delErr)
		}
	}
	if applyErr := client.ApplyManifests(target.Manifest); nil != applyErr {
		rel.Status = release.StatusFailed
		_ = storage.Update(rel)
		if opts.CleanupOnFail {
			if cleanErr := client.DeleteManifests(target.Manifest); nil != cleanErr {
				logger.Warn("cleanup-on-fail: %v", cleanErr)
			}
		}
		return rel, applyErr
	}
	if opts.RecreatePods {
		if rrErr := recreatePodsForManifest(client, target.Manifest); nil != rrErr {
			logger.Warn("recreate-pods: %v", rrErr)
		}
	}

	// Step 5: Wait if requested
	if opts.Wait {
		timeout := opts.Timeout
		if 0 == timeout {
			timeout = 5 * time.Minute
		}
		if waitErr := client.WaitForReady(target.Manifest, timeout); nil != waitErr {
			rel.Status = release.StatusFailed
			_ = storage.Update(rel)
			return rel, waitErr
		}
	}

	// Step 6: Execute post-rollback hooks
	var postResults []release.HookResult
	var postErr error
	if !opts.NoHooks {
		postResults, postErr = hooks.ExecuteHooks(client, parsedHooks, hooks.PostRollback)
	}
	rel.Hooks = append(rel.Hooks, postResults...)
	if nil != postErr {
		rel.Status = release.StatusFailed
		_ = storage.Update(rel)
		return rel, postErr
	}

	// Step 7: Mark new revision deployed, current superseded
	rel.Status = release.StatusDeployed
	if updateErr := storage.Update(rel); nil != updateErr {
		return rel, updateErr
	}

	current.Status = release.StatusSuperseded
	if updateErr := storage.Update(current); nil != updateErr {
		logger.Warn("failed to mark current revision as superseded: %v", updateErr)
	}

	if 0 < opts.HistoryMax {
		pruneReleaseHistory(storage, opts.ReleaseName, opts.HistoryMax)
	}

	return rel, nil
}
