package release

import (
	"os"
	"path/filepath"
	"testing"
)

// End-to-end proof that the SQL backend actually works in the shipped binary:
// the modernc.org/sqlite driver is registered, SelectStorage opens it via env,
// and a release round-trips through Create/Get/List/History/Delete.
func TestSQLStorageSQLiteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "keramos.db")

	t.Setenv("KERAMOS_DRIVER", "sql")
	t.Setenv("KERAMOS_DRIVER_SQL_DRIVER", "sqlite")
	t.Setenv("KERAMOS_DRIVER_SQL_DSN", dsn)

	st, err := SelectStorage(nil, "default")
	if nil != err {
		t.Fatalf("SelectStorage(sql/sqlite) failed — driver not registered? %v", err)
	}

	rel := &Release{Name: "myapp", Revision: 1, Namespace: "default", Status: StatusDeployed, Manifest: "kind: ConfigMap\n"}
	if err := st.Create(rel); nil != err {
		t.Fatalf("Create: %v", err)
	}

	got, err := st.Get("myapp", 1)
	if nil != err {
		t.Fatalf("Get: %v", err)
	}
	if got.Manifest != rel.Manifest || StatusDeployed != got.Status {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	rel2 := &Release{Name: "myapp", Revision: 2, Namespace: "default", Status: StatusDeployed}
	if err := st.Create(rel2); nil != err {
		t.Fatalf("Create rev2: %v", err)
	}

	hist, err := st.History("myapp")
	if nil != err {
		t.Fatalf("History: %v", err)
	}
	if 2 != len(hist) {
		t.Fatalf("expected 2 revisions, got %d", len(hist))
	}

	last, err := st.Last("myapp")
	if nil != err || 2 != last.Revision {
		t.Fatalf("Last: %+v err=%v", last, err)
	}

	if err := st.Delete("myapp", 1); nil != err {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get("myapp", 1); nil == err {
		t.Fatal("expected Get of deleted revision to fail")
	}

	// The DB file must actually exist on disk (proves a real driver ran).
	if _, statErr := os.Stat(dsn); nil != statErr {
		t.Fatalf("sqlite db file not created: %v", statErr)
	}
}
