package dali

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// NullTime represents a time.Time that may be null. NullTime implements
// the sql.Scanner interface so it can be used as a scan destination.
type NullTime struct {
	Time  time.Time
	Valid bool
}

// Scan implements the sql.Scanner interface.
func (n *NullTime) Scan(value interface{}) error {
	if value == nil {
		n.Time, n.Valid = time.Time{}, false
		return nil
	}
	if t, ok := value.(time.Time); ok {
		n.Time, n.Valid = t, true
		return nil
	}
	return fmt.Errorf("cannoct convert %T to dali.NullTime", value)
}

// Value implements the driver.Value interface.
func (n NullTime) Value() (driver.Value, error) {
	if n.Valid {
		return n.Time, nil
	}
	return nil, nil
}
