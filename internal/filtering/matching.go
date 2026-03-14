// Boolean string matching via configuration driven filter
package filtering

import (
	"bytes"
)

// Checks if supplied string matches the filter
func (filter Filter) Match(input []byte) (matches bool) {
	if len(filter.And) > 0 {
		for _, sub := range filter.And {
			if !sub.Match(input) {
				matches = false
				return
			}
		}
		matches = true
		return
	}

	if len(filter.Or) > 0 {
		for _, sub := range filter.Or {
			if sub.Match(input) {
				matches = true
				return
			}
		}
		matches = false
		return
	}

	if filter.Not != nil {
		return !filter.Not.Match(input)
	}

	if filter.Prefix != "" {
		return bytes.HasPrefix(input, []byte(filter.Prefix))
	}

	if filter.Suffix != "" {
		return bytes.HasSuffix(input, []byte(filter.Suffix))
	}

	if filter.Contains != "" {
		return bytes.Contains(input, []byte(filter.Contains))
	}

	if filter.Exact != "" {
		return bytes.Equal(input, []byte(filter.Exact))
	}

	matches = false
	return
}
