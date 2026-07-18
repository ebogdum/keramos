package repo

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestRepoConfigNewFieldsRoundTrip verifies the transport-policy fields persist
// through the repositories.yaml marshal/unmarshal cycle.
func TestRepoConfigNewFieldsRoundTrip(t *testing.T) {
	in := RepoFile{Repositories: []RepoConfig{{
		Name: "r", URL: "https://charts.example",
		InsecureSkipTLSVerify: true,
		PassCredentials:       true,
		PassCredentialsAll:    true,
	}}}
	data, err := yaml.Marshal(in)
	if nil != err {
		t.Fatalf("marshal: %v", err)
	}
	var out RepoFile
	if err := yaml.Unmarshal(data, &out); nil != err {
		t.Fatalf("unmarshal: %v", err)
	}
	got := out.Repositories[0]
	if !got.InsecureSkipTLSVerify || !got.PassCredentials || !got.PassCredentialsAll {
		t.Fatalf("policy fields lost through round-trip: %+v", got)
	}
}

// TestCredentialInsecureRoundTrip verifies login --insecure persists.
func TestCredentialInsecureRoundTrip(t *testing.T) {
	in := Credential{Type: AuthBasic, Username: "u", Insecure: true}
	data, _ := json.Marshal(in)
	var out Credential
	if err := json.Unmarshal(data, &out); nil != err {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Insecure {
		t.Fatal("Insecure flag lost through round-trip")
	}
}
