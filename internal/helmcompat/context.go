package helmcompat

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
	"path"
	"sort"
	"strconv"
	"strings"
)

// versionSet implements Helm's .Capabilities.APIVersions with a Has method.
type versionSet struct {
	set map[string]bool
}

func (v versionSet) Has(s string) bool { return v.set[s] }

func (v versionSet) String() string {
	out := make([]string, 0, len(v.set))
	for k := range v.set {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// kubeVersion mirrors Helm's .Capabilities.KubeVersion (Version/Major/Minor).
type kubeVersion struct {
	Version string
	Major   string
	Minor   string
}

func (k kubeVersion) String() string { return k.Version }

type capabilities struct {
	KubeVersion kubeVersion
	APIVersions versionSet
	HelmVersion map[string]any
}

func newCapabilities(c CapabilitiesMeta) capabilities {
	ver := c.KubeVersion
	if "" == ver {
		ver = defaultKubeVersion
	}
	major, minor := parseMajorMinor(ver)
	set := map[string]bool{}
	for _, a := range c.APIVersions {
		set[a] = true
	}
	// Always-present core groups, as Helm seeds.
	for _, a := range defaultAPIVersions {
		set[a] = true
	}
	return capabilities{
		KubeVersion: kubeVersion{Version: ver, Major: major, Minor: minor},
		APIVersions: versionSet{set: set},
		HelmVersion: map[string]any{"version": "v3", "gitCommit": "", "gitTreeState": "", "goVersion": ""},
	}
}

func parseMajorMinor(ver string) (string, string) {
	v := strings.TrimPrefix(ver, "v")
	parts := strings.SplitN(v, ".", 3)
	major, minor := "", ""
	if len(parts) > 0 {
		major = digitsOnly(parts[0])
	}
	if len(parts) > 1 {
		minor = digitsOnly(parts[1])
	}
	return major, minor
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if _, err := strconv.Atoi(b.String()); nil != err {
		return ""
	}
	return b.String()
}

// files implements Helm's .Files API over a chart's non-template files.
type files struct {
	data map[string][]byte
}

func newFiles(data map[string][]byte) files { return files{data: data} }

func (f files) Get(name string) string { return string(f.data[name]) }

func (f files) GetBytes(name string) []byte { return f.data[name] }

func (f files) Glob(pattern string) files {
	out := map[string][]byte{}
	for name, b := range f.data {
		if ok, _ := path.Match(pattern, name); ok {
			out[name] = b
		}
	}
	return files{data: out}
}

func (f files) Lines(name string) []string {
	b, ok := f.data[name]
	if !ok {
		return []string{}
	}
	return strings.Split(string(b), "\n")
}

func (f files) AsConfig() string {
	out := map[string]string{}
	for name, b := range f.data {
		out[path.Base(name)] = string(b)
	}
	return marshalSortedYAML(out)
}

func (f files) AsSecrets() string {
	out := map[string]string{}
	for name, b := range f.data {
		out[path.Base(name)] = base64.StdEncoding.EncodeToString(b)
	}
	return marshalSortedYAML(out)
}

func marshalSortedYAML(in map[string]string) string {
	if 0 == len(in) {
		return ""
	}

	keys := make([]string, 0, len(in))
	for k := range in {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		encoded, err := yaml.Marshal(map[string]string{k: in[k]})
		if nil != err {
			continue
		}
		b.Write(encoded)
	}

	return strings.TrimRight(b.String(), "\n")
}

const defaultKubeVersion = "v1.31.0"

var defaultAPIVersions = []string{
	"v1",
	"admissionregistration.k8s.io/v1",
	"apiextensions.k8s.io/v1",
	"apps/v1",
	"autoscaling/v1",
	"autoscaling/v2",
	"batch/v1",
	"certificates.k8s.io/v1",
	"coordination.k8s.io/v1",
	"discovery.k8s.io/v1",
	"events.k8s.io/v1",
	"networking.k8s.io/v1",
	"node.k8s.io/v1",
	"policy/v1",
	"rbac.authorization.k8s.io/v1",
	"scheduling.k8s.io/v1",
	"storage.k8s.io/v1",
}
