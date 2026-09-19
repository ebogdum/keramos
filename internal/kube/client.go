package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/ebogdum/keramos/v3/internal/engine"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	keramoslabels "github.com/ebogdum/keramos/v3/internal/labels"
	"github.com/ebogdum/keramos/v3/internal/logger"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const defaultTimeout = 5 * time.Minute

const (
	defaultClientQPS   = 50
	defaultClientBurst = 100
)

// KubeClient is the interface for Kubernetes cluster operations used by
// action/, hooks/, and release/ packages. Enables mock testing.
type KubeClient interface {
	Namespace() string
	Clientset() kubernetes.Interface
	ApplyManifests(manifests string) error
	Dynamic() (dynamic.Interface, error)
	ApplyCRDs(manifests string, timeout time.Duration) error
	DeleteManifests(manifests string) error
	DeleteResources(manifests string, only map[string]bool) error
	SnapshotResources(manifests string) (map[string]bool, error)
	ResourcesNeedingForce(manifests string) (map[string]bool, error)
	WaitForReady(manifests string, timeout time.Duration) error
	GetCapabilities() (map[string]any, error)
	CreateNamespace(name string) error
	WaitForJob(namespace, name string, timeout time.Duration) error
	DryRunApply(manifests string) error
	Lookup(apiVersion, kind, namespace, name string) (map[string]any, error)
	SetTimeout(d time.Duration)
	SetForce(force bool)
}

// Client wraps Kubernetes API access.
type Client struct {
	clientset        kubernetes.Interface
	dynamic          dynamic.Interface
	config           *rest.Config
	namespace        string
	discovery        discovery.DiscoveryInterface
	timeout          time.Duration
	forceApply       bool
	releaseName      string
	releaseNamespace string
	takeOwnership    bool
	mapperMu         sync.RWMutex
	cachedMapper     meta.RESTMapper
}

// NewClient creates a Client from kubeconfig path, context name, and namespace.
// If kubeconfig is empty, uses default loading rules (KUBECONFIG env, ~/.kube/config).
// If context is empty, uses current context.
func NewClient(kubeconfig, kubeContext, namespace string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if "" != kubeconfig {
		loadingRules.ExplicitPath = kubeconfig
	}

	overrides := &clientcmd.ConfigOverrides{}
	if "" != kubeContext {
		overrides.CurrentContext = kubeContext
	}
	if "" != namespace {
		overrides.Context.Namespace = namespace
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	config, err := clientConfig.ClientConfig()
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to load kubeconfig", err)
	}

	if 0 == config.QPS {
		config.QPS = defaultClientQPS
		config.Burst = defaultClientBurst
	}

	clientset, err := kubernetes.NewForConfig(config)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to create kubernetes clientset", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to create dynamic client", err)
	}

	ns := namespace
	if "" == ns {
		configNs, _, err := clientConfig.Namespace()
		if nil != err {
			ns = "default"
		} else {
			ns = configNs
		}
	}

	return &Client{
		clientset:  clientset,
		dynamic:    dynClient,
		config:     config,
		namespace:  ns,
		discovery:  clientset.Discovery(),
		timeout:    defaultTimeout,
		forceApply: true,
	}, nil
}

// SetTimeout configures the default timeout for all API calls.
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// SetForce controls whether server-side apply uses Force mode to take field ownership.
func (c *Client) SetForce(force bool) {
	c.forceApply = force
}

// Namespace returns the configured namespace.
func (c *Client) Namespace() string {
	return c.namespace
}

// Clientset returns the underlying Kubernetes clientset.
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// Dynamic returns the dynamic client for unstructured CR access. Used by the
// KeramosRelease controller to list/update arbitrary CRDs without typed clients.
func (c *Client) Dynamic() (dynamic.Interface, error) {
	return c.dynamic, nil
}

// newContext creates a context with the client's configured timeout.
func (c *Client) newContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.timeout)
}

// ApplyManifests applies YAML manifests to the cluster using server-side apply.
func (c *Client) ApplyManifests(manifests string) error {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return err
	}

	sorted := SortByInstallOrder(resources)

	for _, obj := range sorted {
		if applyErr := c.applyResource(obj); nil != applyErr {
			return applyErr
		}
		// A CRD placed in templates/ (rather than crds/) is applied here in
		// order, but a custom resource OF that CRD later in the same manifest
		// cannot be mapped until the API server is serving the kind. Wait for
		// Established and refresh the mapper before continuing, so the CR maps.
		if "CustomResourceDefinition" == obj.GetKind() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			estErr := c.waitForCRDEstablished(ctx, obj.GetName())
			cancel()
			if nil != estErr {
				return keramoserr.WrapErrorf(keramoserr.ErrKube, estErr,
					"CRD %s did not become Established", obj.GetName())
			}
			if _, refreshErr := c.refreshMapper(); nil != refreshErr {
				logger.Warn("failed to refresh REST mapper after applying CRD %s: %v", obj.GetName(), refreshErr)
			}
		}
	}

	return nil
}

