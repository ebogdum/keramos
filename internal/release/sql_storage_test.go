package release

import "testing"

func TestSQLStorage_PostgresPlaceholders(t *testing.T) {
	s := &SQLStorage{dialect: DialectPostgres}
	got := s.ph(`SELECT * FROM x WHERE a = ? AND b = ? AND c = ?`)
	want := `SELECT * FROM x WHERE a = $1 AND b = $2 AND c = $3`
	if got != want {
		t.Errorf("postgres placeholder rewrite mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

func TestSQLStorage_StandardPlaceholders(t *testing.T) {
	s := &SQLStorage{dialect: DialectStandard}
	got := s.ph(`SELECT * FROM x WHERE a = ? AND b = ?`)
	want := `SELECT * FROM x WHERE a = ? AND b = ?`
	if got != want {
		t.Errorf("standard placeholder rewrite mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}
