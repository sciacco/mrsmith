package afctools

import "html"

func normalizeWHMCSText(s string) string {
	return html.UnescapeString(s)
}

func normalizeWHMCSTextPtrs(fields ...*string) {
	for _, field := range fields {
		if field == nil {
			continue
		}
		*field = normalizeWHMCSText(*field)
	}
}
