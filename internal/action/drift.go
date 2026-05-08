package action

import (
	"context"
	"reflect"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DriftKind classifies a drifted resource.
type DriftKind int

const (
	DriftMissing DriftKind = iota // resource declared by keramos but absent in cluster
	DriftMutated                  // present but spec differs from stored manifest
	DriftExtra                    // labelled by keramos but not in the manifest (rare; ignored unless requested)
)

func (k DriftKind) String() string {
	switch k {
	case DriftMissing:
		return "missing"
	case DriftMutated:
		return "mutated"
	case DriftExtra:
		return "extra"
	}
	return "unknown"
}

// DriftItem records a single drift finding.
type DriftItem struct {
	Kind          DriftKind
	APIVersion    string
	ResourceKind  string
	Namespace     string
	Name          string
	FieldDiffs    []FieldDiff
	StoredManifest string
	LiveManifest   string
}

// FieldDiff is a single per-field drift, suitable for human display.
type FieldDiff struct {
	Path string
	Want any // value from stored manifest
	Got  any // value observed in cluster
}

// Drift compares the stored manifest of a release against the live cluster
// state and returns each diverging resource. Keramos-managed-only fields are
// considered: server-defaulted fields and runtime status are ignored.
//
// This is the primitive behind `keramos drift <release>`. Operators detect
// post-install changes (kubectl edit, controller mutations) without rendering
// the package again.
func Drift(client kube.KubeClient, releaseName string) ([]DriftItem, error) {
	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
	current, err := storage.Last(releaseName)
	if nil != err {
		return nil, err
	}
	return driftAgainstManifestInNamespace(client, current.Manifest, current.Namespace)
}

// DriftAgainstManifest is the variant that takes a manifest directly,
// useful for the controller and `keramos drift --manifest`.
func DriftAgainstManifest(client kube.KubeClient, manifest string) ([]DriftItem, error) {
	return driftAgainstManifestInNamespace(client, manifest, "")
}

// driftAgainstManifestInNamespace is the internal variant that defaults
// any namespace-scoped resource without an explicit `metadata.namespace`
// to `defaultNS`. The release-storage drift path passes the release's
// namespace; the public DriftAgainstManifest passes empty (preserves
// historical behaviour).
func driftAgainstManifestInNamespace(client kube.KubeClient, manifest, defaultNS string) ([]DriftItem, error) {
	resources, err := kube.ParseManifests(manifest)
	if nil != err {
		return nil, err
	}
	if "" != defaultNS {
		for _, r := range resources {
			if "" == r.GetNamespace() {
				r.SetNamespace(defaultNS)
			}
		}
	}
	out := make([]DriftItem, 0)
	for _, want := range resources {
		live, fetchErr := fetchLive(client, want)
		if nil != fetchErr {
			if isNotFound(fetchErr) {
				out = append(out, DriftItem{
					Kind:          DriftMissing,
					APIVersion:    want.GetAPIVersion(),
					ResourceKind:  want.GetKind(),
					Namespace:     want.GetNamespace(),
					Name:          want.GetName(),
					StoredManifest: marshalUnstructured(want),
				})
				continue
			}
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, fetchErr, "fetch %s/%s", want.GetKind(), want.GetName())
		}
		fields := compareSpec(want.Object, live.Object)
		if 0 < len(fields) {
			out = append(out, DriftItem{
				Kind:           DriftMutated,
				APIVersion:     want.GetAPIVersion(),
				ResourceKind:   want.GetKind(),
				Namespace:      want.GetNamespace(),
				Name:           want.GetName(),
				FieldDiffs:     fields,
				StoredManifest: marshalUnstructured(want),
				LiveManifest:   marshalUnstructured(live),
			})
		}
	}
	return out, nil
}

func fetchLive(client kube.KubeClient, want *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	apiVersion := want.GetAPIVersion()
	obj, err := client.Lookup(apiVersion, want.GetKind(), want.GetNamespace(), want.GetName())
	if nil != err {
		return nil, err
	}
	if nil == obj {
		return nil, &liveNotFoundErr{}
	}
	return &unstructured.Unstructured{Object: obj}, nil
}