// Lookup performs a live cluster read for the `lookup` template function.
// An empty `name` returns a list (objects under "items"); a missing resource
// returns (nil, nil) so callers can guard with conditionals.
//
// Secret-kind reads are logged at WARN level: lookup of a Secret embeds
// the (base64-encoded) Secret data into the rendered manifest, which is
// then stored in the release record. Anyone with read access to the
// release record now reads any Secret the operator's RBAC permits at
// install time, possibly broader than the consumer's own access.
func (c *Client) Lookup(apiVersion, kind, namespace, name string) (map[string]any, error) {
	if "Secret" == kind {
		logger.Warn("lookup: reading Secret %s/%s — value will be embedded in the rendered manifest and stored in the release record", namespace, name)
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, err, "lookup: invalid apiVersion %q", apiVersion)
	}
	gvk := gv.WithKind(kind)

	mapper, mapperErr := c.ensureMapper()
	if nil != mapperErr {
		return nil, mapperErr
	}
	mapping, mapErr := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if nil != mapErr {
		// The mapper is cached. A long-running controller that calls
		// Lookup for a CRD installed AFTER keramos started would otherwise
		// permanently fail. Force a discovery refresh and retry once.
		fresh, fErr := c.refreshMapper()
		if nil != fErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, mapErr, "lookup: cannot map %s/%s", apiVersion, kind)
		}
		mapping, mapErr = fresh.RESTMapping(gvk.GroupKind(), gvk.Version)
		if nil != mapErr {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, mapErr, "lookup: cannot map %s/%s", apiVersion, kind)
		}
	}
	gvr := mapping.Resource
	isNamespaced := meta.RESTScopeNameNamespace == mapping.Scope.Name()

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if "" == name {
		var list *unstructured.UnstructuredList
		var listErr error
		switch {
		case "" != namespace && isNamespaced:
			list, listErr = c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
		case isNamespaced:
			// Namespace-scoped kind with empty namespace: list across all
			// namespaces using the cluster-scoped client (which the dynamic
			// client interprets as cross-namespace for namespaced resources).
			list, listErr = c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
		default:
			list, listErr = c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
		}
		if nil != listErr {
			if k8serrors.IsNotFound(listErr) {
				return nil, nil
			}
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, listErr, "lookup: list failed for %s/%s", apiVersion, kind)
		}
		return list.UnstructuredContent(), nil
	}

	var obj *unstructured.Unstructured
	var getErr error
	if "" != namespace {
		obj, getErr = c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, getErr = c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if nil != getErr {
		if k8serrors.IsNotFound(getErr) {
			return nil, nil
		}
		return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, getErr, "lookup: get failed for %s/%s/%s", apiVersion, kind, name)
	}
	return obj.Object, nil
}

// DryRunApply sends manifests to the K8s API with server-side dry-run validation
// without persisting any changes.
func (c *Client) DryRunApply(manifests string) error {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return err
	}

	sorted := SortByInstallOrder(resources)

	for _, obj := range sorted {
		if applyErr := c.dryRunApplyResource(obj); nil != applyErr {
			return applyErr
		}
	}

	return nil
}

func (c *Client) dryRunApplyResource(obj *unstructured.Unstructured) error {
	gvr, err := c.resourceForObj(obj)
	if nil != err {
		return err
	}

	// Stamp managedBy=keramos so dry-run validates the same object that a
	// real apply would persist. Without this, server-side validation
	// could pass for an unlabelled object that the real apply path
	// would then mutate, in theory letting an admission webhook see
	// different data between dry-run and real apply.
	stampKeramosManaged(obj, c.releaseName, c.releaseNamespace)

	ns := c.resolveNamespace(obj)
	data, err := obj.MarshalJSON()
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrKube, "failed to marshal resource", err)
	}

	logger.Debug("dry-run applying %s/%s in namespace %s", obj.GetKind(), obj.GetName(), ns)

	var resource dynamic.ResourceInterface
	if "" != ns {
		resource = c.dynamic.Resource(gvr).Namespace(ns)
	} else {
		resource = c.dynamic.Resource(gvr)
	}

	ctx, cancel := c.newContext()
	defer cancel()

	_, applyErr := resource.Patch(
		ctx,
		obj.GetName(),
		types.ApplyPatchType,
		data,
		metav1.PatchOptions{
			FieldManager: "keramos",
			Force:        boolPtr(c.forceApply),
			DryRun:       []string{metav1.DryRunAll},
		},
	)
	if nil != applyErr {
		return keramoserr.WrapErrorf(keramoserr.ErrKube, applyErr, "server dry-run failed for %s/%s", obj.GetKind(), obj.GetName())
	}

	return nil
}

