package hooks

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ParseHooks — additional cases
// ---------------------------------------------------------------------------

func TestParseHooks_EmptyMap(t *testing.T) {
	parsed, err := ParseHooks(map[string]string{})
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(parsed) {
		t.Errorf("expected 0 hooks, got %d", len(parsed))
	}
}

func TestParseHooks_NilMap(t *testing.T) {
	parsed, err := ParseHooks(nil)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if 0 != len(parsed) {
		t.Errorf("expected 0 hooks, got %d", len(parsed))
	}
}

func TestParseHooks_AllHookTypes(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml":   "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pi\n",
		"post-install.yaml":  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: poi\n",
		"pre-upgrade.yaml":   "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pu\n",
		"post-upgrade.yaml":  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pou\n",
		"pre-delete.yaml":    "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pd\n",
		"post-delete.yaml":   "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pod\n",
		"pre-rollback.yaml":  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pr\n",
		"post-rollback.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: por\n",
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 8 != len(parsed) {
		t.Errorf("expected 8 hooks, got %d", len(parsed))
	}

	typeSet := make(map[HookType]bool, len(parsed))
	for _, h := range parsed {
		typeSet[h.Type] = true
	}

	expectedTypes := []HookType{
		PreInstall, PostInstall, PreUpgrade, PostUpgrade,
		PreDelete, PostDelete, PreRollback, PostRollback,
	}
	for _, et := range expectedTypes {
		if !typeSet[et] {
			t.Errorf("missing hook type: %s", et)
		}
	}
}

func TestParseHooks_InvalidTimeout(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `$hook:
  timeout: not-a-duration
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
	}

	_, err := ParseHooks(hookFiles)
	if nil == err {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestParseHooks_DefaultTimeout(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 1 != len(parsed) {
		t.Fatalf("expected 1 hook, got %d", len(parsed))
	}

	if defaultHookTimeout != parsed[0].Timeout {
		t.Errorf("expected default timeout %v, got %v", defaultHookTimeout, parsed[0].Timeout)
	}
}

func TestParseHooks_WeightSortingMultiple(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `$hook:
  weight: 100
apiVersion: v1
kind: ConfigMap
metadata:
  name: last
`,
		"pre-install-second.yaml": `$hook:
  weight: 50
apiVersion: v1
kind: ConfigMap
metadata:
  name: middle
`,
		"pre-install-first.yaml": `$hook:
  weight: -10
apiVersion: v1
kind: ConfigMap
metadata:
  name: first
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 3 != len(parsed) {
		t.Fatalf("expected 3 hooks, got %d", len(parsed))
	}

	if parsed[0].Weight >= parsed[1].Weight || parsed[1].Weight >= parsed[2].Weight {
		t.Errorf("hooks not sorted by weight: %d, %d, %d",
			parsed[0].Weight, parsed[1].Weight, parsed[2].Weight)
	}
}

func TestParseHooks_MixedRecognizedAndUnrecognized(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: valid
`,
		"unknown-type.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: invalid
`,
		"post-delete.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: valid2
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 2 != len(parsed) {
		t.Errorf("expected 2 hooks (skipping unknown), got %d", len(parsed))
	}
}

func TestParseHooks_DeletePolicyPreserved(t *testing.T) {
	policies := []string{"hook-succeeded", "hook-failed", "before-hook-creation"}

	for _, policy := range policies {
		hookFiles := map[string]string{
			"pre-install.yaml": "$hook:\n  deletePolicy: " + policy + "\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n",
		}

		parsed, err := ParseHooks(hookFiles)
		if nil != err {
			t.Fatalf("ParseHooks failed for policy %s: %v", policy, err)
		}

		if 1 != len(parsed) {
			t.Fatalf("expected 1 hook, got %d", len(parsed))
		}

		if policy != parsed[0].DeletePolicy {
			t.Errorf("expected deletePolicy %s, got %s", policy, parsed[0].DeletePolicy)
		}
	}
}

func TestParseHooks_YmlExtension(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: yml-test
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
		t.Errorf("expected pre-install, got %s", parsed[0].Type)
	}
}

