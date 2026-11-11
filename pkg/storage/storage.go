package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/awang-karisma/cim-warehouse-barcode/pkg/accurate"
	"github.com/awang-karisma/cim-warehouse-barcode/pkg/model"
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when an entity is not found in storage.
var ErrNotFound = errors.New("not found")

// ErrRoleInUse is returned when attempting to delete a role assigned to users.
var ErrRoleInUse = errors.New("role is currently assigned to users")

// ErrSystemRole is returned when attempting to delete or rename a system role.
var ErrSystemRole = errors.New("cannot modify or delete system role")

// ErrInvalidPermission is returned when an unrecognized permission string is provided.
var ErrInvalidPermission = errors.New("invalid permission")
// TableStat represents table row count metrics.
type TableStat struct {
	RowCount int `json:"row_count"`
}

// Storage provides SQLite persistence for barcode layouts, user accounts, and Accurate session state.
type Storage struct {
	db *sql.DB
}

// New opens a SQLite database at dbPath with WAL mode and busy timeout pragmas,
// and automatically migrates the required schema.
func New(dbPath string) (*Storage, error) {
	dsn := dbPath
	if !strings.Contains(dbPath, "_pragma=journal_mode") && !strings.Contains(dbPath, "_pragma=busy_timeout") {
		sep := "?"
		if strings.Contains(dbPath, "?") {
			sep = "&"
		}
		dsn = fmt.Sprintf("%s%s_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath, sep)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// For in-memory databases, ensure all queries reuse the same connection instance
	if strings.HasPrefix(dbPath, ":memory:") || strings.Contains(dbPath, "mode=memory") {
		db.SetMaxOpenConns(1)
	}

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to migrate tables: %w", err)
	}

	return s, nil
}

// DB returns the underlying sql.DB connection.
func (s *Storage) DB() *sql.DB {
	return s.db
}

// Close closes the database handle.
func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Storage) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE COLLATE NOCASE,
		name TEXT NOT NULL DEFAULT '',
		picture TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT 'guest',
		locale TEXT NOT NULL DEFAULT 'id',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

	CREATE TABLE IF NOT EXISTS accurate_session (
		id               INTEGER PRIMARY KEY CHECK (id = 1),
		provider         TEXT NOT NULL DEFAULT 'web',
		email            TEXT NOT NULL DEFAULT '',
		device_id        TEXT NOT NULL DEFAULT '',
		cookies          TEXT NOT NULL DEFAULT '[]',
		tenant           TEXT NOT NULL DEFAULT '',
		official_session TEXT NOT NULL DEFAULT ''
	);
	INSERT OR IGNORE INTO accurate_session (id) VALUES (1);

	CREATE TABLE IF NOT EXISTS page_designs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		paper TEXT NOT NULL DEFAULT 'A4',
		paper_width_mm REAL NOT NULL DEFAULT 210,
		paper_height_mm REAL NOT NULL DEFAULT 297,
		label_width_mm REAL NOT NULL,
		label_height_mm REAL NOT NULL,
		cols INTEGER NOT NULL DEFAULT 1,
		rows INTEGER NOT NULL DEFAULT 1,
		gap_x_mm REAL NOT NULL DEFAULT 0,
		gap_y_mm REAL NOT NULL DEFAULT 0,
		margin_top_mm REAL NOT NULL DEFAULT 0,
		margin_left_mm REAL NOT NULL DEFAULT 0,
		is_audit_sheet INTEGER NOT NULL DEFAULT 0,
		label_layout TEXT DEFAULT NULL,
		settings_json TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS label_layout_presets (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE COLLATE NOCASE,
		label_layout TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS roles (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE COLLATE NOCASE,
		description TEXT NOT NULL DEFAULT '',
		is_system   INTEGER NOT NULL DEFAULT 0,
		created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
		updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS role_permissions (
		role_id    INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
		permission TEXT NOT NULL,
		PRIMARY KEY (role_id, permission)
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	_, _ = s.db.Exec(`ALTER TABLE accurate_session ADD COLUMN provider TEXT NOT NULL DEFAULT 'web';`)
	_, _ = s.db.Exec(`ALTER TABLE accurate_session ADD COLUMN official_session TEXT NOT NULL DEFAULT '';`)
	_, _ = s.db.Exec(`ALTER TABLE page_designs ADD COLUMN label_layout TEXT DEFAULT NULL;`)
	_, _ = s.db.Exec(`ALTER TABLE page_designs ADD COLUMN settings_json TEXT NOT NULL DEFAULT '{}';`)
	_, _ = s.db.Exec(`ALTER TABLE users ADD COLUMN locale TEXT NOT NULL DEFAULT 'id';`)


	// Seed system roles (idempotent)
	systemRoles := []struct{ name, desc string }{
		{"guest", "Newly registered, pending approval"},
		{"user", "Standard warehouse operator"},
		{"admin", "Full access including user and role management"},
		{"superadmin", "Unrestricted system access (env-configured)"},
	}
	for _, r := range systemRoles {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO roles (name, description, is_system) VALUES (?, ?, 1)`, r.name, r.desc); err != nil {
			return fmt.Errorf("failed to seed system role %s: %w", r.name, err)
		}
	}

	// Seed default permissions for user and admin roles
	userPerms := []string{
		string(model.PermDesignRead),
		string(model.PermPresetRead),
		string(model.PermAccurateRead),
		string(model.PermAccurateCreate),
		string(model.PermAccurateUpdate),
		string(model.PermAccurateDelete),
		string(model.PermSystemRead),
	}
	adminPerms := make([]string, len(model.AllPermissions))
	for i, p := range model.AllPermissions {
		adminPerms[i] = string(p)
	}

	rolePermSeeds := []struct {
		roleName string
		perms    []string
	}{
		{"user", userPerms},
		{"admin", adminPerms},
	}

	for _, seed := range rolePermSeeds {
		var roleID int64
		err := s.db.QueryRow(`SELECT id FROM roles WHERE name = ? COLLATE NOCASE`, seed.roleName).Scan(&roleID)
		if err != nil {
			return fmt.Errorf("failed to query role id for %s: %w", seed.roleName, err)
		}

		var count int
		err = s.db.QueryRow(`SELECT COUNT(*) FROM role_permissions WHERE role_id = ?`, roleID).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check role permissions for %s: %w", seed.roleName, err)
		}

		if count == 0 {
			tx, err := s.db.Begin()
			if err != nil {
				return fmt.Errorf("failed to start tx for seeding role permissions: %w", err)
			}
			stmt, err := tx.Prepare(`INSERT OR IGNORE INTO role_permissions (role_id, permission) VALUES (?, ?)`)
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to prepare role permission insert: %w", err)
			}
			for _, p := range seed.perms {
				if _, err := stmt.Exec(roleID, p); err != nil {
					_ = stmt.Close()
					_ = tx.Rollback()
					return fmt.Errorf("failed to insert role permission %s for %s: %w", p, seed.roleName, err)
				}
			}
			_ = stmt.Close()
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit role permissions for %s: %w", seed.roleName, err)
			}
		}
	}
	return nil
}

// GetTableStats returns row counts for users, accurate_session, page_designs, label_layout_presets, roles, and role_permissions tables.
func (s *Storage) GetTableStats(ctx context.Context) (map[string]TableStat, error) {
	stats := make(map[string]TableStat)
	tables := []string{"users", "accurate_session", "page_designs", "label_layout_presets", "roles", "role_permissions"}
	for _, tbl := range tables {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tbl)
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return nil, fmt.Errorf("failed to count %s: %w", tbl, err)
		}
		stats[tbl] = TableStat{RowCount: count}
	}
	return stats, nil
}

// UpsertUser inserts a new user with 'guest' role or updates name/picture/updated_at if existing.
func (s *Storage) UpsertUser(ctx context.Context, email, name, picture string) (*model.User, error) {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return nil, fmt.Errorf("email cannot be empty")
	}
	trimmedName := strings.TrimSpace(name)
	trimmedPicture := strings.TrimSpace(picture)
	now := time.Now().UTC()

	query := `
		INSERT INTO users (email, name, picture, role, created_at, updated_at)
		VALUES (?, ?, ?, 'guest', ?, ?)
		ON CONFLICT(email) DO UPDATE SET
			name = excluded.name,
			picture = excluded.picture,
			updated_at = excluded.updated_at
	`
	if _, err := s.db.ExecContext(ctx, query, trimmedEmail, trimmedName, trimmedPicture, now, now); err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}
	return s.GetUserByEmail(ctx, trimmedEmail)
}

// GetUserByEmail retrieves a user by email case-insensitively, returning nil, nil if not found.
func (s *Storage) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return nil, nil
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, email, name, picture, role, locale, created_at, updated_at
		FROM users
		WHERE email = ? COLLATE NOCASE
	`, trimmedEmail)

	var u model.User
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Picture, &u.Role, &u.Locale, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &u, nil
}

// GetUsers returns all users ordered by created_at DESC.
func (s *Storage) GetUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, name, picture, role, locale, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Picture, &u.Role, &u.Locale, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate users: %w", err)
	}
	if users == nil {
		users = []model.User{}
	}
	return users, nil
}

