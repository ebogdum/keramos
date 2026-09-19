package action

import (
	"fmt"
	"strings"
	"time"

	"github.com/ebogdum/keramos/v3/internal/audit"
	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"gopkg.in/yaml.v3"
)

// AdoptOptions configures `keramos adopt`. The caller supplies a list of
// resource references (apiVersion/Kind/namespace/name); keramos fetches each
// from the cluster, renders a synthetic manifest, and stores it as a new
// release record so subsequent upgrades / drift / rollback work normally.
type AdoptOptions struct {
	ReleaseName string
	Namespace   string
	Description string
	Resources   []ResourceRef // each "apiVersion/Kind/namespace/name"
	Labels      map[string]string
}

// ResourceRef points at a single in-cluster resource.
type ResourceRef struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// ParseResourceRef accepts a CLI-friendly form: "apps/v1/Deployment/ns/name"
// or "v1/Service//name" (empty namespace for cluster-scoped) or
// "kind=Deployment,name=foo,ns=bar".
func ParseResourceRef(s string) (ResourceRef, error) {
	s = strings.TrimSpace(s)
	if "" == s {
		return ResourceRef{}, keramoserr.NewError(keramoserr.ErrCLIValidation, "empty resource reference")
	}
	if strings.Contains(s, "=") {
		// key=value form
		var ref ResourceRef
		for _, kv := range strings.Split(s, ",") {
			k, v, found := strings.Cut(strings.TrimSpace(kv), "=")
			if !found {
				return ResourceRef{}, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "bad reference fragment %q", kv)
			}
			switch k {
			case "apiVersion", "api":
				ref.APIVersion = v
			case "kind":
				ref.Kind = v
			case "ns", "namespace":
				ref.Namespace = v
			case "name":
				ref.Name = v
			}
		}
		if "" == ref.Kind || "" == ref.Name {
			return ResourceRef{}, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"reference %q missing kind or name", s)
		}
		if "" == ref.APIVersion {
			ref.APIVersion = inferAPIVersion(ref.Kind)
		}
		return ref, nil
	}
	parts := strings.Split(s, "/")
	if 4 == len(parts) {
		return ResourceRef{APIVersion: parts[0], Kind: parts[1], Namespace: parts[2], Name: parts[3]}, nil
	}
	if 5 == len(parts) {
		// apps/v1/Deployment/ns/name
		return ResourceRef{
			APIVersion: parts[0] + "/" + parts[1],
			Kind:       parts[2],
			Namespace:  parts[3],
			Name:       parts[4],
		}, nil
	}
	return ResourceRef{}, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
		"unrecognised reference syntax %q (try apps/v1/Deployment/ns/name)", s)
}

// inferAPIVersion picks a canonical apiVersion for the most common kinds.
// When in doubt, falls back to "v1" which is correct for Pod/Service/CM/Secret/Namespace.
func inferAPIVersion(kind string) string {
	switch kind {
	case "Deployment", "ReplicaSet", "DaemonSet", "StatefulSet":
		return "apps/v1"
	case "Job":
		return "batch/v1"
	case "CronJob":
		return "batch/v1"
	case "Ingress", "NetworkPolicy":
		return "networking.k8s.io/v1"
	case "Role", "RoleBinding", "ClusterRole", "ClusterRoleBinding":
		return "rbac.authorization.k8s.io/v1"
	case "ServiceMonitor", "PodMonitor":
		return "monitoring.coreos.com/v1"
	}
	return "v1"
}

// Adopt fetches every referenced resource, builds a synthetic manifest, and
// stores it under `opts.ReleaseName` as revision 1. After adoption, the
// resources can be `keramos diff`'d, `keramos drift`'d, `keramos upgrade`'d, and
// `keramos uninstall`'d normally.
func Adopt(client kube.KubeClient, opts *AdoptOptions) (*release.Release, error) {
	if "" == opts.ReleaseName {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "release name is required")
	}
	if 0 == len(opts.Resources) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIValidation, "no resources to adopt")
	}
	ns := opts.Namespace
	if "" == ns {
		ns = client.Namespace()
	}
	if "" == ns {
		ns = "default"
	}

	storage, storageErr := release.SelectStorage(client.Clientset(), ns)
	if nil != storageErr {
		return nil, storageErr
	}
	if _, err := storage.Last(opts.ReleaseName); nil == err {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"release %s already exists; use 'keramos upgrade' or pick a new name", opts.ReleaseName)
	} else if he, ok := err.(*keramoserr.KeramosError); !ok || keramoserr.ErrReleaseNotFound != he.Type {
		// A non-"not found" error (transient list/read failure) is ambiguous —
		// abort rather than adopt over a release that may in fact exist.
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err,
			"cannot determine whether release %s already exists", opts.ReleaseName)
	}

	docs := make([]string, 0, len(opts.Resources))
	for _, ref := range opts.Resources {
		obj, err := client.Lookup(ref.APIVersion, ref.Kind, ref.Namespace, ref.Name)
		if nil != err {
			return nil, keramoserr.WrapErrorf(keramoserr.ErrKube, err, "fetch %s/%s/%s", ref.Kind, ref.Namespace, ref.Name)
		}
		if nil == obj {
			return nil, keramoserr.NewErrorf(keramoserr.ErrKube,
				"resource not found: %s/%s/%s/%s", ref.APIVersion, ref.Kind, ref.Namespace, ref.Name)
		}
		// Strip server-side fields that should not be part of a stored manifest.
		stripServerFields(obj)
		body, err := yaml.Marshal(obj)
		if nil != err {
			return nil, keramoserr.WrapError(keramoserr.ErrInternal, "marshal adopted resource", err)
		}
		docs = append(docs, string(body))
	}
	manifest := strings.Join(docs, "---\n")

	now := time.Now().UTC()
	rel := &release.Release{
		Name:      opts.ReleaseName,
		Namespace: ns,
		Revision:  1,
		Status:    release.StatusDeployed,
		Package: release.PackageRef{
			Name:    "(adopted)",
			Version: "0.0.0",
		},
		Manifest: manifest,
		Labels:   opts.Labels,
		Audit:    audit.Capture("adopt", 0),
		Info: release.ReleaseInfo{
			FirstDeployed: now,
			LastDeployed:  now,
			Description:   adoptionDescription(opts),
		},
	}
	if storeErr := storage.Create(rel); nil != storeErr {
		return nil, storeErr
	}
	return rel, nil
}

func adoptionDescription(opts *AdoptOptions) string {
	if "" != opts.Description {
		return opts.Description
	}
	return fmt.Sprintf("adopted %d resource(s)", len(opts.Resources))
}

// stripServerFields removes fields that the API server populates and that
// cannot be re-applied verbatim (status, managedFields, generation, uid,
// resourceVersion, creationTimestamp, ownerReferences).
func stripServerFields(obj map[string]any) {
	delete(obj, "status")
	if meta, ok := obj["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
		delete(meta, "generation")
		delete(meta, "uid")
		delete(meta, "resourceVersion")
		delete(meta, "creationTimestamp")
		delete(meta, "ownerReferences")
		delete(meta, "selfLink")
	}
}
