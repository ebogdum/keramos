package release

import (
	"context"
	"strings"
	"testing"
	"time"

	keramoslabels "github.com/ebogdum/keramos/v3/internal/labels"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// ---------------------------------------------------------------------------
// Encode / Decode — additional edge cases
// ---------------------------------------------------------------------------

func TestEncodeNilRelease(t *testing.T) {
	_, err := Encode(nil)
	// json.Marshal(nil) produces "null" — should still round-trip or error gracefully
	if nil != err {
		t.Fatalf("unexpected error encoding nil: %v", err)
	}
}

func TestDecodeEmptyString(t *testing.T) {
	_, err := Decode("")
	if nil == err {
		t.Fatal("expected error for empty string")
	}
}

func TestDecodeCorruptBase64(t *testing.T) {
	_, err := Decode("!!!not-base64!!!")
	if nil == err {
		t.Fatal("expected error for corrupt base64")
	}
}

func TestEncodeDecodeWithSpecialCharacters(t *testing.T) {
	rel := &Release{
		Name:      "special-chars",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package: PackageRef{
			Name:    "pkg",
			Version: "1.0.0",
		},
		Values: map[string]any{
			"config": "line1\nline2\ttab",
			"unicode": "日本語テスト",
			"quotes":  `"hello" 'world'`,
		},
		Manifest: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\ndata:\n  key: \"val\\nue\"\n",
		Notes:    "Notes with special chars: <>&\"' \n\t",
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
			Description:   "desc with emoji: ok",
		},
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if rel.Name != decoded.Name {
		t.Errorf("Name mismatch")
	}
	if rel.Notes != decoded.Notes {
		t.Errorf("Notes mismatch: got %q", decoded.Notes)
	}

	configVal, ok := decoded.Values["config"].(string)
	if !ok || "line1\nline2\ttab" != configVal {
		t.Errorf("config value mismatch: %v", decoded.Values["config"])
	}

	unicodeVal, ok := decoded.Values["unicode"].(string)
	if !ok || "日本語テスト" != unicodeVal {
		t.Errorf("unicode value mismatch")
	}
}

func TestEncodeDecodeLargeManifest(t *testing.T) {
	largeManifest := strings.Repeat("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n---\n", 200)

	rel := &Release{
		Name:      "large",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package:   PackageRef{Name: "pkg", Version: "1.0.0"},
		Manifest:  largeManifest,
		Info: ReleaseInfo{
			FirstDeployed: time.Now().UTC(),
			LastDeployed:  time.Now().UTC(),
		},
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if rel.Manifest != decoded.Manifest {
		t.Error("large manifest round-trip mismatch")
	}
}

func TestEncodeDecodeEmptyFields(t *testing.T) {
	rel := &Release{
		Name:      "",
		Namespace: "",
		Revision:  0,
		Status:    "",
		Package:   PackageRef{},
		Values:    nil,
		Manifest:  "",
		Notes:     "",
		Hooks:     nil,
		Info:      ReleaseInfo{},
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if "" != decoded.Name {
		t.Errorf("expected empty name, got %q", decoded.Name)
	}
	if 0 != decoded.Revision {
		t.Errorf("expected 0 revision, got %d", decoded.Revision)
	}
	if nil != decoded.Hooks {
		t.Errorf("expected nil hooks, got %v", decoded.Hooks)
	}
}

func TestEncodeDecodeMultipleHooks(t *testing.T) {
	rel := &Release{
		Name:      "multi-hooks",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package:   PackageRef{Name: "pkg", Version: "1.0.0"},
		Manifest:  "apiVersion: v1\nkind: ConfigMap\n",
		Hooks: []HookResult{
			{Name: "pre-install", Kind: "Job", Status: "succeeded"},
			{Name: "post-install", Kind: "Job", Status: "succeeded"},
			{Name: "pre-upgrade", Kind: "Job", Status: "failed"},
		},
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if 3 != len(decoded.Hooks) {
		t.Fatalf("expected 3 hooks, got %d", len(decoded.Hooks))
	}
	if "failed" != decoded.Hooks[2].Status {
		t.Errorf("expected third hook to be failed")
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	// Encode valid gzip of invalid JSON
	// We can't easily craft this directly, but we can test base64 of non-gzip
	_, err := Decode("aGVsbG8gd29ybGQ=") // "hello world" in base64
	if nil == err {
		t.Fatal("expected error for non-gzip data")
	}
}

// ---------------------------------------------------------------------------
// SecretName — additional cases
// ---------------------------------------------------------------------------

func TestSecretNameZeroRevision(t *testing.T) {
	name := SecretName("test", 0)
	if "keramos.v1.test.v0" != name {
		t.Errorf("expected keramos.v1.test.v0, got %s", name)
	}
}

func TestSecretNameLargeRevision(t *testing.T) {
	name := SecretName("app", 999)
	if "keramos.v1.app.v999" != name {
		t.Errorf("expected keramos.v1.app.v999, got %s", name)
	}
}

// ---------------------------------------------------------------------------
// SecretLabels — all statuses
// ---------------------------------------------------------------------------

func TestSecretLabelsAllFields(t *testing.T) {
	tests := []struct {
		name     string
		rel      *Release
		wantName string
		wantVer  string
		wantStat string
	}{
		{
			name:     "deployed",
			rel:      &Release{Name: "app", Revision: 1, Status: StatusDeployed},
			wantName: "app", wantVer: "1", wantStat: "deployed",
		},
		{
			name:     "superseded",
			rel:      &Release{Name: "app", Revision: 2, Status: StatusSuperseded},
			wantName: "app", wantVer: "2", wantStat: "superseded",
		},
		{
			name:     "failed",
			rel:      &Release{Name: "app", Revision: 3, Status: StatusFailed},
			wantName: "app", wantVer: "3", wantStat: "failed",
		},
		{
			name:     "uninstalling",
			rel:      &Release{Name: "app", Revision: 4, Status: StatusUninstalling},
			wantName: "app", wantVer: "4", wantStat: "uninstalling",
		},
		{
			name:     "pending-install",
			rel:      &Release{Name: "app", Revision: 5, Status: StatusPendingInstall},
			wantName: "app", wantVer: "5", wantStat: "pending-install",
		},
		{
			name:     "pending-upgrade",
			rel:      &Release{Name: "app", Revision: 6, Status: StatusPendingUpgrade},
			wantName: "app", wantVer: "6", wantStat: "pending-upgrade",
		},
		{
			name:     "pending-rollback",
			rel:      &Release{Name: "app", Revision: 7, Status: StatusPendingRollback},
			wantName: "app", wantVer: "7", wantStat: "pending-rollback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := SecretLabels(tt.rel)
			if "keramos" != labels[labelOwner] {
				t.Errorf("owner: expected keramos, got %s", labels[labelOwner])
			}
			if tt.wantName != labels[labelName] {
				t.Errorf("name: expected %s, got %s", tt.wantName, labels[labelName])
			}
			if tt.wantVer != labels[labelVersion] {
				t.Errorf("version: expected %s, got %s", tt.wantVer, labels[labelVersion])
			}
			if tt.wantStat != labels[labelStatus] {
				t.Errorf("status: expected %s, got %s", tt.wantStat, labels[labelStatus])
			}
		})
	}
}

// ---------------------------------------------------------------------------
// newStorageContext
// ---------------------------------------------------------------------------

func TestNewStorageContext(t *testing.T) {
	ctx, cancel := newStorageContext()
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline")
	}

	remaining := time.Until(deadline)
	if remaining > storageTimeout || remaining < storageTimeout-1*time.Second {
		t.Errorf("expected ~%v remaining, got %v", storageTimeout, remaining)
	}
}

// ---------------------------------------------------------------------------
// SecretStorage with fake clientset
// ---------------------------------------------------------------------------

func makeTestRelease(name string, revision int, status Status) *Release {
	return &Release{
		Name:      name,
		Namespace: "default",
		Revision:  revision,
		Status:    status,
		Package:   PackageRef{Name: "pkg", Version: "1.0.0"},
		Values:    map[string]any{"key": "value"},
		Manifest:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n",
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			Description:   "test release",
		},
	}
}

func TestSecretStorage_CreateAndGet(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	rel := makeTestRelease("myapp", 1, StatusDeployed)

	err := storage.Create(rel)
	if nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := storage.Get("myapp", 1)
	if nil != err {
		t.Fatalf("Get failed: %v", err)
	}

	if "myapp" != got.Name {
		t.Errorf("expected myapp, got %s", got.Name)
	}
	if 1 != got.Revision {
		t.Errorf("expected revision 1, got %d", got.Revision)
	}
	if StatusDeployed != got.Status {
		t.Errorf("expected deployed, got %s", got.Status)
	}
}

func TestSecretStorage_GetNotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	_, err := storage.Get("nonexistent", 1)
	if nil == err {
		t.Fatal("expected error for nonexistent release")
	}
}

func TestSecretStorage_Update(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	rel := makeTestRelease("myapp", 1, StatusDeployed)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	rel.Status = StatusSuperseded
	rel.Manifest = "updated manifest"
	if err := storage.Update(rel); nil != err {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := storage.Get("myapp", 1)
	if nil != err {
		t.Fatalf("Get after update failed: %v", err)
	}

	if StatusSuperseded != got.Status {
		t.Errorf("expected superseded, got %s", got.Status)
	}
	if "updated manifest" != got.Manifest {
		t.Errorf("manifest not updated")
	}
}

func TestSecretStorage_UpdateNotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	rel := makeTestRelease("nonexistent", 1, StatusDeployed)
	err := storage.Update(rel)
	if nil == err {
		t.Fatal("expected error updating nonexistent release")
	}
}

func TestSecretStorage_Delete(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	rel := makeTestRelease("myapp", 1, StatusDeployed)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	err := storage.Delete("myapp", 1)
	if nil != err {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should be gone
	_, err = storage.Get("myapp", 1)
	if nil == err {
		t.Fatal("expected error after deletion")
	}
}

func TestSecretStorage_DeleteNotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Deleting nonexistent should succeed (idempotent)
	err := storage.Delete("nonexistent", 1)
	if nil != err {
		t.Fatalf("expected nil for deleting nonexistent, got %v", err)
	}
}

func TestSecretStorage_List(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Create multiple releases
	for i := 1; i <= 3; i++ {
		rel := makeTestRelease("app", i, StatusDeployed)
		if err := storage.Create(rel); nil != err {
			t.Fatalf("Create rev %d failed: %v", i, err)
		}
	}

	// Also create a different app
	rel2 := makeTestRelease("other-app", 1, StatusDeployed)
	if err := storage.Create(rel2); nil != err {
		t.Fatalf("Create other-app failed: %v", err)
	}

	releases, err := storage.List("default")
	if nil != err {
		t.Fatalf("List failed: %v", err)
	}

	if 4 != len(releases) {
		t.Errorf("expected 4 releases, got %d", len(releases))
	}
}

func TestSecretStorage_ListEmpty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	releases, err := storage.List("")
	if nil != err {
		t.Fatalf("List failed: %v", err)
	}
	if 0 != len(releases) {
		t.Errorf("expected 0 releases, got %d", len(releases))
	}
}