// SetUserRole updates a user's role and updated_at.
// Reject if role is superadmin or does not exist in roles table.
func (s *Storage) SetUserRole(ctx context.Context, email, role string) error {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return fmt.Errorf("email cannot be empty")
	}
	trimmedRole := strings.TrimSpace(role)
	if trimmedRole == "" {
		return fmt.Errorf("role cannot be empty")
	}
	if strings.EqualFold(trimmedRole, string(model.RoleSuperadmin)) {
		return fmt.Errorf("cannot assign superadmin role: superadmin is configured via environment only")
	}
	exists, err := s.RoleExists(ctx, trimmedRole)
	if err != nil {
		return fmt.Errorf("failed to check role existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("role %q does not exist", role)
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `UPDATE users SET role = ?, updated_at = ? WHERE email = ? COLLATE NOCASE`, trimmedRole, now, trimmedEmail)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserLocale updates a user's preferred locale and updated_at.
func (s *Storage) UpdateUserLocale(ctx context.Context, email, locale string) error {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return fmt.Errorf("email cannot be empty")
	}
	trimmedLocale := strings.ToLower(strings.TrimSpace(locale))
	if model.Locale(trimmedLocale) != model.LocaleEnglish && model.Locale(trimmedLocale) != model.LocaleIndonesian {
		return fmt.Errorf("invalid locale %q: must be en or id", locale)
	}
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `UPDATE users SET locale = ?, updated_at = ? WHERE email = ? COLLATE NOCASE`, trimmedLocale, now, trimmedEmail)
	if err != nil {
		return fmt.Errorf("failed to update user locale: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser deletes a user by email case-insensitively.
func (s *Storage) DeleteUser(ctx context.Context, email string) error {
	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		return fmt.Errorf("email cannot be empty")
	}
	query := `DELETE FROM users WHERE email = ? COLLATE NOCASE`
	if _, err := s.db.ExecContext(ctx, query, trimmedEmail); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// SaveAccurateSession serializes session state for either provider to JSON and updates the singleton row.
func (s *Storage) SaveAccurateSession(providerID, email, deviceID string, jar *accurate.MemoryJar, tenant *accurate.TenantSession, official *accurate.OfficialSessionState) error {
	cookiesJSON := "[]"
	if jar != nil {
		allCookies := jar.AllCookies()
		data, err := json.Marshal(allCookies)
		if err != nil {
			return err
		}
		cookiesJSON = string(data)
	}

	tenantJSON := ""
	if tenant != nil {
		data, err := json.Marshal(tenant)
		if err != nil {
			return err
		}
		tenantJSON = string(data)
	}

	officialJSON := ""
	if official != nil {
		data, err := json.Marshal(official)
		if err != nil {
			return err
		}
		officialJSON = string(data)
	}

	if providerID == "" {
		providerID = "web"
	}

	_, err := s.db.Exec(
		`UPDATE accurate_session SET provider = ?, email = ?, device_id = ?, cookies = ?, tenant = ?, official_session = ? WHERE id = 1`,
		providerID, email, deviceID, cookiesJSON, tenantJSON, officialJSON,
	)
	return err
}

// LoadAccurateSession reads the singleton session row and deserializes provider ID and connection state.
func (s *Storage) LoadAccurateSession() (providerID, email, deviceID string, cookies []*accurate.Cookie, tenant *accurate.TenantSession, official *accurate.OfficialSessionState, err error) {
	var cookiesJSON, tenantJSON, officialJSON string
	row := s.db.QueryRow(`SELECT provider, email, device_id, cookies, tenant, official_session FROM accurate_session WHERE id = 1`)
	if err := row.Scan(&providerID, &email, &deviceID, &cookiesJSON, &tenantJSON, &officialJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "web", "", "", nil, nil, nil, nil
		}
		return "web", "", "", nil, nil, nil, err
	}

	if cookiesJSON != "" && cookiesJSON != "[]" {
		if err := json.Unmarshal([]byte(cookiesJSON), &cookies); err != nil {
			return providerID, email, deviceID, nil, nil, nil, fmt.Errorf("failed to decode cookies: %w", err)
		}
	}

	if tenantJSON != "" {
		tenant = &accurate.TenantSession{}
		if err := json.Unmarshal([]byte(tenantJSON), tenant); err != nil {
			return providerID, email, deviceID, nil, nil, nil, fmt.Errorf("failed to decode tenant session: %w", err)
		}
	}

	if officialJSON != "" {
		official = &accurate.OfficialSessionState{}
		if err := json.Unmarshal([]byte(officialJSON), official); err != nil {
			return providerID, email, deviceID, nil, nil, nil, fmt.Errorf("failed to decode official session: %w", err)
		}
	}

	return providerID, email, deviceID, cookies, tenant, official, nil
}

// ClearAccurateSession clears all stored session data while preserving the singleton row.
func (s *Storage) ClearAccurateSession() error {
	_, err := s.db.Exec(`UPDATE accurate_session SET email = '', device_id = '', cookies = '[]', tenant = '', official_session = '' WHERE id = 1`)
	return err
}

// GetLabelLayoutPresets lists reusable label element layouts by name.
func (s *Storage) GetLabelLayoutPresets(ctx context.Context) ([]model.LabelLayoutPreset, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, label_layout, created_at, updated_at
		FROM label_layout_presets
		ORDER BY name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query label layout presets: %w", err)
	}
	defer rows.Close()

	presets := make([]model.LabelLayoutPreset, 0)
	for rows.Next() {
		var preset model.LabelLayoutPreset
		if err := rows.Scan(&preset.ID, &preset.Name, &preset.LabelLayout, &preset.CreatedAt, &preset.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan label layout preset: %w", err)
		}
		presets = append(presets, preset)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate label layout presets: %w", err)
	}
	return presets, nil
}

