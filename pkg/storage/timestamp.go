package storage

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// timestampString normalizes SQLite text and PostgreSQL timestamptz values for
// public records that expose timestamps as strings.
type timestampString struct{ value string }

func (t *timestampString) Scan(src any) error {
	switch v := src.(type) {
	case time.Time:
		t.value = v.UTC().Format(time.RFC3339Nano)
	case string:
		t.value = v
	case []byte:
		t.value = string(v)
	case nil:
		t.value = ""
	default:
		return fmt.Errorf("unsupported timestamp type %T", src)
	}
	return nil
}

func (t timestampString) Value() (driver.Value, error) { return t.value, nil }
