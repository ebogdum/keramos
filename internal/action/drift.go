package action

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StateAndLiveManifests returns a release's stored manifest ("state") and the
// live cluster objects for the resources in it ("running"), as two combined
// manifests. It is the data source for the three-way `keramos drift` view.
// Resources absent from the cluster are omitted from the live manifest.
func StateAndLiveManifests(client kube.KubeClient, releaseName string) (state string, live string, err error) {
	storage, storageErr := release.SelectStorage(client.Clientset(), client.Namespace())
	if nil != storageErr {
		return "", "", storageErr
	}
	current, cErr := storage.Last(releaseName)
	if nil != cErr {
		return "", "", cErr
	}
	liveManifest, lErr := collectLiveManifest(client, current.Manifest, current.Namespace)
	if nil != lErr {
		return "", "", lErr
	}
	return current.Manifest, liveManifest, nil
}

// collectLiveManifest fetches the live object for every resource in a manifest
// and joins them into one manifest, defaulting namespace-scoped resources to
// defaultNS. Not-found resources are skipped (their absence is visible as a
// resource missing from the running side).
func collectLiveManifest(client kube.KubeClient, manifest, defaultNS string) (string, error) {
	resources, err := kube.ParseManifests(manifest)
	if nil != err {
		return "", err
	}
	docs := make([]string, 0, len(resources))
	for _, want := range resources {
		if "" == want.GetNamespace() && "" != defaultNS {
			want.SetNamespace(defaultNS)
		}
		live, fErr := fetchLive(client, want)
		if nil != fErr {
			if isNotFound(fErr) {
				continue
			}
			return "", keramoserr.WrapErrorf(keramoserr.ErrKube, fErr, "fetch %s/%s", want.GetKind(), want.GetName())
		}
		docs = append(docs, marshalUnstructured(live))
	}
	// marshalUnstructured emits compact JSON with no trailing newline, so the
	// separator must carry its own leading newline; "---\n" alone would glue
	// the marker to the previous "}" and the YAML decoder would reject it,
	// silently dropping every live resource after the first.
	return strings.Join(docs, "\n---\n"), nil
}

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
	Kind           DriftKind
	APIVersion     string
	ResourceKind   string
	Namespace      string
	Name           string
	FieldDiffs     []FieldDiff
	StoredManifest string
	LiveManifest   string
}

// FieldDiff is a single per-field drift, suitable for human display.
type FieldDiff struct {
	Path string
	Want any // value from stored manifest
	Got  any // value observed in cluster
}

// DriftAgainstManifest compares a manifest against the live cluster state and
// returns each diverging resource. Keramos-managed-only fields are considered:
// server-defaulted fields and runtime status are ignored. Used by the
// reconcile path to decide what to converge.
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
					Kind:           DriftMissing,
					APIVersion:     want.GetAPIVersion(),
					ResourceKind:   want.GetKind(),
					Namespace:      want.GetNamespace(),
					Name:           want.GetName(),
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
			compareLists(path, wm, lvList, out)
		default:
			if !reflect.DeepEqual(wv, lv) {
				*out = append(*out, FieldDiff{Path: path, Want: wv, Got: lv})
			}
		}
	}
}

func compareLists(path string, want, live []any, out *[]FieldDiff) {
	if len(want) != len(live) {
		*out = append(*out, FieldDiff{Path: path, Want: want, Got: live})
		return
	}

	for i := range want {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		switch wantItem := want[i].(type) {
		case map[string]any:
			liveItem, ok := live[i].(map[string]any)
			if !ok {
				*out = append(*out, FieldDiff{Path: itemPath, Want: wantItem, Got: live[i]})
				continue
			}
			walkCompare(itemPath, wantItem, liveItem, out)
		case []any:
			liveItem, ok := live[i].([]any)
			if !ok {
				*out = append(*out, FieldDiff{Path: itemPath, Want: wantItem, Got: live[i]})
				continue
			}
			compareLists(itemPath, wantItem, liveItem, out)
		default:
			if !reflect.DeepEqual(want[i], live[i]) {
				*out = append(*out, FieldDiff{Path: itemPath, Want: want[i], Got: live[i]})
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
	storage, storageErr := release.SelectStorage(client.Clientset(), client.Namespace())
	if nil != storageErr {
		return nil, storageErr
	}
	current, err := storage.Last(releaseName)
	if nil != err {
		return nil, err
	}
	driftItems, err := driftAgainstManifestInNamespace(client, current.Manifest, current.Namespace)
	if nil != err {
		return nil, err
	}
	if 0 == len(driftItems) {
		return nil, nil
	}
	// Honour the documented contract: resources annotated resource-policy: keep
	// are not re-applied (their drift is intentionally preserved).
	applyManifest, filterErr := stripKeepPolicy(current.Manifest)
	if nil != filterErr {
		return nil, filterErr
	}
	if "" != applyManifest {
		if applyErr := client.ApplyManifests(applyManifest); nil != applyErr {
			return nil, applyErr
		}
		if waitErr := client.WaitForReady(applyManifest, timeout); nil != waitErr {
			return nil, waitErr
		}
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

// stripKeepPolicy returns the manifest with resources annotated
// resource-policy: keep removed, so reconcile does not re-apply (and thus
// preserve) their intentionally-drifted state.
func stripKeepPolicy(manifest string) (string, error) {
	resources, err := kube.ParseManifests(manifest)
	if nil != err {
		return "", err
	}
	docs := make([]string, 0, len(resources))
	for _, r := range resources {
		anns := r.GetAnnotations()
		if "keep" == anns["keramos.sh/resource-policy"] || "keep" == anns["helm.sh/resource-policy"] {
			continue
		}
		docs = append(docs, marshalUnstructured(r))
	}
	return strings.Join(docs, "\n---\n"), nil
}
