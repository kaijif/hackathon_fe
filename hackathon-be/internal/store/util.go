package store

import "strings"

// prefixCols rewrites a comma-separated column list to prefix each bare column
// name with a table alias, e.g. prefixCols("g", "id, name") -> "g.id, g.name".
func prefixCols(alias, cols string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
