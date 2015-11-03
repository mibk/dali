package dali

import (
	"errors"
	"sort"
)

// OnlyCols tells the Preprocessor to use only the specified cols
// when interpolating v. It can be used for example like this:
//	u.GroupID = 5
//	db.Query("UPDATE user ?set", dali.OnlyCols(u, "group_id")).
//		MustExec()
func OnlyCols(v interface{}, cols ...string) onlyCols {
	if len(cols) == 0 {
		return onlyCols{err: errors.New("dali: no columns passed to OnlyCols")}
	}
	sort.Strings(cols)
	return onlyCols{v: v, cols: cols}
}

type onlyCols struct {
	err  error
	v    interface{}
	cols []string
}
