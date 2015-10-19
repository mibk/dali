package dali

import (
	"database/sql/driver"
	"fmt"
	"time"
)

type NullTime struct {
	Time  time.Time
	Valid bool
}

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

func (n NullTime) Value() (driver.Value, error) {
	if n.Valid {
		return n.Time, nil
	}
	return nil, nil
}