// CreateLabelLayoutPreset creates a reusable label element arrangement.
func (s *Storage) CreateLabelLayoutPreset(ctx context.Context, preset model.LabelLayoutPreset) (int64, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO label_layout_presets (name, label_layout) VALUES (?, ?)`,
		preset.Name, preset.LabelLayout,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert label layout preset: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get label layout preset ID: %w", err)
	}
	return id, nil
}

// UpdateLabelLayoutPreset updates an existing reusable label element arrangement.
func (s *Storage) UpdateLabelLayoutPreset(ctx context.Context, id int64, preset model.LabelLayoutPreset) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE label_layout_presets
		SET name = ?, label_layout = ?, updated_at = datetime('now')
		WHERE id = ?
	`, preset.Name, preset.LabelLayout, id)
	if err != nil {
		return fmt.Errorf("failed to update label layout preset %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get label layout preset rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("label layout preset %d: %w", id, ErrNotFound)
	}
	return nil
}

// DeleteLabelLayoutPreset deletes a reusable label element arrangement.
func (s *Storage) DeleteLabelLayoutPreset(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM label_layout_presets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete label layout preset %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get label layout preset rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("label layout preset %d: %w", id, ErrNotFound)
	}
	return nil
}

func normalizedBarcodePresetSettings(settings *model.BarcodePresetSettings) model.BarcodePresetSettings {
	defaults := model.DefaultBarcodePresetSettings()
	if settings == nil {
		return defaults
	}
	normalized := *settings
	if normalized.Symbology == "" {
		normalized.Symbology = defaults.Symbology
	}
	if normalized.BarcodeHeight == "" {
		normalized.BarcodeHeight = defaults.BarcodeHeight
	}
	if normalized.FontSize == "" {
		normalized.FontSize = defaults.FontSize
	}
	if normalized.QtyMode == "" {
		normalized.QtyMode = defaults.QtyMode
	}
	if normalized.SkipCount == nil {
		normalized.SkipCount = defaults.SkipCount
	}
	if normalized.ShowScreenBorder == nil {
		normalized.ShowScreenBorder = defaults.ShowScreenBorder
	}
	if normalized.Zoom == nil {
		normalized.Zoom = defaults.Zoom
	}
	return normalized
}

