package release

import "testing"

func TestReleaseProvenanceRoundTrip(t *testing.T) {
	rel := &Release{Name: "r", Provenance: map[string]string{"replicas": "set (replicas=9)"}}
	enc, err := Encode(rel)
	if nil != err {
		t.Fatalf("encode: %v", err)
	}
	got, err := Decode(enc)
	if nil != err {
		t.Fatalf("decode: %v", err)
	}
	if "set (replicas=9)" != got.Provenance["replicas"] {
		t.Fatalf("provenance lost through state round-trip: %v", got.Provenance)
	}
}
