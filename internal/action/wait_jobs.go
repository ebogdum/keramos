package action

import (
	"time"

	"github.com/ebogdum/keramos/v2/internal/kube"
)

// waitForJobsInManifest blocks until every Job resource in the manifest
// reaches Complete=True (or the timeout expires). Used by --wait-for-jobs
// which is a separate semantic from --wait (which waits for Pods/Deployments).
func waitForJobsInManifest(client kube.KubeClient, manifest string, timeout time.Duration) error {
	resources, err := kube.ParseManifests(manifest)
	if nil != err {
		return err
	}
	for _, obj := range resources {
		if "Job" != obj.GetKind() {
			continue
		}
		ns := obj.GetNamespace()
		if "" == ns {
			ns = client.Namespace()
		}
		if waitErr := client.WaitForJob(ns, obj.GetName(), timeout); nil != waitErr {
			return waitErr
		}
	}
	return nil
}
