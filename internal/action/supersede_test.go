package action

import (
	"testing"

	"github.com/ebogdum/keramos/v3/internal/release"
)

// TestSupersedeOtherDeployedSelfHeals proves that marking a new revision the
// single Deployed one demotes a STALE Deployed revision left by a prior run.
func TestSupersedeOtherDeployedSelfHeals(t *testing.T) {
	st := release.NewMemoryStorage()
	// Two revisions both Deployed (a prior supersede was missed / crashed).
	_ = st.Create(&release.Release{Name: "r", Revision: 1, Status: release.StatusDeployed})
	_ = st.Create(&release.Release{Name: "r", Revision: 2, Status: release.StatusDeployed})

	supersedeOtherDeployed(st, "r", 2)

	hist, _ := st.History("r")
	deployed := 0
	for _, h := range hist {
		if release.StatusDeployed == h.Status {
			deployed++
			if 2 != h.Revision {
				t.Fatalf("only the newest (rev 2) should stay Deployed, got rev %d", h.Revision)
			}
		}
	}
	if 1 != deployed {
		t.Fatalf("expected exactly one Deployed revision after self-heal, got %d", deployed)
	}
}

// TestPruneKeepsNewestDeployedPrunesStale proves prune keeps the newest Deployed
// and can prune an older stale-Deployed revision.
func TestPruneKeepsNewestDeployedPrunesStale(t *testing.T) {
	st := release.NewMemoryStorage()
	// rev1 stale-Deployed, rev2..4 superseded, rev5 Deployed (current).
	_ = st.Create(&release.Release{Name: "r", Revision: 1, Status: release.StatusDeployed})
	_ = st.Create(&release.Release{Name: "r", Revision: 2, Status: release.StatusSuperseded})
	_ = st.Create(&release.Release{Name: "r", Revision: 3, Status: release.StatusSuperseded})
	_ = st.Create(&release.Release{Name: "r", Revision: 4, Status: release.StatusSuperseded})
	_ = st.Create(&release.Release{Name: "r", Revision: 5, Status: release.StatusDeployed})

	pruneReleaseHistory(st, "r", 2) // keep 2 most recent

	hist, _ := st.History("r")
	haveNewest := false
	for _, h := range hist {
		if 1 == h.Revision {
			t.Fatal("stale-Deployed rev 1 should have been pruned")
		}
		if 5 == h.Revision {
			haveNewest = true
		}
	}
	if !haveNewest {
		t.Fatal("newest Deployed rev 5 must never be pruned")
	}
}