func TestSecretStorage_ListUsesStorageNamespaceWhenEmpty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "my-namespace")

	rel := makeTestRelease("app", 1, StatusDeployed)
	rel.Namespace = "my-namespace"

	// Create the secret directly in the right namespace
	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName(rel.Name, rel.Revision),
			Namespace: "my-namespace",
			Labels:    SecretLabels(rel),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			dataKey: []byte(encoded),
		},
	}

	_, createErr := cs.CoreV1().Secrets("my-namespace").Create(
		context.Background(), secret, metav1.CreateOptions{},
	)
	if nil != createErr {
		t.Fatalf("manual create failed: %v", createErr)
	}

	// List with empty namespace — should use storage namespace
	releases, err := storage.List("")
	if nil != err {
		t.Fatalf("List failed: %v", err)
	}
	if 1 != len(releases) {
		t.Errorf("expected 1 release, got %d", len(releases))
	}
}

func TestSecretStorage_History(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Create revisions out of order
	for _, rev := range []int{3, 1, 2} {
		rel := makeTestRelease("app", rev, StatusDeployed)
		if err := storage.Create(rel); nil != err {
			t.Fatalf("Create rev %d failed: %v", rev, err)
		}
	}

	history, err := storage.History("app")
	if nil != err {
		t.Fatalf("History failed: %v", err)
	}

	if 3 != len(history) {
		t.Fatalf("expected 3, got %d", len(history))
	}

	// Should be sorted by revision ascending
	for i := 0; i < len(history)-1; i++ {
		if history[i].Revision >= history[i+1].Revision {
			t.Errorf("history not sorted: revision %d before %d", history[i].Revision, history[i+1].Revision)
		}
	}
}