type liveNotFoundErr struct{}

func (e *liveNotFoundErr) Error() string { return "resource not found in cluster" }

func isNotFound(err error) bool {
	if nil == err {
		return false
	}
	if _, ok := err.(*liveNotFoundErr); ok {
		return true
	}
	// kube.Lookup may return wrapped k8s NotFound; the live-not-found
	// sentinel covers our internal path. Real K8s NotFound surfaces as nil
	// from Lookup per its contract.
	return false
}

func compareSpec(want, live map[string]any) []FieldDiff {
	wantSpec, _ := want["spec"].(map[string]any)
	liveSpec, _ := live["spec"].(map[string]any)
	out := make([]FieldDiff, 0)
	walkCompare("spec", wantSpec, liveSpec, &out)

	// Compare labels and annotations the chart explicitly set.
	if wantMeta, ok := want["metadata"].(map[string]any); ok {
		if liveMeta, ok := live["metadata"].(map[string]any); ok {
			compareMapEntries("metadata.labels", asMap(wantMeta["labels"]), asMap(liveMeta["labels"]), &out)
			compareMapEntries("metadata.annotations", asMap(wantMeta["annotations"]), asMap(liveMeta["annotations"]), &out)
		}
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// compareMapEntries reports drift only for keys present in `want` — controller
// or webhook-injected keys in `live` are ignored.
func compareMapEntries(prefix string, want, live map[string]any, out *[]FieldDiff) {
	for k, wv := range want {
		lv, exists := live[k]
		if !exists {
			*out = append(*out, FieldDiff{Path: prefix + "." + k, Want: wv, Got: nil})
			continue
		}
		if !reflect.DeepEqual(wv, lv) {
			*out = append(*out, FieldDiff{Path: prefix + "." + k, Want: wv, Got: lv})
		}
	}
}

// walkCompare recurses through `want` checking that every key/leaf has the
// expected value in `live`. Extra keys in `live` are ignored — server-side
// apply and admission controllers add fields the chart did not declare.
func walkCompare(prefix string, want, live map[string]any, out *[]FieldDiff) {
	for k, wv := range want {
		path := prefix + "." + k
		lv, exists := live[k]
		if !exists {
			*out = append(*out, FieldDiff{Path: path, Want: wv, Got: nil})
			continue
		}
		switch wm := wv.(type) {
		case map[string]any:
			lm, _ := lv.(map[string]any)
			walkCompare(path, wm, lm, out)
		case []any:
			lvList, _ := lv.([]any)
			if !reflect.DeepEqual(wm, lvList) {
				*out = append(*out, FieldDiff{Path: path, Want: wm, Got: lvList})
			}
		default:
			if !reflect.DeepEqual(wv, lv) {
				*out = append(*out, FieldDiff{Path: path, Want: wv, Got: lv})
			}
		}
	}
}

func marshalUnstructured(u *unstructured.Unstructured) string {
	b, err := u.MarshalJSON()
	if nil != err {
		return ""
	}
	return string(b)
}

// Reconcile re-applies a release's stored manifest to the cluster, returning
// the list of resources that were re-converged. Resources marked `keep` via
// the resource-policy annotation are not touched.
func Reconcile(client kube.KubeClient, releaseName string, timeout time.Duration) ([]string, error) {
	if 0 == timeout {
		timeout = 5 * time.Minute
	}
	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())
	current, err := storage.Last(releaseName)
	if nil != err {
		return nil, err
	}
	driftItems, err := DriftAgainstManifest(client, current.Manifest)
	if nil != err {
		return nil, err
	}
	if 0 == len(driftItems) {
		return nil, nil
	}
	if applyErr := client.ApplyManifests(current.Manifest); nil != applyErr {
		return nil, applyErr
	}
	if waitErr := client.WaitForReady(current.Manifest, timeout); nil != waitErr {
		return nil, waitErr
	}
	out := make([]string, 0, len(driftItems))
	for _, d := range driftItems {
		out = append(out, d.ResourceKind+"/"+d.Name)
	}
	return out, nil
}

// keep apimachinery imported even if unused after refactors.
var _ = metav1.GetOptions{}
var _ = context.Background
