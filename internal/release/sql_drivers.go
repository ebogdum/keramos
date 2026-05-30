package release

// Blank imports register the supported database/sql drivers so the SQL storage
// backend (KERAMOS_DRIVER=sql) actually works in the shipped binary. All three are
// pure-Go (CGO-free), preserving cross-compilation:
//
//   - "postgres" -> github.com/lib/pq
//   - "mysql"    -> github.com/go-sql-driver/mysql
//   - "sqlite"   -> modernc.org/sqlite
import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)