func decodeBarcodePresetSettings(raw sql.NullString) (*model.BarcodePresetSettings, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" || strings.TrimSpace(raw.String) == "{}" {
		settings := normalizedBarcodePresetSettings(nil)
		return &settings, nil
	}
	var settings model.BarcodePresetSettings
	if err := json.Unmarshal([]byte(raw.String), &settings); err != nil {
		return nil, fmt.Errorf("failed to decode page design settings: %w", err)
	}
	normalized := normalizedBarcodePresetSettings(&settings)
	return &normalized, nil
}

func encodeBarcodePresetSettings(settings *model.BarcodePresetSettings) (string, error) {
	normalized := normalizedBarcodePresetSettings(settings)
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to encode page design settings: %w", err)
	}
	return string(encoded), nil
}

// GetPageDesigns returns all custom page designs ordered alphabetically by name.
func (s *Storage) GetPageDesigns(ctx context.Context) ([]model.PageDesign, error) {
	query := `
        SELECT id, name, paper, paper_width_mm, paper_height_mm,
               label_width_mm, label_height_mm, cols, rows,
               gap_x_mm, gap_y_mm, margin_top_mm, margin_left_mm,
               is_audit_sheet, label_layout, settings_json, created_at, updated_at
        FROM page_designs
        ORDER BY name COLLATE NOCASE
    `
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query page designs: %w", err)
	}
	defer rows.Close()

	var designs []model.PageDesign
	for rows.Next() {
		var d model.PageDesign
		var isAuditSheetInt int
		var labelLayout sql.NullString
		var settingsJSON sql.NullString
		if err := rows.Scan(
			&d.ID, &d.Name, &d.Paper, &d.PaperWidthMM, &d.PaperHeightMM,
			&d.LabelWidthMM, &d.LabelHeightMM, &d.Cols, &d.Rows,
			&d.GapXMM, &d.GapYMM, &d.MarginTopMM, &d.MarginLeftMM,
			&isAuditSheetInt, &labelLayout, &settingsJSON, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan page design: %w", err)
		}
		d.IsAuditSheet = isAuditSheetInt == 1
		d.Settings, err = decodeBarcodePresetSettings(settingsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode page design %d settings: %w", d.ID, err)
		}
		if labelLayout.Valid {
			d.LabelLayout = &labelLayout.String
		}
		designs = append(designs, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating page designs: %w", err)
	}
	if designs == nil {
		designs = []model.PageDesign{}
	}
	return designs, nil
}