// ServerSideDiff performs a server-side apply DRY-RUN for each resource in
// manifests and returns two manifest strings: the current LIVE objects, and
// the objects the API server WOULD produce after applying. Diffing live vs
// merged is true server-side diff (kubectl-diff semantics): the merged side
// reflects API-server defaulting, admission-webhook mutation, and other
// server-managed fields — not just keramos's locally stored copy. Resources that
// do not yet exist contribute an empty live side (i.e. a pure creation).
func (c *Client) ServerSideDiff(manifests string) (liveManifest, mergedManifest string, err error) {
	resources, parseErr := ParseManifests(manifests)
	if nil != parseErr {
		return "", "", parseErr
	}
	sorted := SortByInstallOrder(resources)
	liveDocs := make([]string, 0, len(sorted))
	mergedDocs := make([]string, 0, len(sorted))
	for _, obj := range sorted {
		live, merged, rErr := c.serverSideDiffResource(obj)
		if nil != rErr {
			return "", "", rErr
		}
		if "" != strings.TrimSpace(live) {
			liveDocs = append(liveDocs, live)
		}
		if "" != strings.TrimSpace(merged) {
			mergedDocs = append(mergedDocs, merged)
		}
	}
	return strings.Join(liveDocs, "---\n"), strings.Join(mergedDocs, "---\n"), nil
}

func (c *Client) serverSideDiffResource(obj *unstructured.Unstructured) (live, merged string, err error) {
	gvr, mapErr := c.resourceForObj(obj)
	if nil != mapErr {
		return "", "", mapErr
	}
	stampKeramosManaged(obj, c.releaseName, c.releaseNamespace)
	ns := c.resolveNamespace(obj)

	var resource dynamic.ResourceInterface
	if "" != ns {
		resource = c.dynamic.Resource(gvr).Namespace(ns)
	} else {
		resource = c.dynamic.Resource(gvr)
	}

	ctx, cancel := c.newContext()
	defer cancel()

	// Current live object (absent → a creation, empty live side).
	liveObj, getErr := resource.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if nil == getErr {
		live = unstructuredToYAML(liveObj)
	} else if !k8serrors.IsNotFound(getErr) {
		return "", "", keramoserr.WrapErrorf(keramoserr.ErrKube, getErr, "get live %s/%s", obj.GetKind(), obj.GetName())
	}

	data, mErr := obj.MarshalJSON()
	if nil != mErr {
		return "", "", keramoserr.WrapError(keramoserr.ErrKube, "failed to marshal resource", mErr)
	}
	mergedObj, applyErr := resource.Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "keramos",
		Force:        boolPtr(c.forceApply),
		DryRun:       []string{metav1.DryRunAll},
	})
	if nil != applyErr {
		return "", "", keramoserr.WrapErrorf(keramoserr.ErrKube, applyErr, "server dry-run failed for %s/%s", obj.GetKind(), obj.GetName())
	}
	return live, unstructuredToYAML(mergedObj), nil
}

// unstructuredToYAML marshals an object to YAML, returning "" on error so a
// single unmarshalable object degrades gracefully rather than aborting a diff.
func unstructuredToYAML(obj *unstructured.Unstructured) string {
	if nil == obj {
		return ""
	}
	out, err := yaml.Marshal(obj.Object)
	if nil != err {
		return ""
	}
	return string(out)
}

// ApplyCRDs applies the CustomResourceDefinitions in `manifests` and waits
// for each to reach Established=true before returning. This is the
// installation phase for the `crds/` directory: those types must exist
// before any other resource in the package can reference them.
//
// Non-CRD documents in the input are ignored so the caller can pass a mixed
// manifest safely.
func (c *Client) ApplyCRDs(manifests string, timeout time.Duration) error {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return err
	}
	crds := make([]*unstructured.Unstructured, 0)
	for _, obj := range resources {
		if "CustomResourceDefinition" == obj.GetKind() {
			crds = append(crds, obj)
		}
	}
	if 0 == len(crds) {
		return nil
	}

	for _, obj := range crds {
		if applyErr := c.applyResource(obj); nil != applyErr {
			return keramoserr.WrapErrorf(keramoserr.ErrKube, applyErr, "failed to apply CRD %s", obj.GetName())
		}
	}

	if 0 == timeout {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, obj := range crds {
		name := obj.GetName()
		if err := c.waitForCRDEstablished(ctx, name); nil != err {
			return err
		}
	}
	return nil
}

func (c *Client) waitForCRDEstablished(ctx context.Context, name string) error {
	gvr := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		obj, getErr := c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if nil != getErr {
			if k8serrors.IsNotFound(getErr) {
				return false, nil
			}
			return false, getErr
		}
		conds, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
		if !found {
			return false, nil
		}
		for _, c := range conds {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if "Established" == cm["type"] && "True" == cm["status"] {
				return true, nil
			}
		}
		return false, nil
	})
}

