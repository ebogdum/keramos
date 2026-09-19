package action

import (
	"github.com/ebogdum/keramos/v2/internal/kube"
)

// newResourcesOnly returns a key set for resources in `manifest` that are NOT
// in `preExisting`. Used by --cleanup-on-fail so only resources introduced by
// the failed operation are deleted, leaving previously existing resources
// untouched.
//
// If preExisting is nil (snapshot failed), returns nil — caller should fall
// back to deleting the whole manifest or skip cleanup.
func newResourcesOnly(_ kube.KubeClient, manifest string, preExisting map[string]bool) map[string]bool {
	if nil == preExisting {
		// Without a snapshot we cannot distinguish new from existing.
		// Returning nil signals "no per-resource scoping available".
		return nil
	}
	resources, err := kube.ParseManifests(manifest)
	if nil != err {
		return nil
	}
	out := make(map[string]bool, len(resources))
	for _, obj := range resources {
		key := obj.GetAPIVersion() + "|" + obj.GetKind() + "|" + obj.GetNamespace() + "|" + obj.GetName()
		if !preExisting[key] {
			out[key] = true
		}
	}
	return out
}
