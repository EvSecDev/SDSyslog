package filtering

import "fmt"

// Ensures filter configuration is valid
func (filter Filter) Validate() (err error) {
	// Count how many fields are set
	count := 0
	if len(filter.And) > 0 {
		count++
	}
	if len(filter.Or) > 0 {
		count++
	}
	if filter.Not != nil {
		count++
	}
	if filter.Prefix != "" {
		count++
	}
	if filter.Suffix != "" {
		count++
	}
	if filter.Contains != "" {
		count++
	}
	if filter.Exact != "" {
		count++
	}

	// Exactly one field must be set
	if count == 0 {
		err = fmt.Errorf("filter node is empty")
		return
	}
	if count > 1 {
		err = fmt.Errorf("filter node must have exactly one operator/leaf, got %d", count)
		return
	}

	// Recursively validate children
	if len(filter.And) > 0 {
		for i, sub := range filter.And {
			err = sub.Validate()
			if err != nil {
				err = fmt.Errorf("and[%d]: %w", i, err)
				return
			}
		}
	}

	if len(filter.Or) > 0 {
		for i, sub := range filter.Or {
			err = sub.Validate()
			if err != nil {
				err = fmt.Errorf("or[%d]: %w", i, err)
				return
			}
		}
	}

	if filter.Not != nil {
		err = filter.Not.Validate()
		if err != nil {
			err = fmt.Errorf("not: %w", err)
			return
		}
	}

	// Leaf nodes (Prefix, Suffix, Contains, Exact) require no further validation
	return
}
