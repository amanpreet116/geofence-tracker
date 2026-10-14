package repository

import (
	"fmt"
	"strings"
)

// where builds a dynamic WHERE clause with positional parameters only.
// Values always go into args; the SQL text only ever receives "$n"
// placeholders, so no value is interpolated into the query.
type where struct {
	conds []string
	args  []any
}

// add appends a condition. cond must contain exactly one "%s", which is
// replaced by the value's placeholder, e.g. add("e.vehicle_id = %s", id).
func (w *where) add(cond string, value any) {
	w.args = append(w.args, value)
	w.conds = append(w.conds, fmt.Sprintf(cond, w.placeholder()))
}

// arg appends a value that is not a condition (e.g. LIMIT) and returns its
// placeholder.
func (w *where) arg(value any) string {
	w.args = append(w.args, value)
	return w.placeholder()
}

func (w *where) placeholder() string { return fmt.Sprintf("$%d", len(w.args)) }

// sql renders " WHERE a AND b", or "" when there are no conditions.
func (w *where) sql() string {
	if len(w.conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.conds, " AND ")
}
