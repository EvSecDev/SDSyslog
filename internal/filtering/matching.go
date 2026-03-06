// Boolean string matching via configuration driven filter
package filtering

import "strings"

// Checks if supplied string matches the filter
func (filter Filter) Match(input string) (matches bool) {
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
		return strings.HasPrefix(input, filter.Prefix)
	}

	if filter.Suffix != "" {
		return strings.HasSuffix(input, filter.Suffix)
	}

	if filter.Contains != "" {
		return strings.Contains(input, filter.Contains)
	}

	if filter.Exact != "" {
		return input == filter.Exact
	}

	matches = false
	return
}