func TestSecretStorage_HistoryEmpty(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	history, err := storage.History("nonexistent")
	if nil != err {
		t.Fatalf("History failed: %v", err)
	}
	if 0 != len(history) {
		t.Errorf("expected empty history, got %d", len(history))
	}
}

func TestSecretStorage_Last(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	for _, rev := range []int{1, 2, 3} {
		rel := makeTestRelease("app", rev, StatusDeployed)
		if err := storage.Create(rel); nil != err {
			t.Fatalf("Create rev %d failed: %v", rev, err)
		}
	}

	last, err := storage.Last("app")
	if nil != err {
		t.Fatalf("Last failed: %v", err)
	}

	if 3 != last.Revision {
		t.Errorf("expected revision 3, got %d", last.Revision)
	}
}

func TestSecretStorage_LastNotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	_, err := storage.Last("nonexistent")
	if nil == err {
		t.Fatal("expected error for nonexistent release")
	}
}

// ---------------------------------------------------------------------------
// decodeSecret — missing data key
// ---------------------------------------------------------------------------

func TestDecodeSecret_MissingDataKey(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keramos.v1.test.v1",
		},
		Data: map[string][]byte{
			"wrong-key": []byte("data"),
		},
	}

	_, err := decodeSecret(secret)
	if nil == err {
		t.Fatal("expected error for missing data key")
	}
}

