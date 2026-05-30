package diff

import (
	"testing"
)

const oldDeploy = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 2
status:
  readyReplicas: 2
`

const newDeploy = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 5
status:
  readyReplicas: 0
`

func TestComputeDetectsRealChange(t *testing.T) {
	changes, err := Compute(oldDeploy, newDeploy, Filters{})
	if nil != err {
		t.Fatalf("compute: %v", err)
	}
	if 1 != len(changes) {
		t.Fatalf("expected 1 changed resource, got %d", len(changes))
	}
	if ChangeModify != changes[0].Kind {
		t.Fatalf("expected modify, got %s", changes[0].Kind)
	}
	foundReplicas := false
	for _, fc := range changes[0].FieldDiff {
		if "spec.replicas" == fc.Path {
			foundReplicas = true
		}
		if "status.readyReplicas" == fc.Path {
			t.Errorf("status change should be filtered as noise by default")
		}
	}
	if !foundReplicas {
		t.Fatalf("expected spec.replicas field change, got %+v", changes[0].FieldDiff)
	}
}

func TestComputeStatusNoiseOnlyIsNoChange(t *testing.T) {
	// Only .status differs -> filtered to nothing by default.
	changes, err := Compute(oldDeploy, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 2
status:
  readyReplicas: 99
`, Filters{})
	if nil != err {
		t.Fatalf("compute: %v", err)
	}
	if 0 != len(changes) {
		t.Fatalf("expected status-only diff to be filtered to zero changes, got %+v", changes)
	}
}

func TestComputeShowStatusUnfiltersIt(t *testing.T) {
	changes, err := Compute(oldDeploy, `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 2
status:
  readyReplicas: 99
`, Filters{ShowStatus: true})
	if nil != err {
		t.Fatalf("compute: %v", err)
	}
	if 0 == len(changes) {
		t.Fatal("expected status change to surface when ShowStatus=true")
	}
}

func TestComputeAddAndRemove(t *testing.T) {
	add, err := Compute("", newDeploy, Filters{})
	if nil != err {
		t.Fatalf("compute add: %v", err)
	}
	if 1 != len(add) || ChangeAdd != add[0].Kind {
		t.Fatalf("expected one ChangeAdd, got %+v", add)
	}
	rem, err := Compute(oldDeploy, "", Filters{})
	if nil != err {
		t.Fatalf("compute remove: %v", err)
	}
	if 1 != len(rem) || ChangeRemove != rem[0].Kind {
		t.Fatalf("expected one ChangeRemove, got %+v", rem)
	}
}
