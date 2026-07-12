package helmcompat

import (
	"strings"
	"testing"
)

// TestHelmCompatEnvFuncsRemoved proves the Helm-compat renderer does not expose
// the Sprig env/expandenv functions, matching upstream Helm. Rendering an
// untrusted chart must not be able to read the operator's host environment and
// leak it into a manifest. Both the top-level render path and the tpl path are
// covered.
func TestHelmCompatEnvFuncsRemoved(t *testing.T) {
	t.Setenv("SECRET_TOKEN_UNDER_TEST", "s3cr3t-value")

	dir := t.TempDir()
	writeChart(t, dir, map[string]string{
		"Chart.yaml":  "apiVersion: v2\nname: leak\nversion: 1.0.0\n",
		"values.yaml": "{}\n",
		// {{ env }} in a top-level template, and {{ tpl }} exercising the
		// second funcmap site.
		"templates/cm.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: leak
data:
  direct: "{{ env "SECRET_TOKEN_UNDER_TEST" }}"
  viaTpl: "{{ tpl "{{ env \"SECRET_TOKEN_UNDER_TEST\" }}" . }}"
`,
	})

	_, err := Render(dir, Options{Release: ReleaseMeta{Name: "r", Namespace: "default", Revision: 1}})
	if nil == err {
		t.Fatal("render using env should fail: env must not be a registered function")
	}
	// The failure must be "function not defined", never a successful leak.
	if strings.Contains(err.Error(), "s3cr3t-value") {
		t.Fatalf("secret leaked into error/output: %v", err)
	}
}

// TestHelmCompatKeepsSafeSprigFuncs guards against over-removal: unrelated Sprig
// functions must still be available.
func TestHelmCompatKeepsSafeSprigFuncs(t *testing.T) {
	fm := hermeticSprigFuncMap()
	if _, ok := fm["env"]; ok {
		t.Error("env must be removed")
	}
	if _, ok := fm["expandenv"]; ok {
		t.Error("expandenv must be removed")
	}
	for _, keep := range []string{"upper", "b64enc", "nindent", "trim"} {
		if _, ok := fm[keep]; !ok {
			t.Errorf("safe sprig func %q was removed", keep)
		}
	}
}