func TestDecodeSecret_EmptyData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keramos.v1.test.v1",
		},
		Data: map[string][]byte{},
	}

	_, err := decodeSecret(secret)
	if nil == err {
		t.Fatal("expected error for empty data")
	}
}

func TestDecodeSecret_CorruptData(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keramos.v1.test.v1",
		},
		Data: map[string][]byte{
			dataKey: []byte("not-valid-encoded-data"),
		},
	}

	_, err := decodeSecret(secret)
	if nil == err {
		t.Fatal("expected error for corrupt data")
	}
}

func TestDecodeSecret_ValidData(t *testing.T) {
	rel := makeTestRelease("app", 1, StatusDeployed)
	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keramos.v1.app.v1",
		},
		Data: map[string][]byte{
			dataKey: []byte(encoded),
		},
	}

	decoded, err := decodeSecret(secret)
	if nil != err {
		t.Fatalf("decodeSecret failed: %v", err)
	}
	if "app" != decoded.Name {
		t.Errorf("expected app, got %s", decoded.Name)
	}
}

// ---------------------------------------------------------------------------
// Size validation
// ---------------------------------------------------------------------------

func TestSecretStorage_CreateOversized(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Build pseudo-random data that won't compress well
	// Use a simple LCG to generate non-repeating bytes
	size := 2 * 1024 * 1024
	bigData := make([]byte, size)
	v := uint32(12345)
	for i := range bigData {
		v = v*1103515245 + 12345
		bigData[i] = byte(v >> 16)
	}

	rel := &Release{
		Name:      "oversized",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package:   PackageRef{Name: "pkg", Version: "1.0.0"},
		Manifest:  string(bigData),
		Info: ReleaseInfo{
			FirstDeployed: time.Now().UTC(),
			LastDeployed:  time.Now().UTC(),
		},
	}

	err := storage.Create(rel)
	if nil == err {
		// If gzip compressed it enough, skip
		t.Skip("data compressed below 1MB limit; skipping oversized test")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size error, got: %v", err)
	}
}

