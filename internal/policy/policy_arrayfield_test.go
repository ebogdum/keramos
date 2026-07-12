package policy

import "testing"

// privilegedManifest has the privileged flag nested UNDER an array
// (spec.template.spec.containers[]), which is the realistic shape.
const privilegedManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: side
          image: busybox
        - name: web
          image: nginx
          securityContext:
            privileged: true
`

// TestForbidFieldAcrossArray proves a forbid rule whose path crosses a slice is
// actually enforced. Previously getDotted stopped at the containers array and
// returned "not found", silently allowing privileged: true to pass — a
// fail-open in a security control.
func TestForbidFieldAcrossArray(t *testing.T) {
	rules := []Rule{{
		Name:     "no-privileged",
		Severity: SeverityDeny,
		Match:    Match{Kinds: []string{"Deployment"}},
		Forbid:   Require{Fields: []string{"spec.template.spec.containers.securityContext.privileged"}},
		Message:  "privileged containers are forbidden",
	}}
	vs, err := Evaluate(rules, privilegedManifest)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 == len(vs) {
		t.Fatal("expected a violation: privileged:true nested under containers[] must be caught")
	}
	if !HasDeny(vs) {
		t.Fatal("expected a deny-severity violation")
	}
}

// TestForbidFieldAcrossArrayClean confirms the array-aware traversal does not
// produce false positives when no container sets the forbidden field.
func TestForbidFieldAcrossArrayClean(t *testing.T) {
	const clean = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx
          securityContext:
            privileged: false
`
	rules := []Rule{{
		Name:    "no-privileged",
		Match:   Match{Kinds: []string{"Deployment"}},
		Forbid:  Require{Fields: []string{"spec.template.spec.containers.securityContext.privileged"}},
		Message: "privileged containers are forbidden",
	}}
	vs, err := Evaluate(rules, clean)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 != len(vs) {
		t.Fatalf("expected no violation for privileged:false, got %d", len(vs))
	}
}

// TestRequireFieldAcrossArray confirms require semantics also traverse arrays:
// a required field satisfied inside a container element is recognized.
func TestRequireFieldAcrossArray(t *testing.T) {
	rules := []Rule{{
		Name:    "require-securityContext",
		Match:   Match{Kinds: []string{"Deployment"}},
		Require: Require{Fields: []string{"spec.template.spec.containers.securityContext.privileged"}},
		Message: "securityContext.privileged must be set",
	}}
	// privilegedManifest has one container WITH the field set (non-zero) → satisfied.
	vs, err := Evaluate(rules, privilegedManifest)
	if nil != err {
		t.Fatalf("evaluate: %v", err)
	}
	if 0 != len(vs) {
		t.Fatalf("required field present in a container should satisfy the rule, got %d violations", len(vs))
	}
}