// GetPageDesignByID returns a single page design by ID, or ErrNotFound if it does not exist.
func (s *Storage) GetPageDesignByID(ctx context.Context, id int64) (*model.PageDesign, error) {
	query := `
        SELECT id, name, paper, paper_width_mm, paper_height_mm,
               label_width_mm, label_height_mm, cols, rows,
               gap_x_mm, gap_y_mm, margin_top_mm, margin_left_mm,
               is_audit_sheet, label_layout, settings_json, created_at, updated_at
        FROM page_designs
        WHERE id = ?
    `
	row := s.db.QueryRowContext(ctx, query, id)
	var d model.PageDesign
	var isAuditSheetInt int
	var labelLayout sql.NullString
	var settingsJSON sql.NullString
	if err := row.Scan(
		&d.ID, &d.Name, &d.Paper, &d.PaperWidthMM, &d.PaperHeightMM,
		&d.LabelWidthMM, &d.LabelHeightMM, &d.Cols, &d.Rows,
		&d.GapXMM, &d.GapYMM, &d.MarginTopMM, &d.MarginLeftMM,
		&isAuditSheetInt, &labelLayout, &settingsJSON, &d.CreatedAt, &d.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("page design %d: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("failed to scan page design %d: %w", id, err)
	}
	d.IsAuditSheet = isAuditSheetInt == 1
	settings, err := decodeBarcodePresetSettings(settingsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode page design %d settings: %w", d.ID, err)
	}
	d.Settings = settings
	if labelLayout.Valid {
		d.LabelLayout = &labelLayout.String
	}
	return &d, nil
}

// CreatePageDesign inserts a new page design into storage and returns its generated ID.
func (s *Storage) CreatePageDesign(ctx context.Context, d model.PageDesign) (int64, error) {
	query := `
        INSERT INTO page_designs (
            name, paper, paper_width_mm, paper_height_mm,
            label_width_mm, label_height_mm, cols, rows,
            gap_x_mm, gap_y_mm, margin_top_mm, margin_left_mm,
            is_audit_sheet, label_layout, settings_json
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	isAuditSheetInt := 0
	if d.IsAuditSheet {
		isAuditSheetInt = 1
	}
	settingsJSON, err := encodeBarcodePresetSettings(d.Settings)
	if err != nil {
		return 0, err
	}
	res, err := s.db.ExecContext(ctx, query,
		d.Name, d.Paper, d.PaperWidthMM, d.PaperHeightMM,
		d.LabelWidthMM, d.LabelHeightMM, d.Cols, d.Rows,
		d.GapXMM, d.GapYMM, d.MarginTopMM, d.MarginLeftMM,
		isAuditSheetInt, d.LabelLayout, settingsJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert page design: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get page design insert id: %w", err)
	}
	return id, nil
}

// UpdatePageDesign updates an existing page design by ID. Returns ErrNotFound if the row does not exist.
func (s *Storage) UpdatePageDesign(ctx context.Context, id int64, d model.PageDesign) error {
	query := `
        UPDATE page_designs SET
            name = ?, paper = ?, paper_width_mm = ?, paper_height_mm = ?,
            label_width_mm = ?, label_height_mm = ?, cols = ?, rows = ?,
            gap_x_mm = ?, gap_y_mm = ?, margin_top_mm = ?, margin_left_mm = ?,
            is_audit_sheet = ?, label_layout = ?, settings_json = ?, updated_at = datetime('now')
        WHERE id = ?
    `
	isAuditSheetInt := 0
	if d.IsAuditSheet {
		isAuditSheetInt = 1
	}
	settingsJSON, err := encodeBarcodePresetSettings(d.Settings)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, query,
		d.Name, d.Paper, d.PaperWidthMM, d.PaperHeightMM,
		d.LabelWidthMM, d.LabelHeightMM, d.Cols, d.Rows,
		d.GapXMM, d.GapYMM, d.MarginTopMM, d.MarginLeftMM,
		isAuditSheetInt, d.LabelLayout, settingsJSON, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update page design %d: %w", id, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("page design %d: %w", id, ErrNotFound)
	}
	return nil
}

// DeletePageDesign deletes a page design by ID. Returns ErrNotFound if the row does not exist.
func (s *Storage) DeletePageDesign(ctx context.Context, id int64) error {
	query := `DELETE FROM page_designs WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete page design %d: %w", id, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("page design %d: %w", id, ErrNotFound)
	}
	return nil
}