// SnapshotResources returns a set of resource keys (gvk|namespace|name) that
// already exist in the cluster from the given manifest. Used by upgrade's
// --cleanup-on-fail to scope cleanup to resources the operation introduced.
func (c *Client) SnapshotResources(manifests string) (map[string]bool, error) {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return nil, err
	}
	out := make(map[string]bool, len(resources))
	for _, obj := range resources {
		gvr, err := c.resourceForObj(obj)
		if nil != err {
			continue
		}
		ns := c.resolveNamespace(obj)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		var getErr error
		if "" == ns {
			_, getErr = c.dynamic.Resource(gvr).Get(ctx, obj.GetName(), metav1.GetOptions{})
		} else {
			_, getErr = c.dynamic.Resource(gvr).Namespace(ns).Get(ctx, obj.GetName(), metav1.GetOptions{})
		}
		cancel()
		if nil == getErr {
			out[resourceKey(obj)] = true
		}
	}
	return out, nil
}

// DeleteResources deletes only those resources from `manifests` whose
// resource-key appears in `only` (or all resources when only is nil).
func (c *Client) DeleteResources(manifests string, only map[string]bool) error {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return err
	}
	sorted := SortByUninstallOrder(resources)
	for _, obj := range sorted {
		annotations := obj.GetAnnotations()
		if "keep" == annotations["helm.sh/resource-policy"] || "keep" == annotations["keramos.sh/resource-policy"] {
			continue
		}
		if nil != only && !only[resourceKey(obj)] {
			continue
		}
		if delErr := c.deleteResource(obj); nil != delErr {
			return delErr
		}
	}
	return nil
}

// resourceKey produces a stable identifier for a manifest object.
func resourceKey(obj *unstructured.Unstructured) string {
	return obj.GetAPIVersion() + "|" + obj.GetKind() + "|" + obj.GetNamespace() + "|" + obj.GetName()
}

// DeleteManifests deletes resources defined in the YAML manifests.
// Resources annotated with `keramos.sh/resource-policy: keep` (or the legacy
// `helm.sh/resource-policy: keep` key, accepted for inputs that already
// carry it) are preserved.
func (c *Client) DeleteManifests(manifests string) error {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return err
	}

	sorted := SortByUninstallOrder(resources)

	for _, obj := range sorted {
		annotations := obj.GetAnnotations()
		if "keep" == annotations["helm.sh/resource-policy"] || "keep" == annotations["keramos.sh/resource-policy"] {
			logger.Debug("preserving %s/%s due to resource-policy: keep", obj.GetKind(), obj.GetName())
			continue
		}
		if delErr := c.deleteResource(obj); nil != delErr {
			return delErr
		}
	}

	return nil
}

// WaitForReady waits for all resources in the manifests to become ready.
func (c *Client) WaitForReady(manifests string, timeout time.Duration) error {
	resources, err := ParseManifests(manifests)
	if nil != err {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, obj := range resources {
		if waitErr := c.waitForResource(ctx, obj); nil != waitErr {
			return waitErr
		}
	}

	return nil
}

// GetCapabilities returns cluster capabilities for template rendering.
func (c *Client) GetCapabilities() (map[string]any, error) {
	serverVersion, err := c.discovery.ServerVersion()
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to get server version", err)
	}

	apiGroups, apiResourceLists, err := c.discovery.ServerGroupsAndResources()
	if nil != err {
		// Partial results are acceptable
		logger.Warn("incomplete API discovery: %v", err)
	}

	groups := make([]string, 0, len(apiGroups))
	for _, g := range apiGroups {
		groups = append(groups, g.Name)
	}

	apiVersions := make([]string, 0)
	for _, rl := range apiResourceLists {
		apiVersions = append(apiVersions, rl.GroupVersion)
	}

	return map[string]any{
		"kubeVersion": &engine.KubeVersion{
			Version:    serverVersion.GitVersion,
			Major:      serverVersion.Major,
			Minor:      serverVersion.Minor,
			GitVersion: serverVersion.GitVersion,
		},
		"apiVersions": engine.NewVersionSet(apiVersions...),
		"groups":      groups,
	}, nil
}

