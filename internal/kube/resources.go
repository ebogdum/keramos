package kube

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"sort"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// installOrder defines the precedence for resource installation.
// Lower index means installed first. CustomResourceDefinition goes FIRST so
// custom-resource instances later in the same render set find their
// definition already in place. The crds/ directory is the strict path for
// CRD installation (with Established=true wait); this ordering exists for
// charts that place CRDs in templates/.
var installOrder = map[string]int{
	"CustomResourceDefinition": 0,
	"Namespace":                1,
	"ServiceAccount":           2,
	"ClusterRole":              3,
	"ClusterRoleBinding":       4,
	"Role":                     5,
	"RoleBinding":              6,
	"ConfigMap":                7,
	"Secret":                   8,
	"PersistentVolumeClaim":    9,
	"Service":                  10,
	"Deployment":               11,
	"StatefulSet":              12,
	"DaemonSet":                13,
	"Job":                      14,
	"CronJob":                  15,
	"Ingress":                  16,
}

const defaultInstallOrder = 100

// ParseManifests parses a multi-document YAML string into unstructured resources.
func ParseManifests(manifests string) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured

	reader := yaml.NewYAMLReader(bufio.NewReader(strings.NewReader(manifests)))
	for {
		data, err := reader.Read()
		if nil != err {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to read YAML document", err)
		}

		trimmed := bytes.TrimSpace(data)
		if 0 == len(trimmed) || "---" == string(trimmed) {
			continue
		}

		// Convert YAML to JSON first to handle all YAML variants
		jsonData, convertErr := yaml.ToJSON(trimmed)
		if nil != convertErr {
			return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to convert YAML to JSON", convertErr)
		}

		// Skip null documents (empty YAML docs like "---\n---")
		jsonTrimmed := bytes.TrimSpace(jsonData)
		if 0 == len(jsonTrimmed) || "null" == string(jsonTrimmed) {
			continue
		}

		obj := &unstructured.Unstructured{}
		if unmarshalErr := obj.UnmarshalJSON(jsonData); nil != unmarshalErr {
			return nil, keramoserr.WrapError(keramoserr.ErrKube, "failed to parse manifest", unmarshalErr)
		}

		// Skip empty or invalid objects
		if "" == obj.GetKind() {
			continue
		}

		resources = append(resources, obj)
	}

	return resources, nil
}

// SortByInstallOrder sorts resources for installation (Namespace first, then RBAC, then workloads, etc.).
func SortByInstallOrder(resources []*unstructured.Unstructured) []*unstructured.Unstructured {
	sorted := make([]*unstructured.Unstructured, len(resources))
	copy(sorted, resources)

	sort.SliceStable(sorted, func(i, j int) bool {
		return kindOrder(sorted[i].GetKind()) < kindOrder(sorted[j].GetKind())
	})

	return sorted
}

// SortByUninstallOrder sorts resources for deletion (reverse of install order).
func SortByUninstallOrder(resources []*unstructured.Unstructured) []*unstructured.Unstructured {
	sorted := make([]*unstructured.Unstructured, len(resources))
	copy(sorted, resources)

	sort.SliceStable(sorted, func(i, j int) bool {
		return kindOrder(sorted[i].GetKind()) > kindOrder(sorted[j].GetKind())
	})

	return sorted
}

func kindOrder(kind string) int {
	order, exists := installOrder[kind]
	if !exists {
		return defaultInstallOrder
	}
	return order
}
