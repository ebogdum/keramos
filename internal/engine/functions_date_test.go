package engine

import (
	"testing"
	"time"
)

func TestStrftimeLayoutConversion(t *testing.T) {
	cases := map[string]string{
		"%Y-%m-%d":         "2006-01-02",
		"%H:%M:%S":         "15:04:05",
		"%Y-%m-%dT%H:%M:%S": "2006-01-02T15:04:05",
		"plain literal":    "plain literal",
		"%%":               "%",
	}
	for in, want := range cases {
		got := goLayout(in)
		if got != want {
			t.Errorf("goLayout(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFnDateStrftime(t *testing.T) {
	stamp := time.Date(2024, 6, 15, 12, 30, 45, 0, time.UTC)
	got, err := fnDate(stamp, "%Y-%m-%d")
	if nil != err {
		t.Fatalf("date: %v", err)
	}
	if "2024-06-15" != got {
		t.Errorf("date = %v, want 2024-06-15", got)
	}
}

func TestFnNow(t *testing.T) {
	got, err := fnNow(nil)
	if nil != err {
		t.Fatalf("now: %v", err)
	}
	if _, ok := got.(time.Time); !ok {
		t.Errorf("now type = %T, want time.Time", got)
	}
}

func TestFnToDate(t *testing.T) {
	got, err := fnToDate("2024-06-15", "2006-01-02")
	if nil != err {
		t.Fatalf("toDate: %v", err)
	}
	tm := got.(time.Time)
	if 2024 != tm.Year() {
		t.Errorf("toDate year = %d", tm.Year())
	}
}
