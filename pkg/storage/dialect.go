package storage

import (
	"strconv"
	"strings"
)

// rebind converts positional SQLite placeholders to PostgreSQL placeholders while
// leaving quoted SQL text untouched. SQLite returns the original string.
func (s *Storage) rebind(query string) string {
	if s.dialect != "postgres" || !strings.Contains(query, "?") {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	arg := 0
	quoted := byte(0)
	for i := 0; i < len(query); i++ {
		c := query[i]
		if quoted != 0 {
			b.WriteByte(c)
			if c == quoted {
				if i+1 < len(query) && query[i+1] == quoted {
					b.WriteByte(query[i+1])
					i++
					continue
				}
				quoted = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quoted = c
			b.WriteByte(c)
			continue
		}
		if c == '?' {
			arg++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(arg))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
