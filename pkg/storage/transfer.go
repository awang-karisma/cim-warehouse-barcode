package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const sqliteTransferMarker = "sqlite_to_postgres_v1"
const postgresTransferLock int64 = 748392610135

// importSQLiteIfEligible imports the application schema discovered from PostgreSQL.
// Only golang-migrate's bookkeeping table is excluded; application tables and future
// migration-created tables are copied without a second Go-side schema manifest.
func importSQLiteIfEligible(ctx context.Context, target *sql.DB, sqlitePath string) error {
	if sqlitePath == "" || sqlitePath == ":memory:" || strings.Contains(sqlitePath, "mode=memory") {
		return nil
	}
	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect SQLite transfer source: %w", err)
	}
	conn, err := target.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, postgresTransferLock); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, postgresTransferLock)

	var marker int
	if err = conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM migration_markers WHERE name=$1`, sqliteTransferMarker).Scan(&marker); err != nil {
		return err
	}
	if marker != 0 {
		return nil
	}
	tables, err := postgresApplicationTables(ctx, conn)
	if err != nil {
		return err
	}
	tables, err = postgresDependencyOrder(ctx, conn, tables)
	if err != nil {
		return err
	}
	if populated, err := postgresHasBusinessData(ctx, conn, tables); err != nil || populated {
		return err
	}

	sourcePath := sqlitePath
	if !strings.HasPrefix(sourcePath, "file:") {
		sourcePath = "file:" + filepath.ToSlash(sourcePath) + "?mode=ro"
	}
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		return fmt.Errorf("open SQLite transfer source: %w", err)
	}
	defer source.Close()
	if err = source.PingContext(ctx); err != nil {
		return fmt.Errorf("connect SQLite transfer source: %w", err)
	}

	columnsByTable := make(map[string][]string, len(tables))
	for _, table := range tables {
		columns, err := postgresTableColumns(ctx, conn, table)
		if err != nil {
			return err
		}
		columnsByTable[table] = columns
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `TRUNCATE TABLE `+quoteIdentifiers(tables)+` RESTART IDENTITY CASCADE`); err != nil {
		return err
	}
	for _, table := range tables {
		if err = copySQLiteTable(ctx, tx, source, table, columnsByTable[table]); err != nil {
			return fmt.Errorf("copy %s: %w", table, err)
		}
	}
	for _, table := range tables {
		if !containsColumn(columnsByTable[table], "id") {
			continue
		}
		var serial sql.NullString
		if err = tx.QueryRowContext(ctx, `SELECT pg_get_serial_sequence($1, 'id')`, table).Scan(&serial); err != nil {
			return fmt.Errorf("find sequence for %s: %w", table, err)
		}
		if !serial.Valid {
			continue
		}
		if _, err = tx.ExecContext(ctx, `SELECT setval($1, COALESCE((SELECT MAX(id) FROM `+quoteIdentifier(table)+`), 1), EXISTS(SELECT 1 FROM `+quoteIdentifier(table)+`))`, serial.String); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO migration_markers(name) VALUES($1)`, sqliteTransferMarker); err != nil {
		return err
	}
	return tx.Commit()
}

func postgresApplicationTables(ctx context.Context, conn *sql.Conn) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema=current_schema() AND table_type='BASE TABLE' AND table_name <> 'schema_migrations' ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func postgresDependencyOrder(ctx context.Context, conn *sql.Conn, tables []string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT child.relname, parent.relname FROM pg_constraint c JOIN pg_class child ON child.oid=c.conrelid JOIN pg_class parent ON parent.oid=c.confrelid WHERE c.contype='f' AND child.relnamespace=current_schema()::regnamespace`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	known := make(map[string]bool, len(tables))
	indegree := make(map[string]int, len(tables))
	children := make(map[string][]string, len(tables))
	for _, table := range tables {
		known[table] = true
	}
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, err
		}
		if known[child] && known[parent] {
			indegree[child]++
			children[parent] = append(children[parent], child)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ready := make([]string, 0, len(tables))
	for _, table := range tables {
		if indegree[table] == 0 {
			ready = append(ready, table)
		}
	}
	sort.Strings(ready)
	ordered := make([]string, 0, len(tables))
	for len(ready) != 0 {
		table := ready[0]
		ready = ready[1:]
		ordered = append(ordered, table)
		for _, child := range children[table] {
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
			}
		}
		sort.Strings(ready)
	}
	if len(ordered) != len(tables) {
		return nil, fmt.Errorf("application schema has cyclic foreign-key dependencies")
	}
	return ordered, nil
}

func postgresHasBusinessData(ctx context.Context, conn *sql.Conn, tables []string) (bool, error) {
	for _, table := range tables {
		if table == "roles" || table == "role_permissions" || table == "migration_markers" {
			continue
		}
		var count int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteIdentifier(table)).Scan(&count); err != nil {
			return false, err
		}
		if count != 0 {
			return true, nil
		}
	}
	var customRoles int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM roles WHERE name NOT IN ('guest','user','admin','superadmin')`).Scan(&customRoles); err != nil {
		return false, err
	}
	var permissionCount int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM role_permissions`).Scan(&permissionCount); err != nil {
		return false, err
	}
	return customRoles != 0 || permissionCount > 32, nil
}

func postgresTableColumns(ctx context.Context, conn *sql.Conn, table string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 ORDER BY ordinal_position`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func copySQLiteTable(ctx context.Context, target *sql.Tx, source *sql.DB, table string, targetColumns []string) error {
	sourceColumns, err := sqliteTableColumns(ctx, source, table)
	if err != nil {
		return err
	}
	if len(sourceColumns) == 0 {
		return nil
	}
	available := make(map[string]bool, len(sourceColumns))
	for _, column := range sourceColumns {
		available[column] = true
	}
	columns := make([]string, 0, len(targetColumns))
	for _, column := range targetColumns {
		if available[column] {
			columns = append(columns, column)
		}
	}
	if len(columns) == 0 {
		return nil
	}
	rows, err := source.QueryContext(ctx, `SELECT `+quoteIdentifiers(columns)+` FROM `+quoteIdentifier(table))
	if err != nil {
		return err
	}
	defer rows.Close()
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	statement := `INSERT INTO ` + quoteIdentifier(table) + ` (` + quoteIdentifiers(columns) + `) VALUES (` + strings.Join(placeholders, ",") + `)`
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err = rows.Scan(destinations...); err != nil {
			return err
		}
		if _, err = target.ExecContext(ctx, statement, values...); err != nil {
			return err
		}
	}
	return rows.Err()
}

func sqliteTableColumns(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+quoteIdentifier(table)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	sort.Strings(columns)
	return columns, rows.Err()
}

func containsColumn(columns []string, wanted string) bool {
	for _, column := range columns {
		if column == wanted {
			return true
		}
	}
	return false
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
func quoteIdentifiers(identifiers []string) string {
	quoted := make([]string, len(identifiers))
	for i, identifier := range identifiers {
		quoted[i] = quoteIdentifier(identifier)
	}
	return strings.Join(quoted, ",")
}