func TestParseHooks_CustomTimeout(t *testing.T) {
	hookFiles := map[string]string{
		"pre-install.yaml": `$hook:
  timeout: 30s
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
	}

	parsed, err := ParseHooks(hookFiles)
	if nil != err {
		t.Fatalf("ParseHooks failed: %v", err)
	}

	if 30*time.Second != parsed[0].Timeout {
		t.Errorf("expected 30s timeout, got %v", parsed[0].Timeout)
	}
}

// ---------------------------------------------------------------------------
// hookTypeFromFilename — additional cases
// ---------------------------------------------------------------------------

func TestHookTypeFromFilename_WeightSuffix(t *testing.T) {
	tests := []struct {
		filename string
		expected HookType
	}{
		{"pre-install-10.yaml", PreInstall},
		{"post-install-migration.yaml", PostInstall},
		{"pre-upgrade-check.yml", PreUpgrade},
		{"post-upgrade-notify.yml", PostUpgrade},
		{"pre-delete-backup.yaml", PreDelete},
		{"post-delete-cleanup.yaml", PostDelete},
		{"pre-rollback-verify.yaml", PreRollback},
		{"post-rollback-notify.yaml", PostRollback},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, err := hookTypeFromFilename(tt.filename)
			if nil != err {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.expected != got {
				t.Errorf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestHookTypeFromFilename_UnrecognizedNames(t *testing.T) {
	names := []string{
		"unknown.yaml",
		"install.yaml",
		"delete.yaml",
		"upgrade.yaml",
		"rollback.yaml",
		"random.yml",
		"",
	}

	for _, name := range names {
		_, err := hookTypeFromFilename(name)
		if nil == err {
			t.Errorf("expected error for filename %q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// extractDirective — additional cases
// ---------------------------------------------------------------------------

func TestExtractDirective_WeightAsFloat(t *testing.T) {
	input := `$hook:
  weight: 10.0
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	directive, _, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	// YAML parses 10.0 as float64
	if 10 != directive.Weight {
		t.Errorf("expected weight 10, got %d", directive.Weight)
	}
}

func TestExtractDirective_WeightAsString(t *testing.T) {
	input := `$hook:
  weight: "15"
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	directive, _, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	if 15 != directive.Weight {
		t.Errorf("expected weight 15, got %d", directive.Weight)
	}
}

func TestExtractDirective_InvalidWeightString(t *testing.T) {
	input := `$hook:
  weight: "not-a-number"
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	_, _, err := extractDirective(input)
	if nil == err {
		t.Fatal("expected error for invalid weight string")
	}
}

func TestExtractDirective_NonMapContent(t *testing.T) {
	input := `just a plain string`

	directive, manifest, err := extractDirective(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if 0 != directive.Weight {
		t.Errorf("expected default weight 0, got %d", directive.Weight)
	}
	if manifest != input {
		t.Error("manifest should be unchanged for non-map content")
	}
}

func TestExtractDirective_HookNotMap(t *testing.T) {
	input := `$hook: just-a-string
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	directive, _, err := extractDirective(input)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-map $hook should be ignored
	if 0 != directive.Weight {
		t.Errorf("expected default weight, got %d", directive.Weight)
	}
}

func TestExtractDirective_OnlyHookNoOtherFields(t *testing.T) {
	input := `$hook:
  weight: 5
  deletePolicy: hook-succeeded
  timeout: 1m
`

	directive, manifest, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	if 5 != directive.Weight {
		t.Errorf("expected weight 5, got %d", directive.Weight)
	}
	if "hook-succeeded" != directive.DeletePolicy {
		t.Errorf("expected hook-succeeded, got %s", directive.DeletePolicy)
	}
	if "1m" != directive.Timeout {
		t.Errorf("expected 1m, got %s", directive.Timeout)
	}

	// The manifest should be the remaining YAML after removing $hook
	// It should be a valid (empty or near-empty) YAML document
	if strings.Contains(manifest, "$hook") {
		t.Error("manifest should not contain $hook after extraction")
	}
}

func TestExtractDirective_PartialDirective(t *testing.T) {
	input := `$hook:
  weight: 3
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`

	directive, manifest, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	if 3 != directive.Weight {
		t.Errorf("expected weight 3, got %d", directive.Weight)
	}
	if "" != directive.DeletePolicy {
		t.Errorf("expected empty deletePolicy, got %s", directive.DeletePolicy)
	}
	if "" != directive.Timeout {
		t.Errorf("expected empty timeout, got %s", directive.Timeout)
	}
	if strings.Contains(manifest, "$hook") {
		t.Error("$hook should be removed from manifest")
	}
}

func TestExtractDirective_PreservesOtherFields(t *testing.T) {
	input := `$hook:
  weight: 1
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-cm
data:
  key: value
`

	_, manifest, err := extractDirective(input)
	if nil != err {
		t.Fatalf("extractDirective failed: %v", err)
	}

	if !strings.Contains(manifest, "apiVersion") {
		t.Error("manifest missing apiVersion")
	}
	if !strings.Contains(manifest, "ConfigMap") {
		t.Error("manifest missing kind")
	}
	if !strings.Contains(manifest, "my-cm") {
		t.Error("manifest missing name")
	}
	if !strings.Contains(manifest, "key: value") {
		t.Error("manifest missing data")
	}
	if strings.Contains(manifest, "$hook") {
		t.Error("manifest should not contain $hook")
	}
}

// ---------------------------------------------------------------------------
// filterByType — additional cases
// ---------------------------------------------------------------------------

func TestFilterByType_EmptyInput(t *testing.T) {
	result := filterByType(nil, PreInstall)
	if 0 != len(result) {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestFilterByType_NoMatch(t *testing.T) {
	hooks := []Hook{
		{Type: PreInstall, Weight: 1},
		{Type: PostInstall, Weight: 2},
	}

	result := filterByType(hooks, PreDelete)
	if 0 != len(result) {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestFilterByType_AllMatch(t *testing.T) {
	hooks := []Hook{
		{Type: PreInstall, Weight: 1},
		{Type: PreInstall, Weight: 2},
		{Type: PreInstall, Weight: 3},
	}

	result := filterByType(hooks, PreInstall)
	if 3 != len(result) {
		t.Errorf("expected 3, got %d", len(result))
	}
}

func TestFilterByType_AllTypes(t *testing.T) {
	allTypes := []HookType{
		PreInstall, PostInstall, PreUpgrade, PostUpgrade,
		PreDelete, PostDelete, PreRollback, PostRollback,
	}

	hooks := make([]Hook, 0, len(allTypes))
	for _, ht := range allTypes {
		hooks = append(hooks, Hook{Type: ht, Weight: 0})
	}

	for _, ht := range allTypes {
		result := filterByType(hooks, ht)
		if 1 != len(result) {
			t.Errorf("type %s: expected 1, got %d", ht, len(result))
		}
		if ht != result[0].Type {
			t.Errorf("type %s: got %s", ht, result[0].Type)
		}
	}
}

// ---------------------------------------------------------------------------
// Hook constants
// ---------------------------------------------------------------------------

func TestHookTypeConstants(t *testing.T) {
	types := []HookType{
		PreInstall, PostInstall, PreUpgrade, PostUpgrade,
		PreDelete, PostDelete, PreRollback, PostRollback,
	}

	seen := make(map[HookType]bool, len(types))
	for _, ht := range types {
		if seen[ht] {
			t.Errorf("duplicate hook type: %s", ht)
		}
		seen[ht] = true
		if "" == string(ht) {
			t.Error("hook type should not be empty")
		}
	}

	if 8 != len(types) {
		t.Errorf("expected 8 hook types, got %d", len(types))
	}
}

func TestDefaultHookTimeout(t *testing.T) {
	if 5*time.Minute != defaultHookTimeout {
		t.Errorf("expected 5m default timeout, got %v", defaultHookTimeout)
	}
}

// ---------------------------------------------------------------------------
// deleteHookResources — string matching logic
// ---------------------------------------------------------------------------

func TestDeleteHookResourcesNotFoundDetection(t *testing.T) {
	tests := []struct {
		name        string
		errMsg      string
		shouldMatch bool
	}{
		{"lowercase not found", "resource not found", true},
		{"camelCase NotFound", "resource NotFound in cluster", true},
		{"permission denied", "permission denied", false},
		{"timeout", "context deadline exceeded", false},
		{"network error", "connection refused", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := strings.Contains(tt.errMsg, "not found") || strings.Contains(tt.errMsg, "NotFound")
			if tt.shouldMatch != matched {
				t.Errorf("expected match=%v for %q", tt.shouldMatch, tt.errMsg)
			}
		})
	}
}