// RoleExists reports whether a role with the given name exists (case-insensitive).
func (s *Storage) RoleExists(ctx context.Context, name string) (bool, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false, nil
	}
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM roles WHERE name = ? COLLATE NOCASE LIMIT 1`, trimmed).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check role existence: %w", err)
	}
	return true, nil
}

// CountUsersWithRole returns how many users are assigned to the specified role name.
func (s *Storage) CountUsersWithRole(ctx context.Context, roleName string) (int, error) {
	trimmed := strings.TrimSpace(roleName)
	if trimmed == "" {
		return 0, nil
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ? COLLATE NOCASE`, trimmed).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users with role %s: %w", roleName, err)
	}
	return count, nil
}

// GetRoles returns all roles ordered by ID ascending, with their permissions populated.
func (s *Storage) GetRoles(ctx context.Context) ([]model.Role, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, is_system, created_at, updated_at FROM roles ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query roles: %w", err)
	}
	defer rows.Close()

	var roles []model.Role
	for rows.Next() {
		var r model.Role
		var isSystem int
		if err := rows.Scan(&r.ID, &r.Name, &r.Description, &isSystem, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan role: %w", err)
		}
		r.IsSystem = isSystem != 0
		r.Permissions = []string{}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate roles: %w", err)
	}
	if roles == nil {
		return []model.Role{}, nil
	}

	pRows, err := s.db.QueryContext(ctx, `SELECT role_id, permission FROM role_permissions ORDER BY permission ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query role permissions: %w", err)
	}
	defer pRows.Close()

	permsByRole := make(map[int64][]string)
	for pRows.Next() {
		var rID int64
		var perm string
		if err := pRows.Scan(&rID, &perm); err != nil {
			return nil, fmt.Errorf("failed to scan role permission: %w", err)
		}
		permsByRole[rID] = append(permsByRole[rID], perm)
	}
	if err := pRows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate role permissions: %w", err)
	}

	for i := range roles {
		if perms, ok := permsByRole[roles[i].ID]; ok {
			roles[i].Permissions = perms
		}
	}

	return roles, nil
}

// GetRoleByID returns a role by its primary key ID, or ErrNotFound.
func (s *Storage) GetRoleByID(ctx context.Context, id int64) (*model.Role, error) {
	var r model.Role
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, is_system, created_at, updated_at FROM roles WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Description, &isSystem, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query role by id %d: %w", id, err)
	}
	r.IsSystem = isSystem != 0
	perms, err := s.GetRolePermissions(ctx, id)
	if err != nil {
		return nil, err
	}
	r.Permissions = perms
	return &r, nil
}

// GetRoleByName returns a role by its name (case-insensitive), or ErrNotFound.
func (s *Storage) GetRoleByName(ctx context.Context, name string) (*model.Role, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, ErrNotFound
	}
	var r model.Role
	var isSystem int
	err := s.db.QueryRowContext(ctx, `SELECT id, name, description, is_system, created_at, updated_at FROM roles WHERE name = ? COLLATE NOCASE`, trimmed).
		Scan(&r.ID, &r.Name, &r.Description, &isSystem, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to query role by name %s: %w", trimmed, err)
	}
	r.IsSystem = isSystem != 0
	perms, err := s.GetRolePermissions(ctx, r.ID)
	if err != nil {
		return nil, err
	}
	r.Permissions = perms
	return &r, nil
}

// CreateRole creates a new custom role (is_system = 0).
func (s *Storage) CreateRole(ctx context.Context, name, description string) (*model.Role, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("role name cannot be empty")
	}
	systemNames := []string{
		string(model.RoleGuest),
		string(model.RoleUser),
		string(model.RoleAdmin),
		string(model.RoleSuperadmin),
	}
	for _, sn := range systemNames {
		if strings.EqualFold(trimmedName, sn) {
			return nil, fmt.Errorf("role name %q conflicts with system role name", trimmedName)
		}
	}
	exists, err := s.RoleExists(ctx, trimmedName)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("role %q already exists", trimmedName)
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `INSERT INTO roles (name, description, is_system, created_at, updated_at) VALUES (?, ?, 0, ?, ?)`, trimmedName, strings.TrimSpace(description), now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert role: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return s.GetRoleByID(ctx, id)
}

// UpdateRole updates a role's name and description. System roles can only have description updated.
func (s *Storage) UpdateRole(ctx context.Context, id int64, name, description string) error {
	r, err := s.GetRoleByID(ctx, id)
	if err != nil {
		return err
	}
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return fmt.Errorf("role name cannot be empty")
	}
	trimmedDesc := strings.TrimSpace(description)

	if r.IsSystem {
		if !strings.EqualFold(r.Name, trimmedName) {
			return fmt.Errorf("cannot rename system role %q: %w", r.Name, ErrSystemRole)
		}
		now := time.Now().UTC()
		_, err := s.db.ExecContext(ctx, `UPDATE roles SET description = ?, updated_at = ? WHERE id = ?`, trimmedDesc, now, id)
		if err != nil {
			return fmt.Errorf("failed to update system role description: %w", err)
		}
		return nil
	}

	if !strings.EqualFold(r.Name, trimmedName) {
		systemNames := []string{
			string(model.RoleGuest),
			string(model.RoleUser),
			string(model.RoleAdmin),
			string(model.RoleSuperadmin),
		}
		for _, sn := range systemNames {
			if strings.EqualFold(trimmedName, sn) {
				return fmt.Errorf("role name %q conflicts with system role name", trimmedName)
			}
		}
		exists, err := s.RoleExists(ctx, trimmedName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("role %q already exists", trimmedName)
		}
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `UPDATE roles SET name = ?, description = ?, updated_at = ? WHERE id = ?`, trimmedName, trimmedDesc, now, id)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}
	return nil
}

// DeleteRole deletes a custom role by ID. Returns error if role is system role or assigned to users.
func (s *Storage) DeleteRole(ctx context.Context, id int64) error {
	r, err := s.GetRoleByID(ctx, id)
	if err != nil {
		return err
	}
	if r.IsSystem {
		return fmt.Errorf("cannot delete system role %q: %w", r.Name, ErrSystemRole)
	}
	userCount, err := s.CountUsersWithRole(ctx, r.Name)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return fmt.Errorf("%d user(s) currently assigned to role %q: %w", userCount, r.Name, ErrRoleInUse)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM roles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// GetRolePermissions returns all permissions configured for a role ID.
func (s *Storage) GetRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT permission FROM role_permissions WHERE role_id = ? ORDER BY permission ASC`, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query role permissions for role %d: %w", roleID, err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate permissions: %w", err)
	}
	if perms == nil {
		perms = []string{}
	}
	return perms, nil
}

