package protocol

import (
	"fmt"
)

// Checks if filter matches the message fields
func (mf MessageFilter) Match(msg Message) (msgMatch bool) {
	matches := []bool{}

	if mf.Data != nil {
		matches = append(matches, mf.Data.Match(msg.Data))
	}

	if mf.FieldsKey != nil {
		keyMatched := false
		for key := range msg.Fields {
			keyMatched = mf.FieldsKey.Match([]byte(key))
			if keyMatched {
				break
			}
		}
		matches = append(matches, keyMatched)
	}

	if mf.FieldsValue != nil {
		valMatched := false
		for _, val := range msg.Fields {
			textVal := []byte(fmt.Sprint(val))
			valMatched = mf.FieldsValue.Match(textVal)
			if valMatched {
				break
			}
		}
		matches = append(matches, valMatched)
	}

	// No filters active, nothing to match
	if len(matches) == 0 {
		msgMatch = false
		return
	}

	// Combine results according to UseAnd flag
	if mf.UseAnd {
		for _, m := range matches {
			if !m {
				msgMatch = false
				return
			}
		}
		msgMatch = true
		return
	} else {
		for _, m := range matches {
			if m {
				msgMatch = true
				return
			}
		}
		msgMatch = false
		return
	}
}

// Checks if all configured message filters are valid
func (mf MessageFilter) Validate() (err error) {
	if mf.Data != nil {
		err = mf.Data.Validate()
		if err != nil {
			err = fmt.Errorf("invalid data filter: %w", err)
			return
		}
	}
	if mf.FieldsKey != nil {
		err = mf.FieldsKey.Validate()
		if err != nil {
			err = fmt.Errorf("invalid fields name filter: %w", err)
			return
		}
	}
	if mf.FieldsValue != nil {
		err = mf.FieldsValue.Validate()
		if err != nil {
			err = fmt.Errorf("invalid fields name filter: %w", err)
			return
		}
	}
	return
}
