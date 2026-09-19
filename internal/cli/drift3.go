package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ebogdum/keramos/v3/internal/diff"
)

// triField is one field whose value differs across the three views. A "" value
// with its present=false flag means the field is absent from that view.
type triField struct {
	path                   string
	pkg, state, live       string
	inPkg, inState, inLive bool
}

// triResource is one resource's three-way comparison. Only fields that differ
// across the three views are recorded.
type triResource struct {
	key                    string
	kind, name, namespace  string
	inPkg, inState, inLive bool
	fields                 []triField
}

// threeWay compares a rendered package, the stored state, and the live cluster
// manifests, reporting per resource where package/state/running disagree.
// Comparison is limited to keramos-managed leaf paths (those present in the
// package or the state), so cluster-injected noise (status, managedFields,
// defaults) on the live side is ignored — the same philosophy as 2-way drift.
func threeWay(pkgManifest, stateManifest, liveManifest string) ([]triResource, error) {
	pr, err := diff.Parse(pkgManifest)
	if nil != err {
		return nil, err
	}
	sr, err := diff.Parse(stateManifest)
	if nil != err {
		return nil, err
	}
	lr, err := diff.Parse(liveManifest)
	if nil != err {
		return nil, err
	}

	keys := map[string]bool{}
	for k := range pr {
		keys[k] = true
	}
	for k := range sr {
		keys[k] = true
	}
	for k := range lr {
		keys[k] = true
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var out []triResource
	for _, key := range sortedKeys {
		pRes, pOK := pr[key]
		sRes, sOK := sr[key]
		lRes, lOK := lr[key]

		pf := flattenLeaves(bodyOf(pRes, pOK))
		sf := flattenLeaves(bodyOf(sRes, sOK))
		lf := flattenLeaves(bodyOf(lRes, lOK))

		// Only compare keramos-managed paths: those the package or state declare.
		paths := map[string]bool{}
		for p := range pf {
			paths[p] = true
		}
		for p := range sf {
			paths[p] = true
		}
		sortedPaths := make([]string, 0, len(paths))
		for p := range paths {
			sortedPaths = append(sortedPaths, p)
		}
		sort.Strings(sortedPaths)

		var fields []triField
		for _, p := range sortedPaths {
			pv, pok := pf[p]
			sv, sok := sf[p]
			lv, lok := lf[p]
			if pv == sv && sv == lv && pok == sok && sok == lok {
				continue
			}
			fields = append(fields, triField{path: p, pkg: pv, state: sv, live: lv, inPkg: pok, inState: sok, inLive: lok})
		}

		// Skip resources that are identical across all present views.
		presenceMixed := !(pOK == sOK && sOK == lOK)
		if 0 == len(fields) && !presenceMixed {
			continue
		}
		meta := firstPresent(pRes, sRes, lRes, pOK, sOK, lOK)
		out = append(out, triResource{
			key: key, kind: meta.Kind, name: meta.Name, namespace: meta.Namespace,
			inPkg: pOK, inState: sOK, inLive: lOK, fields: fields,
		})
	}
	return out, nil
}

func bodyOf(r diff.Resource, ok bool) map[string]any {
	if !ok {
		return nil
	}
	return r.Body
}

func firstPresent(p, s, l diff.Resource, pOK, sOK, lOK bool) diff.Resource {
	if pOK {
		return p
	}
	if sOK {
		return s
	}
	_ = lOK
	return l
}

// flattenLeaves flattens a resource body into dotted leaf paths (numeric
// segments for array indices), stringifying scalar values for comparison.
func flattenLeaves(m map[string]any) map[string]string {
	out := map[string]string{}
	if nil == m {
		return out
	}
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		switch t := v.(type) {
		case map[string]any:
			for k, sub := range t {
				p := k
				if "" != prefix {
					p = prefix + "." + k
				}
				walk(p, sub)
			}
		case []any:
			for i, sub := range t {
				walk(prefix+"."+strconv.Itoa(i), sub)
			}
		default:
			out[prefix] = fmt.Sprintf("%v", v)
		}
	}
	walk("", m)
	return out
}

// formatThreeWay renders the three-way comparison for humans.
func formatThreeWay(resources []triResource, color bool) string {
	var b strings.Builder
	var drift, pending, orphan, missing, create int
	for _, r := range resources {
		symbol, verb, code := "~", "differs", ansiYellow
		switch {
		case r.inPkg && !r.inState && !r.inLive:
			symbol, verb, code = "+", "in package only", ansiGreen
			create++
		case !r.inPkg && (r.inState || r.inLive):
			symbol, verb, code = "-", "removed from package", ansiRed
			orphan++
		case r.inState && !r.inLive:
			symbol, verb, code = "!", "missing from cluster", ansiRed
			missing++
		}
		ident := r.kind + "/" + r.name
		if "" != r.namespace {
			ident += "  (namespace " + r.namespace + ")"
		}
		line := fmt.Sprintf("%s %-22s %s\n", symbol, verb, ident)
		if color {
			line = code + line + ansiReset
		}
		b.WriteString(line)

		for _, f := range r.fields {
			tag := ""
			switch {
			case f.inState && f.inLive && f.state != f.live:
				tag = "  ⚠ cluster drift"
				drift++
			case f.inPkg && f.inState && f.pkg != f.state:
				tag = "  → pending apply"
				pending++
			}
			b.WriteString("      " + f.path + tag + "\n")
			b.WriteString("          package: " + presentOr(f.pkg, f.inPkg) + "\n")
			b.WriteString("          state:   " + presentOr(f.state, f.inState) + "\n")
			b.WriteString("          running: " + presentOr(f.live, f.inLive) + "\n")
		}
	}
	b.WriteString(fmt.Sprintf("\n%d cluster-drift, %d pending-apply, %d orphan, %d missing, %d to-create.\n",
		drift, pending, orphan, missing, create))
	return b.String()
}

func presentOr(v string, present bool) string {
	if !present {
		return "(absent)"
	}
	if "" == v {
		return `""`
	}
	return v
}