func (c *Client) applyResource(obj *unstructured.Unstructured) error {
	gvr, err := c.resourceForObj(obj)
	if nil != err {
		return err
	}

	if ownerErr := c.checkOwnership(obj); nil != ownerErr {
		return ownerErr
	}

	// Stamp every applied resource with the keramos-managed label. This is
	// the source of truth that purge/list/drift use to recognise keramos's
	// own work — without it we'd be reduced to name pattern guessing.
	// Stamping is idempotent: setting a label that's already there is a
	// no-op for server-side apply.
	stampKeramosManaged(obj, c.releaseName, c.releaseNamespace)

	ns := c.resolveNamespace(obj)
	data, err := obj.MarshalJSON()
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrKube, "failed to marshal resource", err)
	}

	logger.Debug("applying %s/%s in namespace %s", obj.GetKind(), obj.GetName(), ns)

	var resource dynamic.ResourceInterface
	if "" != ns {
		resource = c.dynamic.Resource(gvr).Namespace(ns)
	} else {
		resource = c.dynamic.Resource(gvr)
	}

	ctx, cancel := c.newContext()
	defer cancel()

	_, applyErr := resource.Patch(
		ctx,
		obj.GetName(),
		types.ApplyPatchType,
		data,
		metav1.PatchOptions{
			FieldManager: "keramos",
			Force:        boolPtr(c.forceApply),
		},
	)
	if nil != applyErr {
		return keramoserr.WrapErrorf(keramoserr.ErrKube, applyErr, "failed to apply %s/%s", obj.GetKind(), obj.GetName())
	}

	return nil
}

func (c *Client) deleteResource(obj *unstructured.Unstructured) error {
	gvr, err := c.resourceForObj(obj)
	if nil != err {
		return err
	}

	ns := c.resolveNamespace(obj)

	logger.Debug("deleting %s/%s in namespace %s", obj.GetKind(), obj.GetName(), ns)

	var resource dynamic.ResourceInterface
	if "" != ns {
		resource = c.dynamic.Resource(gvr).Namespace(ns)
	} else {
		resource = c.dynamic.Resource(gvr)
	}

	ctx, cancel := c.newContext()
	defer cancel()

	deleteErr := resource.Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	if nil != deleteErr {
		if k8serrors.IsNotFound(deleteErr) {
			logger.Debug("resource %s/%s already deleted", obj.GetKind(), obj.GetName())
			return nil
		}
		return keramoserr.WrapErrorf(keramoserr.ErrKube, deleteErr, "failed to delete %s/%s", obj.GetKind(), obj.GetName())
	}

	return nil
}

func (c *Client) waitForResource(ctx context.Context, obj *unstructured.Unstructured) error {
	kind := obj.GetKind()

	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	switch kind {
	case "Deployment":
		return c.waitForDeployment(ctx, obj)
	case "StatefulSet":
		return c.waitForStatefulSet(ctx, obj)
	case "Pod":
		return c.waitForPod(ctx, obj)
	case "Service":
		return c.waitForService(ctx, obj)
	case "PersistentVolumeClaim":
		return c.waitForPVC(ctx, obj)
	case "Job":
		remaining := defaultTimeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining = time.Until(deadline)
		}
		if 0 >= remaining {
			remaining = defaultTimeout
		}
		return c.WaitForJob(ns, name, remaining)
	case "DaemonSet":
		return c.waitForDaemonSet(ctx, obj)
	default:
		logger.Debug("no readiness check for kind %s/%s, skipping", kind, name)
		return nil
	}
}

