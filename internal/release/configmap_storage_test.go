package release

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigMapStorage_CRUD(t *testing.T) {
	cs := fake.NewSimpleClientset()
	s := NewConfigMapStorage(cs, "default")

	rel := &Release{
		Name:      "demo",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Manifest:  "kind: ConfigMap",
		Info:      ReleaseInfo{FirstDeployed: time.Now(), LastDeployed: time.Now()},
	}
	if err := s.Create(rel); nil != err {
		t.Fatalf("create: %v", err)
	}

	// Verify the underlying ConfigMap object exists.
	cm, getErr := cs.CoreV1().ConfigMaps("default").Get(
		context.Background(), ConfigMapName("demo", 1), metav1.GetOptions{})
	if nil != getErr {
		t.Fatalf("expected ConfigMap to exist: %v", getErr)
	}
	if "" == cm.Data[dataKey] {
		t.Error("ConfigMap missing release data")
	}

	got, err := s.Get("demo", 1)
	if nil != err {
		t.Fatalf("get: %v", err)
	}
	if got.Manifest != "kind: ConfigMap" {
		t.Errorf("manifest mismatch")
	}
}

func TestConfigMapStorage_HistoryAscending(t *testing.T) {
	cs := fake.NewSimpleClientset()
	s := NewConfigMapStorage(cs, "default")
	for _, r := range []int{2, 1, 3} {
		_ = s.Create(&Release{Name: "h", Namespace: "default", Revision: r, Status: StatusDeployed})
	}
	hist, err := s.History("h")
	if nil != err {
		t.Fatalf("history: %v", err)
	}
	if 3 != len(hist) || hist[0].Revision != 1 || hist[2].Revision != 3 {
		t.Errorf("history not sorted: %v", hist)
	}
}

func TestConfigMapStorage_Last(t *testing.T) {
	cs := fake.NewSimpleClientset()
	s := NewConfigMapStorage(cs, "default")
	_ = s.Create(&Release{Name: "x", Namespace: "default", Revision: 1, Status: StatusSuperseded})
	_ = s.Create(&Release{Name: "x", Namespace: "default", Revision: 2, Status: StatusDeployed})
	last, err := s.Last("x")
	if nil != err {
		t.Fatalf("last: %v", err)
	}
	if 2 != last.Revision {
		t.Errorf("last revision = %d", last.Revision)
	}
}

func TestConfigMapStorage_Underlying(t *testing.T) {
	// Sanity: fake clientset and CoreV1 ConfigMaps interaction.
	cs := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	})
	cm, err := cs.CoreV1().ConfigMaps("default").Get(context.Background(), "test", metav1.GetOptions{})
	if nil != err || "test" != cm.Name {
		t.Fatalf("fake clientset broken: %v", err)
	}
}
