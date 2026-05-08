package engine

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestFnDerivePassword_Deterministic(t *testing.T) {
	a, err := fnDerivePassword(1, "long", "example.com", "alice", "supersecret")
	if nil != err {
		t.Fatalf("derive: %v", err)
	}
	b, _ := fnDerivePassword(1, "long", "example.com", "alice", "supersecret")
	if a != b {
		t.Errorf("not deterministic: %v vs %v", a, b)
	}
	c, _ := fnDerivePassword(2, "long", "example.com", "alice", "supersecret")
	if a == c {
		t.Errorf("counter change should produce different password")
	}
}

func TestFnDerivePassword_PINLength(t *testing.T) {
	got, err := fnDerivePassword(1, "pin", "site", "user", "master")
	if nil != err {
		t.Fatalf("derive pin: %v", err)
	}
	if 4 != len(got.(string)) {
		t.Errorf("pin length = %d, want 4", len(got.(string)))
	}
}

func TestFnDerivePassword_UnknownType(t *testing.T) {
	if _, err := fnDerivePassword(1, "bogus", "s", "u", "m"); nil == err {
		t.Fatal("expected unknown-type error")
	}
}

func TestFnHTMLDate(t *testing.T) {
	stamp := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	got, err := fnHTMLDate(stamp)
	if nil != err {
		t.Fatalf("htmlDate: %v", err)
	}
	if "2025-06-01" != got {
		t.Errorf("htmlDate = %v", got)
	}
}

func TestFnHTMLDateInZone(t *testing.T) {
	stamp := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	got, err := fnHTMLDateInZone(stamp, "America/New_York")
	if nil != err {
		t.Fatalf("htmlDateInZone: %v", err)
	}
	// 00:00 UTC on June 1 is May 31 in NYC.
	if "2025-05-31" != got {
		t.Errorf("htmlDateInZone = %v", got)
	}
}

func TestFnDateModify(t *testing.T) {
	stamp := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	got, err := fnDateModify(stamp, "1h")
	if nil != err {
		t.Fatalf("dateModify: %v", err)
	}
	tm := got.(time.Time)
	if 13 != tm.Hour() {
		t.Errorf("dateModify hour = %d, want 13", tm.Hour())
	}
	neg, _ := fnDateModify(stamp, "-30m")
	if 11 != neg.(time.Time).Hour() {
		t.Errorf("dateModify negative hour = %d, want 11", neg.(time.Time).Hour())
	}
}

func TestFloatMath(t *testing.T) {
	cases := []struct {
		fn   string
		val  any
		args []any
		want float64
	}{
		{"addf", 1, []any{2, 3}, 6.0},
		{"subf", 10, []any{3, 2}, 5.0},
		{"mulf", 2, []any{3.5}, 7.0},
		{"divf", 1, []any{3}, 1.0 / 3.0},
	}
	r := NewFuncRegistry()
	for _, c := range cases {
		fn, _ := r.Get(c.fn)
		got, err := fn(c.val, c.args...)
		if nil != err {
			t.Errorf("%s: %v", c.fn, err)
			continue
		}
		f, ok := got.(float64)
		if !ok {
			t.Errorf("%s: expected float64, got %T", c.fn, got)
			continue
		}
		if 1e-9 < absDiff(f, c.want) {
			t.Errorf("%s = %v, want %v", c.fn, f, c.want)
		}
	}
}

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestFnDeepCopy(t *testing.T) {
	original := map[string]any{
		"a": 1,
		"b": map[string]any{"nested": []any{1, 2, 3}},
	}
	copied, err := fnDeepCopy(original)
	if nil != err {
		t.Fatalf("deepCopy: %v", err)
	}
	cp := copied.(map[string]any)
	// Mutate the copy; original must remain intact.
	cp["b"].(map[string]any)["nested"] = []any{99}
	if !reflect.DeepEqual(original["b"].(map[string]any)["nested"], []any{1, 2, 3}) {
		t.Errorf("deepCopy did not isolate nested slice: %v", original)
	}
}

func TestFnDeepEqual(t *testing.T) {
	a := map[string]any{"x": []any{1, 2}}
	b := map[string]any{"x": []any{1, 2}}
	c := map[string]any{"x": []any{1, 3}}
	yes, _ := fnDeepEqual(a, b)
	if true != yes {
		t.Errorf("deepEqual(equal) = %v, want true", yes)
	}
	no, _ := fnDeepEqual(a, c)
	if false != no {
		t.Errorf("deepEqual(unequal) = %v, want false", no)
	}
}

func TestFnChunk(t *testing.T) {
	got, err := fnChunk([]any{1, 2, 3, 4, 5}, 2)
	if nil != err {
		t.Fatalf("chunk: %v", err)
	}
	want := []any{[]any{1, 2}, []any{3, 4}, []any{5}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("chunk = %v, want %v", got, want)
	}
}

func TestFnChunkBadSize(t *testing.T) {
	if _, err := fnChunk([]any{1, 2}, 0); nil == err {
		t.Fatal("expected positive-size error")
	}
}

func TestFnGenSignedCert_Roundtrip(t *testing.T) {
	caRaw, err := fnGenCA("test-ca", 30)
	if nil != err {
		t.Fatalf("genCA: %v", err)
	}
	ca := caRaw.(map[string]any)
	signed, err := fnGenSignedCert("svc.example.com", "192.168.1.10", "alt.example.com", 30, ca["Cert"], ca["Key"])
	if nil != err {
		t.Fatalf("genSignedCert: %v", err)
	}
	out := signed.(map[string]any)
	if !strings.Contains(out["Cert"].(string), "BEGIN CERTIFICATE") {
		t.Error("signed cert missing PEM header")
	}
	// Verify the signed cert really chains to the CA.
	caCert, parseErr := parseCertPEM(ca["Cert"].(string))
	if nil != parseErr {
		t.Fatalf("parseCert: %v", parseErr)
	}
	leaf, parseErr := parseCertPEM(out["Cert"].(string))
	if nil != parseErr {
		t.Fatalf("parseLeaf: %v", parseErr)
	}
	if err := leaf.CheckSignatureFrom(caCert); nil != err {
		t.Errorf("leaf.CheckSignatureFrom(ca) failed: %v", err)
	}
}
