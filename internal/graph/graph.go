// Package graph emits dependency-graph descriptions of a release's manifests
// and hooks. Output formats: Mermaid (default, copy-paste into Markdown),
// DOT (Graphviz), and a flat ascii list.
package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ebogdum/keramos/internal/hooks"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/release"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Format selects the graph output dialect.
type Format string

const (
	FormatMermaid Format = "mermaid"
	FormatDOT     Format = "dot"
	FormatASCII   Format = "ascii"
)

// Render produces a graph description for a release. The graph contains:
//
//   - one node per K8s resource in the manifest, labelled "kind/name"
//   - one node per hook, ordered by lifecycle phase and weight
//   - edges from ConfigMap/Secret to Pods that mount them (via name match)
func Render(rel *release.Release, format Format) (string, error) {
	if nil == rel {
		return "", fmt.Errorf("nil release")
	}
	resources, err := kube.ParseManifests(rel.Manifest)
	if nil != err {
		return "", err
	}
	g := newGraph()
	for _, r := range resources {
		g.addResource(r.GetKind(), r.GetName(), r.GetNamespace())
	}
	addEdges(g, resources)

	if 0 < len(rel.HookTemplates) {
		parsed, parseErr := hooks.ParseHooks(rel.HookTemplates)
		if nil == parseErr {
			addHookNodes(g, parsed)
		}
	}

	switch format {
	case FormatDOT:
		return g.dot(rel.Name), nil
	case FormatASCII:
		return g.ascii(), nil
	default:
		return g.mermaid(rel.Name), nil
	}
}

type node struct {
	id    string
	label string
	kind  string // resource | hook
	group string
}

type edge struct {
	from string
	to   string
	via  string
}

type graphData struct {
	nodes []node
	edges []edge
	seen  map[string]bool
}

func newGraph() *graphData {
	return &graphData{seen: make(map[string]bool)}
}

func nodeID(kind, name string) string {
	return strings.ToLower(kind) + "_" + sanitize(name)
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

func (g *graphData) addResource(kind, name, namespace string) {
	id := nodeID(kind, name)
	if g.seen[id] {
		return
	}
	g.seen[id] = true
	label := kind + "/" + name
	if "" != namespace {
		label += " (" + namespace + ")"
	}
	g.nodes = append(g.nodes, node{id: id, label: label, kind: "resource"})
}

// addEdges encodes coarse intra-release dependencies for visualization. It
// looks at PodSpec volumes referencing ConfigMaps/Secrets and adds dataflow
// edges. Best-effort; not authoritative.
func addEdges(g *graphData, resources []*unstructured.Unstructured) {
	for _, r := range resources {
		switch r.GetKind() {
		case "Pod", "Deployment", "StatefulSet", "DaemonSet", "Job", "CronJob":
			addPodEdges(g, r)
		}
	}
}

func addPodEdges(g *graphData, r *unstructured.Unstructured) {
	podSpec := podSpecOf(r)
	if nil == podSpec {
		return
	}
	wkID := nodeID(r.GetKind(), r.GetName())
	if vols, ok := podSpec["volumes"].([]any); ok {
		for _, v := range vols {
			vm, _ := v.(map[string]any)
			if cm, ok := vm["configMap"].(map[string]any); ok {
				if name, ok := cm["name"].(string); ok && "" != name {
					g.addResource("ConfigMap", name, r.GetNamespace())
					g.edges = append(g.edges, edge{from: nodeID("ConfigMap", name), to: wkID, via: "volume"})
				}
			}
			if sec, ok := vm["secret"].(map[string]any); ok {
				secretName, _ := sec["secretName"].(string)
				if "" != secretName {
					g.addResource("Secret", secretName, r.GetNamespace())
					g.edges = append(g.edges, edge{from: nodeID("Secret", secretName), to: wkID, via: "volume"})
				}
			}
		}
	}
}

// podSpecOf returns the embedded pod spec for any common workload kind.
func podSpecOf(r *unstructured.Unstructured) map[string]any {
	spec, _ := r.Object["spec"].(map[string]any)
	if nil == spec {
		return nil
	}
	switch r.GetKind() {
	case "Pod":
		return spec
	case "Deployment", "StatefulSet", "DaemonSet", "Job", "ReplicaSet":
		if t, ok := spec["template"].(map[string]any); ok {
			if ps, ok := t["spec"].(map[string]any); ok {
				return ps
			}
		}
	case "CronJob":
		if jt, ok := spec["jobTemplate"].(map[string]any); ok {
			if jspec, ok := jt["spec"].(map[string]any); ok {
				if t, ok := jspec["template"].(map[string]any); ok {
					if ps, ok := t["spec"].(map[string]any); ok {
						return ps
					}
				}
			}
		}
	}
	return nil
}

func addHookNodes(g *graphData, parsed []hooks.Hook) {
	sort.SliceStable(parsed, func(i, j int) bool {
		if parsed[i].Type != parsed[j].Type {
			return string(parsed[i].Type) < string(parsed[j].Type)
		}
		return parsed[i].Weight < parsed[j].Weight
	})
	prev := ""
	for _, h := range parsed {
		id := fmt.Sprintf("hook_%s_w%d", sanitize(string(h.Type)), h.Weight)
		label := fmt.Sprintf("%s w=%d", h.Type, h.Weight)
		g.nodes = append(g.nodes, node{id: id, label: label, kind: "hook", group: string(h.Type)})
		if "" != prev {
			g.edges = append(g.edges, edge{from: prev, to: id, via: "order"})
		}
		prev = id
	}
}

func (g *graphData) mermaid(title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%%%% keramos graph: %s\n", title)
	b.WriteString("flowchart TD\n")
	for _, n := range g.nodes {
		shape := "[" + n.label + "]"
		if "hook" == n.kind {
			shape = "((" + n.label + "))"
		}
		fmt.Fprintf(&b, "  %s%s\n", n.id, shape)
	}
	for _, e := range g.edges {
		fmt.Fprintf(&b, "  %s --> %s\n", e.from, e.to)
	}
	return b.String()
}

func (g *graphData) dot(title string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %q {\n", title)
	b.WriteString("  rankdir=TD;\n")
	for _, n := range g.nodes {
		shape := "box"
		if "hook" == n.kind {
			shape = "ellipse"
		}
		fmt.Fprintf(&b, "  %s [label=%q shape=%s];\n", n.id, n.label, shape)
	}
	for _, e := range g.edges {
		fmt.Fprintf(&b, "  %s -> %s;\n", e.from, e.to)
	}
	b.WriteString("}\n")
	return b.String()
}

func (g *graphData) ascii() string {
	var b strings.Builder
	for _, n := range g.nodes {
		marker := "·"
		if "hook" == n.kind {
			marker = "○"
		}
		fmt.Fprintf(&b, "%s %s\n", marker, n.label)
	}
	if 0 < len(g.edges) {
		b.WriteString("\nedges:\n")
		for _, e := range g.edges {
			fmt.Fprintf(&b, "  %s → %s\n", e.from, e.to)
		}
	}
	return b.String()
}
