package kube

import (
	"context"
	"reflect"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ImmutableFieldsForKind enumerates the spec field paths that are immutable
// for each common Kubernetes kind. The values are JSON paths into spec.
// Non-exhaustive; covers the kinds where the upgrade `--force` path needs
// to recreate-on-divergence.
var ImmutableFieldsForKind = map[string][][]string{
	"Service":     {{"clusterIP"}, {"type"}, {"selector"}},
	"Job":         {{"selector"}, {"template"}},
	"StatefulSet": {{"serviceName"}, {"selector"}, {"volumeClaimTemplates"}, {"podManagementPolicy"}},
	"PersistentVolume":      {{"capacity"}, {"persistentVolumeReclaimPolicy"}},
	"PersistentVolumeClaim": {{"accessModes"}, {"resources", "requests"}, {"storageClassName"}, {"volumeName"}},
}

// ResourcesNeedingForce returns the set of resource keys (gvk|namespace|name)
// from the desired manifest whose immutable fields differ from the
// currently-deployed instance. Used by upgrade/rollback `--force` so only
// the truly-divergent resources are deleted-and-recreated, leaving the rest
// to a normal server-side apply.
func (c *Client) ResourcesNeedingForce(manifest string) (map[string]bool, error) {
	desired, err := ParseManifests(manifest)
	if nil != err {
		return nil, err
	}
	out := make(map[string]bool, len(desired))
	for _, want := range desired {
		paths, ok := ImmutableFieldsForKind[want.GetKind()]
		if !ok {
			continue
		}
		current, getErr := c.fetchCurrent(want)
		if nil != getErr || nil == current {
			continue // not present yet — apply will create it normally.
		}
		for _, path := range paths {
			a, _, _ := unstructured.NestedFieldNoCopy(want.Object, append([]string{"spec"}, path...)...)
			b, _, _ := unstructured.NestedFieldNoCopy(current.Object, append([]string{"spec"}, path...)...)
			if !reflect.DeepEqual(a, b) {
				out[resourceKey(want)] = true
				break
			}
		}
	}
	return out, nil
}

func (c *Client) fetchCurrent(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	gvr, err := c.resourceForObj(obj)
	if nil != err {
		return nil, err
	}
	ns := c.resolveNamespace(obj)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if "" == ns {
		return c.dynamic.Resource(gvr).Get(ctx, obj.GetName(), metav1.GetOptions{})
	}
	return c.dynamic.Resource(gvr).Namespace(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
}
