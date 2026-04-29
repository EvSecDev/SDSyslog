package global

type (
	// Context keys
	CtxMode            string
	CtxOriginalExePath string

	// Prevent mixing when moving through function signatures
	MinValue int // Number representing the minimum value for any use case
	MaxValue int // Number representing the maximum value for any use case
)
