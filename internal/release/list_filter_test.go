package release

import (
	"testing"

	keramoslabels "github.com/ebogdum/keramos/internal/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// A keramos-managed application Secret carries managedBy=keramos but is NOT a release
// record. List/History must ignore it silently rather than mis-decoding it.
func TestListIgnoresManagedNonReleaseSecrets(t *testing.T) {
	cs := fake.NewSimpleClientset(
		// A genuine release record.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretName("myapp", 1),
				Namespace: "default",
				Labels: map[string]string{
					keramoslabels.ManagedByLabel: keramoslabels.ManagedByValue,
					labelName:                 "myapp",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{dataKey: mustEncode(t, &Release{Name: "myapp", Revision: 1, Namespace: "default"})},
		},
		// A keramos-managed application secret — same label, NOT a release record.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pihole-password",
				Namespace: "default",
				Labels:    map[string]string{keramoslabels.ManagedByLabel: keramoslabels.ManagedByValue},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"password": []byte("s3cr3t")},
		},
	)
	st := NewSecretStorage(cs, "default")

	rels, err := st.List("default")
	if nil != err {
		t.Fatalf("List erred: %v", err)
	}
	if 1 != len(rels) {
		t.Fatalf("expected exactly 1 release (the record), got %d: %+v", len(rels), rels)
	}
	if "myapp" != rels[0].Name {
		t.Fatalf("unexpected release: %s", rels[0].Name)
	}
}

func TestIsReleaseRecord(t *testing.T) {
	cases := []struct {
		name string
		data map[string][]byte
		want bool
	}{
		{"keramos.v1.myapp.v1", map[string][]byte{dataKey: []byte("x")}, true},
		{"keramos.v1.my.app.with.dots.v12", map[string][]byte{dataKey: []byte("x")}, true},
		{"keramos.v1.myapp.v1", map[string][]byte{"other": []byte("x")}, false}, // right name, no data key
		{"pihole-password", map[string][]byte{"password": []byte("x")}, false},
		{"keramos.v1.myapp", map[string][]byte{dataKey: []byte("x")}, false},   // missing revision
		{"my-keramos.v1.x.v1", map[string][]byte{dataKey: []byte("x")}, false}, // prefix not anchored
	}
	for _, c := range cases {
		s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: c.name}, Data: c.data}
		if got := isReleaseRecord(s); got != c.want {
			t.Errorf("isReleaseRecord(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func mustEncode(t *testing.T, rel *Release) []byte {
	t.Helper()
	enc, err := Encode(rel)
	if nil != err {
		t.Fatalf("encode: %v", err)
	}
	return []byte(enc)
}
