package repo

import (
	"testing"
)

func TestValidateScopedName_Valid(t *testing.T) {
	validNames := []string{
		"myapp",
		"redis",
		"my.app",
		"app123",
		"@myorg/myapp",
		"@my-org/my.app",
		"@org123/pkg",
		"@a/b",
	}

	for _, name := range validNames {
		if err := ValidateScopedName(name); nil != err {
			t.Errorf("expected %q to be valid, got error: %v", name, err)
		}
	}
}

func TestValidateScopedName_Invalid(t *testing.T) {
	invalidNames := []string{
		"",
		"@/foo",
		"@@org/foo",
		"@org/",
		"@ORG/foo",
		"MyApp",
		"-badstart",
		"@-bad/pkg",
		".dotstart",
		"@org/Bad",
	}

	for _, name := range invalidNames {
		if err := ValidateScopedName(name); nil == err {
			t.Errorf("expected %q to be invalid, but got nil error", name)
		}
	}
}

func TestIsScoped(t *testing.T) {
	tests := []struct {
		name   string
		expect bool
	}{
		{"myapp", false},
		{"@myorg/myapp", true},
		{"@a/b", true},
		{"redis", false},
		{"@noSlash", false},
	}

	for _, tc := range tests {
		got := IsScoped(tc.name)
		if got != tc.expect {
			t.Errorf("IsScoped(%q) = %v, want %v", tc.name, got, tc.expect)
		}
	}
}

func TestScopeAndName(t *testing.T) {
	tests := []struct {
		input     string
		wantScope string
		wantName  string
	}{
		{"myapp", "", "myapp"},
		{"@myorg/redis", "@myorg", "redis"},
		{"@a/b", "@a", "b"},
		{"plain", "", "plain"},
	}

	for _, tc := range tests {
		scope, name := ScopeAndName(tc.input)
		if scope != tc.wantScope || name != tc.wantName {
			t.Errorf("ScopeAndName(%q) = (%q, %q), want (%q, %q)", tc.input, scope, name, tc.wantScope, tc.wantName)
		}
	}
}

func TestArchiveFileName(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"redis", "1.0.0", "redis-1.0.0.keramos.tgz"},
		{"@myorg/redis", "1.0.0", "@myorg-redis-1.0.0.keramos.tgz"},
		{"@a/b", "2.3.4", "@a-b-2.3.4.keramos.tgz"},
	}

	for _, tc := range tests {
		got := ArchiveFileName(tc.name, tc.version)
		if got != tc.want {
			t.Errorf("ArchiveFileName(%q, %q) = %q, want %q", tc.name, tc.version, got, tc.want)
		}
	}
}

func TestPackageDir(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"redis", "packages/redis"},
		{"@myorg/redis", "packages/@myorg/redis"},
		{"@a/b", "packages/@a/b"},
	}

	for _, tc := range tests {
		got := PackageDir(tc.name)
		if got != tc.want {
			t.Errorf("PackageDir(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
