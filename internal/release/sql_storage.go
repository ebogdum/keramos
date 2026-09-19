package release

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
)

// Dialect identifies the SQL flavor for parameter binding. Postgres uses
// `$1`/`$2`; sqlite/mysql/etc. use `?`.
type Dialect int

const (
	DialectStandard Dialect = iota // sqlite, mysql, mssql — `?` placeholders
	DialectPostgres                // postgres — `$N` placeholders
)

// SQLStorage persists releases in a SQL database. The caller supplies any
// *sql.DB compatible with `database/sql` (postgres, mysql, sqlite, etc.).
//
// The schema is versioned via keramos_releases_meta(version) and migrated on
// first use:
//
//	CREATE TABLE IF NOT EXISTS keramos_releases (
//	    name        TEXT NOT NULL,
//	    namespace   TEXT NOT NULL,
//	    revision    INTEGER NOT NULL,
//	    status      TEXT NOT NULL,
//	    body        TEXT NOT NULL,
//	    created_at  TIMESTAMP NOT NULL,
//	    PRIMARY KEY (namespace, name, revision)
//	)
type SQLStorage struct {
	db        *sql.DB
	namespace string
	dialect   Dialect
}

// NewSQLStorageWithDialect lets the caller pick the placeholder style.
func NewSQLStorageWithDialect(db *sql.DB, namespace string, dialect Dialect) (Storage, error) {
	s := &SQLStorage{db: db, namespace: namespace, dialect: dialect}
	if err := s.ensureSchema(); nil != err {
		return nil, err
	}
	return s, nil
}

// ph rewrites `?` placeholders to the dialect-appropriate form. Callers
// continue to author SQL with `?` for portability.
func (s *SQLStorage) ph(query string) string {
	if DialectPostgres != s.dialect {
		return query
	}
	out := make([]byte, 0, len(query)+4)
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		if '?' == c {
			n++
			out = append(out, '$')
			out = append(out, []byte(fmt.Sprintf("%d", n))...)
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

const sqlSchemaVersion = 1

// sqlMigrations is indexed by target version (1-based). To bump the schema,
// append a new entry and `sqlSchemaVersion` follows. Each migration is run
// inside a transaction.
var sqlMigrations = []string{
	`CREATE TABLE IF NOT EXISTS keramos_releases (
		name        TEXT NOT NULL,
		namespace   TEXT NOT NULL,
		revision    INTEGER NOT NULL,
		status      TEXT NOT NULL,
		body        TEXT NOT NULL,
		created_at  TIMESTAMP NOT NULL,
		PRIMARY KEY (namespace, name, revision)
	)`,
}

func (s *SQLStorage) ensureSchema() error {
	ctx, cancel := newSQLContext()
	defer cancel()

	// Schema-version table: tracks what's been applied so re-runs are idempotent.
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS keramos_releases_meta (version INTEGER NOT NULL)`); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRelease, "failed to create keramos_releases_meta", err)
	}

	// Run each pending migration inside its own transaction so a partial
	// failure leaves the schema in a coherent state. Two concurrent
	// initialisers would still race on read-then-write of the version
	// counter; if either commit fails (e.g. duplicate-row insertion) the
	// migration is rolled back and the loser retries on the next call.
	for {
		var current int
		row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM keramos_releases_meta`)
		if err := row.Scan(&current); nil != err {
			return keramoserr.WrapError(keramoserr.ErrRelease, "failed to read schema version", err)
		}
		if current >= sqlSchemaVersion {
			return nil
		}
		next := current + 1
		tx, beginErr := s.db.BeginTx(ctx, nil)
		if nil != beginErr {
			return keramoserr.WrapErrorf(keramoserr.ErrRelease, beginErr,
				"failed to begin schema migration %d", next)
		}
		if _, err := tx.ExecContext(ctx, sqlMigrations[next-1]); nil != err {
			_ = tx.Rollback()
			return keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "schema migration %d failed", next)
		}
		if _, err := tx.ExecContext(ctx,
			s.ph(`INSERT INTO keramos_releases_meta (version) VALUES (?)`), next); nil != err {
			_ = tx.Rollback()
			return keramoserr.WrapErrorf(keramoserr.ErrRelease, err,
				"failed to record schema version %d", next)
		}
		if cErr := tx.Commit(); nil != cErr {
			return keramoserr.WrapErrorf(keramoserr.ErrRelease, cErr,
				"failed to commit schema migration %d", next)
		}
	}
}

func newSQLContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func (s *SQLStorage) Create(rel *Release) error {
	body, err := Encode(rel)
	if nil != err {
		return err
	}
	ctx, cancel := newSQLContext()
	defer cancel()
	_, execErr := s.db.ExecContext(ctx, s.ph(
		`INSERT INTO keramos_releases (name, namespace, revision, status, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`),
		rel.Name, rel.Namespace, rel.Revision, string(rel.Status), body, time.Now().UTC())
	if nil != execErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, execErr, "failed to insert release %s v%d", rel.Name, rel.Revision)
	}
	return nil
}

func (s *SQLStorage) Update(rel *Release) error {
	body, err := Encode(rel)
	if nil != err {
		return err
	}
	ctx, cancel := newSQLContext()
	defer cancel()
	res, execErr := s.db.ExecContext(ctx, s.ph(
		`UPDATE keramos_releases SET body = ?, status = ?
		 WHERE namespace = ? AND name = ? AND revision = ?`),
		body, string(rel.Status), rel.Namespace, rel.Name, rel.Revision)
	if nil != execErr {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, execErr, "failed to update release %s v%d", rel.Name, rel.Revision)
	}
	rows, _ := res.RowsAffected()
	if 0 == rows {
		return keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s v%d not found for update", rel.Name, rel.Revision)
	}
	return nil
}

func (s *SQLStorage) Get(name string, revision int) (*Release, error) {
	ctx, cancel := newSQLContext()
	defer cancel()
	row := s.db.QueryRowContext(ctx, s.ph(
		`SELECT body FROM keramos_releases WHERE namespace = ? AND name = ? AND revision = ?`),
		s.namespace, name, revision)
	var body string
	if err := row.Scan(&body); nil != err {
		if sql.ErrNoRows == err {
			return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s v%d not found", name, revision)
		}
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to scan release row", err)
	}
	return Decode(body)
}

func (s *SQLStorage) List(namespace string) ([]*Release, error) {
	ctx, cancel := newSQLContext()
	defer cancel()

	var rows *sql.Rows
	var err error
	if "" == namespace {
		rows, err = s.db.QueryContext(ctx, `SELECT body FROM keramos_releases`)
	} else {
		rows, err = s.db.QueryContext(ctx, s.ph(`SELECT body FROM keramos_releases WHERE namespace = ?`), namespace)
	}
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to list releases", err)
	}
	defer rows.Close()

	out := make([]*Release, 0)
	for rows.Next() {
		var body string
		if scanErr := rows.Scan(&body); nil != scanErr {
			return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to scan release row", scanErr)
		}
		rel, decErr := Decode(body)
		if nil != decErr {
			continue
		}
		out = append(out, rel)
	}
	if rowsErr := rows.Err(); nil != rowsErr {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "row iteration failed", rowsErr)
	}
	return out, nil
}

func (s *SQLStorage) Last(name string) (*Release, error) {
	history, err := s.History(name)
	if nil != err {
		return nil, err
	}
	if 0 == len(history) {
		return nil, keramoserr.NewErrorf(keramoserr.ErrReleaseNotFound, "release %s not found", name)
	}
	return history[len(history)-1], nil
}

func (s *SQLStorage) History(name string) ([]*Release, error) {
	ctx, cancel := newSQLContext()
	defer cancel()
	rows, err := s.db.QueryContext(ctx, s.ph(
		`SELECT body FROM keramos_releases WHERE namespace = ? AND name = ? ORDER BY revision ASC`),
		s.namespace, name)
	if nil != err {
		return nil, keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to list history for %s", name)
	}
	defer rows.Close()

	out := make([]*Release, 0)
	for rows.Next() {
		var body string
		if scanErr := rows.Scan(&body); nil != scanErr {
			return nil, keramoserr.WrapError(keramoserr.ErrRelease, "failed to scan history row", scanErr)
		}
		rel, decErr := Decode(body)
		if nil != decErr {
			continue
		}
		out = append(out, rel)
	}
	if rowsErr := rows.Err(); nil != rowsErr {
		return nil, keramoserr.WrapError(keramoserr.ErrRelease, "row iteration failed", rowsErr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Revision < out[j].Revision })
	return out, nil
}

func (s *SQLStorage) Delete(name string, revision int) error {
	ctx, cancel := newSQLContext()
	defer cancel()
	if _, err := s.db.ExecContext(ctx, s.ph(
		`DELETE FROM keramos_releases WHERE namespace = ? AND name = ? AND revision = ?`),
		s.namespace, name, revision); nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrRelease, err, "failed to delete release %s v%d", name, revision)
	}
	return nil
}