func (c *Client) waitForDeployment(ctx context.Context, obj *unstructured.Unstructured) error {
	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	if err := c.requireClientset(obj.GetKind(), name); nil != err {
		return err
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		dep, err := c.clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		// Strict readiness: status must reflect the current generation AND
		// every replica of the new revision must be updated, available,
		// and not outpaced by a still-rolling old replicaset. Without
		// checking UpdatedReplicas + ObservedGeneration, a rolling
		// upgrade would report "available" while the OLD pod is still up
		// — even when the new pod is in ImagePullBackOff and will never
		// become ready.
		if dep.Status.ObservedGeneration < dep.Generation {
			return false, nil
		}
		// Surface permanent failures fast: ProgressDeadlineExceeded
		// means the rollout has stalled past its grace period, and
		// ReplicaFailure=True is set when the controller can't create
		// the new ReplicaSet (image-pull, quota, RBAC). Without these
		// the polling silently waits the entire --timeout (default
		// 5 min) for what is in fact a hard failure.
		for _, cond := range dep.Status.Conditions {
			if appsv1.DeploymentProgressing == cond.Type && corev1.ConditionFalse == cond.Status &&
				"ProgressDeadlineExceeded" == cond.Reason {
				return false, keramoserr.NewErrorf(keramoserr.ErrKube,
					"Deployment %s/%s rollout failed: %s", ns, name, cond.Message)
			}
			if appsv1.DeploymentReplicaFailure == cond.Type && corev1.ConditionTrue == cond.Status {
				return false, keramoserr.NewErrorf(keramoserr.ErrKube,
					"Deployment %s/%s replica failure: %s", ns, name, cond.Message)
			}
		}
		desired := int32(1)
		if nil != dep.Spec.Replicas {
			desired = *dep.Spec.Replicas
		}
		if dep.Status.UpdatedReplicas < desired {
			return false, nil
		}
		if dep.Status.AvailableReplicas < desired {
			return false, nil
		}
		// Replicas (total) > UpdatedReplicas means the old ReplicaSet is
		// still being scaled down; the new ReplicaSet alone must own all
		// replicas.
		if dep.Status.Replicas > dep.Status.UpdatedReplicas {
			return false, nil
		}
		for _, cond := range dep.Status.Conditions {
			if cond.Type == appsv1.DeploymentAvailable && cond.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
}

func (c *Client) waitForStatefulSet(ctx context.Context, obj *unstructured.Unstructured) error {
	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	if err := c.requireClientset(obj.GetKind(), name); nil != err {
		return err
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		ss, err := c.clientset.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		if ss.Status.ObservedGeneration < ss.Generation {
			return false, nil
		}
		replicas := int32(1)
		if nil != ss.Spec.Replicas {
			replicas = *ss.Spec.Replicas
		}
		if ss.Status.UpdatedReplicas < replicas {
			return false, nil
		}
		if ss.Status.ReadyReplicas < replicas {
			return false, nil
		}
		if "" != ss.Status.UpdateRevision && ss.Status.CurrentRevision != ss.Status.UpdateRevision {
			return false, nil
		}
		return true, nil
	})
}

func (c *Client) waitForPod(ctx context.Context, obj *unstructured.Unstructured) error {
	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	if err := c.requireClientset(obj.GetKind(), name); nil != err {
		return err
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := c.clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		if corev1.PodSucceeded == pod.Status.Phase {
			return true, nil
		}
		if corev1.PodFailed == pod.Status.Phase {
			return false, keramoserr.NewErrorf(keramoserr.ErrKube,
				"Pod %s/%s failed: %s", ns, name, pod.Status.Message)
		}
		if corev1.PodRunning != pod.Status.Phase {
			return false, nil
		}
		for _, cond := range pod.Status.Conditions {
			if corev1.PodReady == cond.Type && corev1.ConditionTrue == cond.Status {
				return true, nil
			}
		}
		return false, nil
	})
}

func (c *Client) waitForDaemonSet(ctx context.Context, obj *unstructured.Unstructured) error {
	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	if err := c.requireClientset(obj.GetKind(), name); nil != err {
		return err
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		ds, err := c.clientset.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		if ds.Status.ObservedGeneration < ds.Generation {
			return false, nil
		}
		if 0 == ds.Status.DesiredNumberScheduled {
			return true, nil
		}
		if ds.Status.UpdatedNumberScheduled < ds.Status.DesiredNumberScheduled {
			return false, nil
		}
		return ds.Status.NumberReady >= ds.Status.DesiredNumberScheduled, nil
	})
}

func (c *Client) waitForPVC(ctx context.Context, obj *unstructured.Unstructured) error {
	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	if err := c.requireClientset(obj.GetKind(), name); nil != err {
		return err
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		pvc, err := c.clientset.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		if corev1.ClaimLost == pvc.Status.Phase {
			return false, keramoserr.NewErrorf(keramoserr.ErrKube,
				"PersistentVolumeClaim %s/%s is Lost", ns, name)
		}
		if corev1.ClaimBound == pvc.Status.Phase {
			return true, nil
		}
		return c.bindsOnFirstConsumer(ctx, pvc), nil
	})
}

func (c *Client) waitForService(ctx context.Context, obj *unstructured.Unstructured) error {
	ns := c.resolveNamespace(obj)
	name := obj.GetName()

	serviceType, _, _ := unstructured.NestedString(obj.Object, "spec", "type")
	if string(corev1.ServiceTypeLoadBalancer) != serviceType {
		return nil
	}

	if err := c.requireClientset(obj.GetKind(), name); nil != err {
		return err
	}

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		svc, err := c.clientset.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		return 0 < len(svc.Status.LoadBalancer.Ingress), nil
	})
}

func (c *Client) resolveNamespace(obj *unstructured.Unstructured) string {
	if ns := obj.GetNamespace(); "" != ns {
		return ns
	}
	// Derive scope authoritatively from the cluster's REST mapping. The old
	// hardcoded list covered only 5 kinds, so any other cluster-scoped kind
	// (StorageClass, *WebhookConfiguration, PriorityClass, IngressClass,
	// APIService, cluster-scoped CRs, …) was wrongly namespaced — the apply
	// 404'd and delete silently NotFound-skipped, orphaning the object.
	if namespaced, known := c.isNamespacedScoped(obj); known {
		if namespaced {
			return c.namespace
		}
		return ""
	}
	// Mapping unavailable (e.g. a kind whose CRD is not yet registered): fall
	// back to the static list, defaulting unknown kinds to namespaced.
	if staticClusterScoped[obj.GetKind()] {
		return ""
	}
	return c.namespace
}

