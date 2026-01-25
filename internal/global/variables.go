package global

import "sync"

// Global state that is unchanging during program lifetime.
// Enforced WORM (write-once read-many) to prevent mutating global state.
var (
	bootID     string
	bootIDOnce sync.Once
)

// Initializes boot ID.
// Can only be called once, subsequent calls do nothing,
func SetBootID(id string) {
	bootIDOnce.Do(func() {
		bootID = id
	})
}

// Retrieve boot ID.
// Can return empty value if not initialized yet.
func BootID() string {
	return bootID
}
