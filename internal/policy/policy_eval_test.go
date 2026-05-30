package policy

import (
	"testing"
)

const podManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: web
          image: docker.io/library/nginx:latest
`

func TestEvaluateMinReplicasDeny(t *testing.T) {
	rules := []Rule{{
		Name:     "min-replicas",
		Severity: SeverityDeny,
		Match:    Match{Kinds: []string{"Deployment"}},
		Require:  Require{MinReplicas: 3},
		Message:  "need >=3 replicas",
	}}
	vs, err := Evaluate(rules, podManifest)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 == len(vs) {
		t.Fatal("expected a MinReplicas violation (replicas=1 < 3)")
	}
	if !HasDeny(vs) {
		t.Fatal("expected HasDeny true for an error-severity violation")
	}
}

func TestEvaluateImageRegistryAllowlist(t *testing.T) {
	rules := []Rule{{
		Name:    "registry-allowlist",
		Match:   Match{Kinds: []string{"Deployment"}},
		Require: Require{ImageRegistries: []string{"registry.internal"}},
	}}
	vs, err := Evaluate(rules, podManifest)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 == len(vs) {
		t.Fatal("expected a violation: docker.io image not in the registry.internal allowlist")
	}
}

func TestEvaluateImageNotTagged(t *testing.T) {
	rules := []Rule{{
		Name:    "no-latest",
		Match:   Match{Kinds: []string{"Deployment"}},
		Require: Require{ImageNotTagged: true},
	}}
	vs, err := Evaluate(rules, podManifest)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 == len(vs) {
		t.Fatal("expected a violation: image uses :latest")
	}
}

func TestEvaluateAllowsCompliantManifest(t *testing.T) {
	good := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: web
          image: registry.internal/web:1.2.3
`
	rules := []Rule{{
		Name:  "compliance",
		Match: Match{Kinds: []string{"Deployment"}},
		Require: Require{
			MinReplicas:     3,
			ImageRegistries: []string{"registry.internal"},
			ImageNotTagged:  true,
		},
	}}
	vs, err := Evaluate(rules, good)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 != len(vs) {
		t.Fatalf("expected no violations for compliant manifest, got %+v", vs)
	}
}

func TestEvaluateMatchScoping(t *testing.T) {
	// A rule scoped to StatefulSet must NOT fire on a Deployment.
	rules := []Rule{{
		Name:    "sts-only",
		Match:   Match{Kinds: []string{"StatefulSet"}},
		Require: Require{MinReplicas: 99},
	}}
	vs, err := Evaluate(rules, podManifest)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 != len(vs) {
		t.Fatalf("rule scoped to StatefulSet should not fire on Deployment, got %+v", vs)
	}
}