// staticClusterScoped is a fallback used only when the REST mapper cannot
// resolve a kind's scope (e.g. a not-yet-registered CRD).
var staticClusterScoped = map[string]bool{
	"Namespace":                true,
	"ClusterRole":              true,
	"ClusterRoleBinding":       true,
	"PersistentVolume":         true,
	"CustomResourceDefinition": true,
}

// isNamespacedScoped reports whether obj's kind is namespace-scoped per the
// cluster's REST mapping. known=false when the mapping cannot be resolved.
func (c *Client) isNamespacedScoped(obj *unstructured.Unstructured) (namespaced, known bool) {
	// No cluster connection (dry-run / unit test): defer to the static list.
	if nil == c.discovery {
		return false, false
	}
	mapper, err := c.ensureMapper()
	if nil != err {
		return false, false
	}
	gvk := obj.GroupVersionKind()
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if nil != err {
		fresh, refreshErr := c.refreshMapper()
		if nil != refreshErr {
			return false, false
		}
		mapping, err = fresh.RESTMapping(gvk.GroupKind(), gvk.Version)
		if nil != err {
			return false, false
		}
	}
	return meta.RESTScopeNameNamespace == mapping.Scope.Name(), true
}

// ensureMapper initializes or returns the cached REST mapper.
func (c *Client) ensureMapper() (meta.RESTMapper, error) {
	c.mapperMu.RLock()
	cached := c.cachedMapper
	c.mapperMu.RUnlock()
	if nil != cached {
		return cached, nil
	}
	return c.refreshMapper()
}

// refreshMapper forces a fresh discovery call and caches the result.
func (c *Client) refreshMapper() (meta.RESTMapper, error) {
	apiGroupResources, err := restmapper.GetAPIGroupResources(c.discovery)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to get API group resources", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(apiGroupResources)
	c.mapperMu.Lock()
	c.cachedMapper = mapper
	c.mapperMu.Unlock()
	return mapper, nil
}

func (c *Client) resourceForObj(obj *unstructured.Unstructured) (schema.GroupVersionResource, error) {
	gvk := obj.GroupVersionKind()

	mapper, err := c.ensureMapper()
	if nil != err {
		return schema.GroupVersionResource{}, err
	}

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if nil != err {
		// Lazy refresh: the cached mapper may be stale for newly registered CRDs.
		mapper, refreshErr := c.refreshMapper()
		if nil != refreshErr {
			return schema.GroupVersionResource{}, refreshErr
		}
		mapping, err = mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if nil != err {
			return schema.GroupVersionResource{}, keramoserr.WrapErrorf(keramoserr.ErrKube, err, "failed to find resource mapping for %s", gvk.String())
		}
	}

	return mapping.Resource, nil
}

// CreateNamespace creates a namespace if it does not exist. Created
// namespaces are stamped with the keramos-managed label so purge can find
// them later by selector. If the namespace already exists we do NOT
// patch its labels — keramos only claims namespaces it actually created,
// to avoid hijacking ownership of namespaces operators provisioned.
func (c *Client) CreateNamespace(name string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				keramoslabels.ManagedByLabel: keramoslabels.ManagedByValue,
			},
		},
	}

	ctx, cancel := c.newContext()
	defer cancel()

	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if nil != err {
		if k8serrors.IsAlreadyExists(err) {
			return nil
		}
		return keramoserr.WrapErrorf(keramoserr.ErrKube, err, "failed to create namespace %s", name)
	}
	return nil
}

// stampKeramosManaged sets the canonical keramos-managed label on the
// resource's metadata if not already present. The label key is taken
// from internal/labels so the value is centrally defined.
//
// In addition to the top-level metadata, this function also stamps the
// embedded pod template (`spec.template.metadata.labels`) for the
// standard pod-spawning workloads — Deployment, StatefulSet, DaemonSet,
// ReplicaSet, ReplicationController, Job — and the doubly-nested pod
// template under CronJob (`spec.jobTemplate.spec.template.metadata.labels`).
// Without that, Pods spawned by these controllers would not carry the
// label even though the controller object does, and a cluster-wide
// `kubectl get pods -l managedBy=keramos` would miss them.
//
// CRDs that embed pod templates (Argo Rollouts, Argo Workflows, KEDA
// ScaledJobs, etc.) are not specially handled — the top-level
// resource carries the label, which is sufficient to identify the
// CRD instance as keramos-managed. Their controllers are responsible for
// propagating labels to spawned pods if they want.
func stampKeramosManaged(obj *unstructured.Unstructured, releaseName, releaseNamespace string) {
	if nil == obj {
		return
	}
	lbls := obj.GetLabels()
	if nil == lbls {
		lbls = map[string]string{}
	}
	if keramoslabels.ManagedByValue != lbls[keramoslabels.ManagedByLabel] {
		lbls[keramoslabels.ManagedByLabel] = keramoslabels.ManagedByValue
		obj.SetLabels(lbls)
	}

	if "" != releaseName {
		annotations := obj.GetAnnotations()
		if nil == annotations {
			annotations = map[string]string{}
		}
		annotations[keramoslabels.ReleaseNameAnnotation] = releaseName
		annotations[keramoslabels.ReleaseNamespaceAnnotation] = releaseNamespace
		obj.SetAnnotations(annotations)
	}

	switch obj.GetKind() {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "ReplicationController", "Job":
		stampPodTemplate(obj.Object, "spec", "template")
	case "CronJob":
		stampPodTemplate(obj.Object, "spec", "jobTemplate", "spec", "template")
	}
}