// SetRolePermissions sets the permissions for a role, replacing existing ones.
// Rejects modification of superadmin and guest roles, and validates permission names against the registry.
func (s *Storage) SetRolePermissions(ctx context.Context, roleID int64, permissions []string) error {
	r, err := s.GetRoleByID(ctx, roleID)
	if err != nil {
		return err
	}
	if strings.EqualFold(r.Name, string(model.RoleSuperadmin)) {
		return fmt.Errorf("superadmin permissions cannot be modified: superadmin bypasses all checks")
	}
	if strings.EqualFold(r.Name, string(model.RoleGuest)) {
		return fmt.Errorf("guest permissions cannot be modified: guest has no permissions")
	}

	seen := make(map[string]bool)
	var deduped []string
	for _, p := range permissions {
		trimmed := strings.TrimSpace(p)
		if !model.ValidPermission(trimmed) {
			return fmt.Errorf("invalid permission %q: %w", p, ErrInvalidPermission)
		}
		if !seen[trimmed] {
			seen[trimmed] = true
			deduped = append(deduped, trimmed)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id = ?`, roleID); err != nil {
		return fmt.Errorf("failed to delete existing permissions: %w", err)
	}

	if len(deduped) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO role_permissions (role_id, permission) VALUES (?, ?)`)
		if err != nil {
			return fmt.Errorf("failed to prepare permission insert: %w", err)
		}
		defer stmt.Close()

		for _, p := range deduped {
			if _, err := stmt.ExecContext(ctx, roleID, p); err != nil {
				return fmt.Errorf("failed to insert permission %s: %w", p, err)
			}
		}
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE roles SET updated_at = ? WHERE id = ?`, now, roleID); err != nil {
		return fmt.Errorf("failed to update role updated_at: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit permission changes: %w", err)
	}
	return nil
}

// GetPermissionsByRoleName returns permissions configured for the role with the given name.
func (s *Storage) GetPermissionsByRoleName(ctx context.Context, roleName string) ([]string, error) {
	trimmed := strings.TrimSpace(roleName)
	if trimmed == "" {
		return nil, ErrNotFound
	}
	exists, err := s.RoleExists(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT p.permission
		FROM role_permissions p
		JOIN roles r ON r.id = p.role_id
		WHERE r.name = ? COLLATE NOCASE
		ORDER BY p.permission ASC
	`, trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to query permissions for role %s: %w", trimmed, err)
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan permission: %w", err)
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate permissions: %w", err)
	}
	if perms == nil {
		perms = []string{}
	}
	return perms, nil
}
