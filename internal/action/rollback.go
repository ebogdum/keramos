package action

import (
	"fmt"
	"time"

	"github.com/ebogdum/keramos/v3/internal/audit"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/hooks"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/release"
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

	storage, storageErr := release.SelectStorage(client.Clientset(), ns)
	if nil != storageErr {
		return nil, storageErr
	}

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
		// Carry the target revision's full record forward, matching Upgrade.
		// Dropping these left the rollback revision with no audit trail, lost
		// user labels, and empty Tests/HookTemplates — so `keramos test` and a
		// later rollback-to-this-revision could no longer run.
		UserValues:    target.UserValues,
		Provenance:    target.Provenance,
		Manifest:      target.Manifest,
		Tests:         target.Tests,
		HookTemplates: target.HookTemplates,
		Notes:         target.Notes,
		Labels:        target.Labels,
		Audit:         audit.Capture("rollback", targetRevision),
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
		rel.Hooks = preResults
		return rel, combineFailure(preErr, markFailed(storage, rel))
	}
	rel.Hooks = append(rel.Hooks, preResults...)

	// Step 4: Apply target revision's manifests
	if opts.Force {
		if delErr := client.DeleteManifests(current.Manifest); nil != delErr {
			// Pre-delete failed: abort rather than apply over a partially
			// deleted state and report only a misleading apply error.
			return rel, combineFailure(
				keramoserr.WrapErrorf(keramoserr.ErrKube, delErr, "force rollback aborted: pre-delete of current resources failed"),
				markFailed(storage, rel))
		}
	}
	if applyErr := client.ApplyManifests(target.Manifest); nil != applyErr {
		sec := markFailed(storage, rel)
		if opts.CleanupOnFail {
			if cleanErr := client.DeleteManifests(target.Manifest); nil != cleanErr {
				sec = append(sec, "cleanup-on-fail delete failed: "+cleanErr.Error())
			}
		}
		return rel, combineFailure(applyErr, sec)
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
			return rel, combineFailure(waitErr, markFailed(storage, rel))
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
		return rel, combineFailure(postErr, markFailed(storage, rel))
	}

	// Step 7: Mark new revision deployed, all older ones superseded.
	rel.Status = release.StatusDeployed
	if updateErr := updateReleaseWithRetry(storage, rel); nil != updateErr {
		return rel, updateErr
	}
	supersedeOtherDeployed(storage, opts.ReleaseName, rel.Revision)

	if 0 < opts.HistoryMax {
		pruneReleaseHistory(storage, opts.ReleaseName, opts.HistoryMax)
	}

	return rel, nil
}