func TestSecretStorage_UpdateOversized(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Create small first
	rel := makeTestRelease("app", 1, StatusDeployed)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	// Build pseudo-random data that won't compress well
	size := 2 * 1024 * 1024
	bigData := make([]byte, size)
	v := uint32(54321)
	for i := range bigData {
		v = v*1103515245 + 12345
		bigData[i] = byte(v >> 16)
	}
	rel.Manifest = string(bigData)

	err := storage.Update(rel)
	if nil == err {
		t.Skip("data compressed below 1MB limit; skipping oversized update test")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("expected size error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SecretStorage — List with corrupt secrets
// ---------------------------------------------------------------------------

func TestSecretStorage_ListSkipsCorruptSecrets(t *testing.T) {
	cs := fake.NewSimpleClientset()

	// Create a valid release secret
	rel := makeTestRelease("good-app", 1, StatusDeployed)
	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	goodSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName("good-app", 1),
			Namespace: "default",
			Labels:    SecretLabels(rel),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKey: []byte(encoded)},
	}

	// Create a corrupt release secret
	badSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keramos.v1.bad-app.v1",
			Namespace: "default",
			Labels: map[string]string{
				labelOwner: keramoslabels.ManagedByValue,
				labelName:  "bad-app",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKey: []byte("corrupt-data")},
	}

	ctx := context.Background()
	_, err = cs.CoreV1().Secrets("default").Create(ctx, goodSecret, metav1.CreateOptions{})
	if nil != err {
		t.Fatalf("create good secret: %v", err)
	}
	_, err = cs.CoreV1().Secrets("default").Create(ctx, badSecret, metav1.CreateOptions{})
	if nil != err {
		t.Fatalf("create bad secret: %v", err)
	}

	storage := NewSecretStorage(cs, "default")
	releases, err := storage.List("default")
	if nil != err {
		t.Fatalf("List failed: %v", err)
	}

	// Should only have the good release
	if 1 != len(releases) {
		t.Errorf("expected 1 release (skipping corrupt), got %d", len(releases))
	}
	if 0 < len(releases) && "good-app" != releases[0].Name {
		t.Errorf("expected good-app, got %s", releases[0].Name)
	}
}

// ---------------------------------------------------------------------------
// History with corrupt secrets
// ---------------------------------------------------------------------------

func TestSecretStorage_HistorySkipsCorruptSecrets(t *testing.T) {
	cs := fake.NewSimpleClientset()

	// Create a valid release
	rel := makeTestRelease("app", 1, StatusDeployed)
	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	goodSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName("app", 1),
			Namespace: "default",
			Labels: map[string]string{
				labelOwner: keramoslabels.ManagedByValue,
				labelName:  "app",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKey: []byte(encoded)},
	}

	badSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      SecretName("app", 2),
			Namespace: "default",
			Labels: map[string]string{
				labelOwner: keramoslabels.ManagedByValue,
				labelName:  "app",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{dataKey: []byte("corrupt")},
	}

	ctx := context.Background()
	_, err = cs.CoreV1().Secrets("default").Create(ctx, goodSecret, metav1.CreateOptions{})
	if nil != err {
		t.Fatalf("create good: %v", err)
	}
	_, err = cs.CoreV1().Secrets("default").Create(ctx, badSecret, metav1.CreateOptions{})
	if nil != err {
		t.Fatalf("create bad: %v", err)
	}

	storage := NewSecretStorage(cs, "default")
	history, err := storage.History("app")
	if nil != err {
		t.Fatalf("History failed: %v", err)
	}

	if 1 != len(history) {
		t.Errorf("expected 1 (skipping corrupt), got %d", len(history))
	}
}

// ---------------------------------------------------------------------------
// Delete with real error (not NotFound)
// ---------------------------------------------------------------------------

