package action

import (
	"context"
	"fmt"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// recreatePodsForManifest forces a rolling restart of every workload in the
// manifest by patching its spec.template.metadata.annotations with a fresh
// timestamp. This is the underlying behaviour of the --recreate-pods flag.
func recreatePodsForManifest(client kube.KubeClient, manifest string) error {
	resources, err := kube.ParseManifests(manifest)
	if nil != err {
		return err
	}
	clientset := client.Clientset()
	if nil == clientset {
		return keramoserr.NewError(keramoserr.ErrKube, "recreate-pods requires a clientset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	patch := []byte(fmt.Sprintf(
		`{"spec":{"template":{"metadata":{"annotations":{"keramos.sh/restartedAt":%q}}}}}`,
		stamp))

	for _, obj := range resources {
		ns := obj.GetNamespace()
		if "" == ns {
			ns = client.Namespace()
		}
		name := obj.GetName()
		switch obj.GetKind() {
		case "Deployment":
			_, _ = clientset.AppsV1().Deployments(ns).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		case "StatefulSet":
			_, _ = clientset.AppsV1().StatefulSets(ns).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		case "DaemonSet":
			_, _ = clientset.AppsV1().DaemonSets(ns).Patch(ctx, name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		}
	}
	return nil
}
