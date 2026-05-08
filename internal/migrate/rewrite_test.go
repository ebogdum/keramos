package migrate

import "testing"

func TestRewriteSimpleHelmFunc_Tpl(t *testing.T) {
	in := `  {{ tpl .Values.template . }}`
	want := `  ${tpl Values.template}`
	got := rewriteSimpleHelmFunc(in)
	if got != want {
		t.Errorf("rewrite tpl:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_Lookup(t *testing.T) {
	in := `{{ lookup "v1" "Secret" "default" "creds" }}`
	want := `${lookup "v1" "Secret" "default" "creds"}`
	got := rewriteSimpleHelmFunc(in)
	if got != want {
		t.Errorf("rewrite lookup:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_Dict(t *testing.T) {
	in := `{{ dict "a" 1 "b" 2 }}`
	want := `${dict "a" 1 "b" 2}`
	got := rewriteSimpleHelmFunc(in)
	if got != want {
		t.Errorf("rewrite dict:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_Index(t *testing.T) {
	in := `name: {{ index .Values.x "key" }}`
	want := `name: ${get Values.x "key"}`
	got := rewriteSimpleHelmFunc(in)
	if got != want {
		t.Errorf("rewrite index:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_Printf(t *testing.T) {
	in := `value: {{ printf "%s-%s" .a .b }}`
	want := `value: ${printf "%s-%s" .a .b}`
	got := rewriteSimpleHelmFunc(in)
	if got != want {
		t.Errorf("rewrite printf:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_Untouched(t *testing.T) {
	plain := "name: hello"
	if rewriteSimpleHelmFunc(plain) != plain {
		t.Error("non-helm line should pass through unchanged")
	}
}

func TestRewriteSimpleHelmFunc_DotRoot(t *testing.T) {
	in := `name: $.Release.Name`
	got := rewriteSimpleHelmFunc(in)
	want := "name: ${values.Release.Name}"
	if got != want {
		t.Errorf("$.X rewrite:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_RangeIndexValue(t *testing.T) {
	in := `  {{ range $i, $v := .Values.items }}`
	got := rewriteSimpleHelmFunc(in)
	want := "  $each: ${values.items}\n  $as: v"
	if got != want {
		t.Errorf("range $i,$v rewrite:\n  got:  %q\n  want: %q", got, want)
	}
}

func TestRewriteSimpleHelmFunc_RangeValueOnly(t *testing.T) {
	in := `{{ range $item := .Values.list }}`
	got := rewriteSimpleHelmFunc(in)
	want := "$each: ${values.list}\n$as: item"
	if got != want {
		t.Errorf("range $v rewrite:\n  got:  %q\n  want: %q", got, want)
	}
}