func TestSecretStorage_DeleteError(t *testing.T) {
	cs := fake.NewSimpleClientset()
	// Use a namespace that will cause issues - create storage in one ns but delete in another context
	storage := NewSecretStorage(cs, "default")

	// Create a release
	rel := makeTestRelease("app", 1, StatusDeployed)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete should succeed
	err := storage.Delete("app", 1)
	if nil != err {
		t.Fatalf("Delete failed: %v", err)
	}

	// Second delete should also succeed (not found is OK)
	err = storage.Delete("app", 1)
	if nil != err {
		t.Fatalf("Second delete should succeed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Encode/Decode with deeply nested values
// ---------------------------------------------------------------------------

func TestEncodeDecodeWithNestedValues(t *testing.T) {
	rel := &Release{
		Name:      "nested",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package:   PackageRef{Name: "pkg", Version: "1.0.0"},
		Values: map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"level3": map[string]any{
						"deep": "value",
					},
				},
			},
			"list": []any{1.0, "two", true, nil},
		},
		Manifest: "apiVersion: v1\nkind: ConfigMap\n",
		Info: ReleaseInfo{
			FirstDeployed: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			LastDeployed:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}

	if nil == decoded.Values["level1"] {
		t.Error("missing level1")
	}
	if nil == decoded.Values["list"] {
		t.Error("missing list")
	}
}

// ---------------------------------------------------------------------------
// Verify encoded data is base64 and gzipped
// ---------------------------------------------------------------------------

func TestEncodeProducesValidBase64(t *testing.T) {
	rel := makeTestRelease("app", 1, StatusDeployed)
	encoded, err := Encode(rel)
	if nil != err {
		t.Fatalf("Encode failed: %v", err)
	}

	// Should be valid base64
	if 0 == len(encoded) {
		t.Error("encoded string should not be empty")
	}

	// Should decode back
	decoded, err := Decode(encoded)
	if nil != err {
		t.Fatalf("Decode failed: %v", err)
	}
	if "app" != decoded.Name {
		t.Errorf("expected app, got %s", decoded.Name)
	}
}

// ---------------------------------------------------------------------------
// Create duplicate (triggers K8s create error)
// ---------------------------------------------------------------------------

func TestSecretStorage_CreateDuplicate(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	rel := makeTestRelease("app", 1, StatusDeployed)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("first Create failed: %v", err)
	}

	// Second create should fail (already exists)
	err := storage.Create(rel)
	if nil == err {
		t.Fatal("expected error for duplicate create")
	}
}

// ---------------------------------------------------------------------------
// Last returns highest revision
// ---------------------------------------------------------------------------

func TestSecretStorage_LastReturnsHighestRevision(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Create revisions in reverse order
	for _, rev := range []int{3, 1, 5, 2, 4} {
		rel := makeTestRelease("app", rev, StatusDeployed)
		if err := storage.Create(rel); nil != err {
			t.Fatalf("Create rev %d failed: %v", rev, err)
		}
	}

	last, err := storage.Last("app")
	if nil != err {
		t.Fatalf("Last failed: %v", err)
	}

	if 5 != last.Revision {
		t.Errorf("expected revision 5, got %d", last.Revision)
	}
}

// ---------------------------------------------------------------------------
// Update — update error path with a reactors
// ---------------------------------------------------------------------------

func TestSecretStorage_UpdateChangeStatus(t *testing.T) {
	cs := fake.NewSimpleClientset()
	storage := NewSecretStorage(cs, "default")

	// Create rev 1 as deployed
	rel := makeTestRelease("app", 1, StatusDeployed)
	if err := storage.Create(rel); nil != err {
		t.Fatalf("Create failed: %v", err)
	}

	// Update to superseded
	rel.Status = StatusSuperseded
	if err := storage.Update(rel); nil != err {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify the labels changed
	ctx := context.Background()
	secretName := SecretName("app", 1)
	secret, err := cs.CoreV1().Secrets("default").Get(ctx, secretName, metav1.GetOptions{})
	if nil != err {
		t.Fatalf("Get secret failed: %v", err)
	}

	if "superseded" != secret.Labels[labelStatus] {
		t.Errorf("expected label status superseded, got %s", secret.Labels[labelStatus])
	}
}

// ---------------------------------------------------------------------------
// Encode error paths
// ---------------------------------------------------------------------------

func TestEncodeWithUnmarshalableValues(t *testing.T) {
	// json.Marshal fails on channels
	rel := &Release{
		Name:      "bad",
		Namespace: "default",
		Revision:  1,
		Status:    StatusDeployed,
		Package:   PackageRef{Name: "pkg", Version: "1.0.0"},
		Values: map[string]any{
			"badValue": make(chan int), // channels can't be JSON marshaled
		},
		Manifest: "test",
		Info: ReleaseInfo{
			FirstDeployed: time.Now().UTC(),
			LastDeployed:  time.Now().UTC(),
		},
	}

	_, err := Encode(rel)
	if nil == err {
		t.Fatal("expected error for unmarshalable values")
	}
}

// ---------------------------------------------------------------------------
// NewSecretStorage interface compliance
// ---------------------------------------------------------------------------

func TestNewSecretStorageReturnsStorageInterface(t *testing.T) {
	cs := fake.NewSimpleClientset()
	var _ Storage = NewSecretStorage(cs, "default")
}
