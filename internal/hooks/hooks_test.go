package hooks

import (
	"testing"
	"time"
)

func TestParseHooksBasic(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `apiVersion: batch/v1
kind: Job
metadata:
  name: pre-install-job
spec:
  template:
    spec:
      containers:
      - name: init
        image: busybox
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 1 != len(parsed) {
		t.Fatalf("expected 1 hook, got %d", len(parsed))
	}

	if PreInstall != parsed[0].Type {
		t.Errorf("expected type pre-install, got %s", parsed[0].Type)
	}
	if 0 != parsed[0].Weight {
		t.Errorf("expected weight 0, got %d", parsed[0].Weight)
	}
}

func TestParseHooksWithDirective(t *testing.T) {
	hookFiles := map[string]string{
		"post-install.yaml": `$hook:
  weight: 10
  deletePolicy: hook-succeeded
  timeout: 10m
apiVersion: batch/v1
kind: Job
metadata:
  name: post-install-job
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 1 != len(parsed) {
		t.Fatalf("expected 1 hook, got %d", len(parsed))
	}

	h := parsed[0]
	if PostInstall != h.Type {
		t.Errorf("expected type post-install, got %s", h.Type)
	}
	if 10 != h.Weight {
		t.Errorf("expected weight 10, got %d", h.Weight)
	}
	if "hook-succeeded" != h.DeletePolicy {
		t.Errorf("expected deletePolicy hook-succeeded, got %s", h.DeletePolicy)
	}
	if 10*time.Minute != h.Timeout {
		t.Errorf("expected timeout 10m, got %s", h.Timeout)
	}
}

func TestParseHooksSortedByWeight(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `$hook:
  weight: 20
apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-b
`,
		"pre-install-first.yaml": `$hook:
  weight: 5
apiVersion: v1
kind: ConfigMap
metadata:
  name: hook-a
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 2 != len(parsed) {
		t.Fatalf("expected 2 hooks, got %d", len(parsed))
	}

	if parsed[0].Weight >= parsed[1].Weight {
		t.Errorf("hooks should be sorted by weight ascending: got %d before %d", parsed[0].Weight, parsed[1].Weight)
	}
}

func TestParseHooksUnrecognizedFilename(t *testing.T) {
	hookFiles := map[string]string{
		"unknown-event.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks should not error for unrecognized hooks: %v", err)
	}

	if 0 != len(parsed) {
		t.Errorf("expected 0 hooks for unrecognized filename, got %d", len(parsed))
	}
}

func TestHookTypeFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected HookType
		wantErr  bool
	}{
		{"pre-install.yaml", PreInstall, false},
		{"post-install.yaml", PostInstall, false},
		{"pre-upgrade.yaml", PreUpgrade, false},
		{"post-upgrade.yaml", PostUpgrade, false},
		{"pre-delete.yaml", PreDelete, false},
		{"post-delete.yaml", PostDelete, false},
		{"pre-rollback.yaml", PreRollback, false},
		{"post-rollback.yaml", PostRollback, false},
		{"pre-install-init.yaml", PreInstall, false},
		{"unknown.yaml", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, err := hookTypeFromFilename(tt.filename)
			if tt.wantErr {
				if nil == err {
					t.Error("expected error, got nil")
				}
				return
			}
			if nil != err {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expected != got {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestExtractDirective(t *testing.T) {
	input := `$hook:
  weight: 5
  deletePolicy: before-hook-creation
  timeout: 2m
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`

	directive, manifest, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	if 5 != directive.Weight {
		t.Errorf("expected weight 5, got %d", directive.Weight)
	}
	if "before-hook-creation" != directive.DeletePolicy {
		t.Errorf("expected deletePolicy before-hook-creation, got %s", directive.DeletePolicy)
	}
	if "2m" != directive.Timeout {
		t.Errorf("expected timeout 2m, got %s", directive.Timeout)
	}

	// Manifest should not contain $hook
	if len(manifest) == 0 {
		t.Error("manifest should not be empty")
	}
}

func TestExtractDirectiveNoHook(t *testing.T) {
	input := `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	directive, manifest, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	if 0 != directive.Weight {
		t.Errorf("expected default weight 0, got %d", directive.Weight)
	}
	if "" != directive.DeletePolicy {
		t.Errorf("expected empty deletePolicy, got %s", directive.DeletePolicy)
	}

	if manifest != input {
		t.Error("manifest should be unchanged when no $hook directive")
	}
}

func TestFilterByType(t *testing.T) {
	allHooks := []Hook{
		{Type: PreInstall, Weight: 1},
		{Type: PostInstall, Weight: 2},
		{Type: PreInstall, Weight: 3},
		{Type: PreUpgrade, Weight: 4},
	}

	preInstalls := filterByType(allHooks, PreInstall)
	if 2 != len(preInstalls) {
		t.Errorf("expected 2 pre-install hooks, got %d", len(preInstalls))
	}

	postInstalls := filterByType(allHooks, PostInstall)
	if 1 != len(postInstalls) {
		t.Errorf("expected 1 post-install hook, got %d", len(postInstalls))
	}

	preDeletes := filterByType(allHooks, PreDelete)
	if 0 != len(preDeletes) {
		t.Errorf("expected 0 pre-delete hooks, got %d", len(preDeletes))
	}
}