// stampPodTemplate adds the keramos-managed label to the pod template
// metadata at the given path. It is a no-op if the path does not exist
// or is malformed (e.g. a CRD that happens to share Kind="Job" but
// has a different schema). The function never errors — at worst the
// label is not stamped and we degrade to "controller-only" coverage.
func stampPodTemplate(obj map[string]any, path ...string) {
	cur := obj
	for _, p := range path {
		next, ok := cur[p].(map[string]any)
		if !ok {
			return
		}
		cur = next
	}
	meta, ok := cur["metadata"].(map[string]any)
	if !ok {
		meta = map[string]any{}
		cur["metadata"] = meta
	}
	lbls, ok := meta["labels"].(map[string]any)
	if !ok {
		lbls = map[string]any{}
		meta["labels"] = lbls
	}
	if keramoslabels.ManagedByValue == lbls[keramoslabels.ManagedByLabel] {
		return
	}
	lbls[keramoslabels.ManagedByLabel] = keramoslabels.ManagedByValue
}

// WaitForJob waits for a Job to complete within the given timeout.
func (c *Client) WaitForJob(namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return wait.PollUntilContextCancel(ctx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		job, err := c.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if nil != err {
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if "Complete" == string(cond.Type) && corev1.ConditionTrue == cond.Status {
				return true, nil
			}
			if "Failed" == string(cond.Type) && corev1.ConditionTrue == cond.Status {
				return false, fmt.Errorf("job %s/%s failed: %s", namespace, name, cond.Message)
			}
		}
		return false, nil
	})
}

func boolPtr(b bool) *bool {
	return &b
}

func (c *Client) bindsOnFirstConsumer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) bool {
	className := ""
	if nil != pvc.Spec.StorageClassName {
		className = *pvc.Spec.StorageClassName
	}

	if "" == className {
		classes, listErr := c.clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
		if nil != listErr {
			return false
		}
		for i := range classes.Items {
			if "true" == classes.Items[i].Annotations["storageclass.kubernetes.io/is-default-class"] {
				className = classes.Items[i].Name
				break
			}
		}
	}

	if "" == className {
		return false
	}

	class, getErr := c.clientset.StorageV1().StorageClasses().Get(ctx, className, metav1.GetOptions{})
	if nil != getErr || nil == class.VolumeBindingMode {
		return false
	}

	return storagev1.VolumeBindingWaitForFirstConsumer == *class.VolumeBindingMode
}

func (c *Client) requireClientset(kind, name string) error {
	if nil == c.clientset {
		return keramoserr.NewErrorf(keramoserr.ErrKube,
			"cannot wait for %s/%s: no Kubernetes client is configured", kind, name)
	}
	return nil
}

func (c *Client) SetRelease(name, namespace string) {
	c.releaseName = name
	c.releaseNamespace = namespace
}

func (c *Client) SetTakeOwnership(take bool) {
	c.takeOwnership = take
}

func (c *Client) checkOwnership(obj *unstructured.Unstructured) error {
	if c.takeOwnership || "" == c.releaseName {
		return nil
	}

	current, err := c.fetchCurrent(obj)
	if nil != err || nil == current {
		return nil
	}

	annotations := current.GetAnnotations()
	if nil == annotations {
		return nil
	}

	owner := annotations[keramoslabels.ReleaseNameAnnotation]
	ownerNS := annotations[keramoslabels.ReleaseNamespaceAnnotation]
	if "" != owner && (owner != c.releaseName || ownerNS != c.releaseNamespace) {
		return keramoserr.NewErrorf(keramoserr.ErrKube,
			"%s/%s is owned by release %s in namespace %s; pass --take-ownership to claim it",
			obj.GetKind(), obj.GetName(), owner, ownerNS)
	}

	helmOwner := annotations["meta.helm.sh/release-name"]
	if "" != helmOwner {
		return keramoserr.NewErrorf(keramoserr.ErrKube,
			"%s/%s is owned by the Helm release %s; pass --take-ownership to claim it",
			obj.GetKind(), obj.GetName(), helmOwner)
	}

	return nil
}
