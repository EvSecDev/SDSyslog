package metrics

import (
	"testing"
	"time"
)

func TestRegistry_Prune(t *testing.T) {
	reg, ts := setupRegistryWithData(t)

	// Sanity check: ensure we start with 3 time slices worth of data
	allBefore := reg.Search("", nil, time.Time{}, time.Time{})
	if len(allBefore) == 0 {
		t.Fatalf("expected metrics before prune, got none")
	}

	// Choose current time just after ts3
	currentTime := ts["ts3"].Add(30 * time.Second)

	// Prune anything older than 1 minute
	reg.Prune(currentTime, 1*time.Minute)

	// After prune:
	// ts1 should be removed
	// ts2 may or may not survive depending on exact delta
	// ts3 must survive
	results := reg.Search("", nil, time.Time{}, time.Time{})

	if len(results) == 0 {
		t.Fatalf("expected metrics after prune, got none")
	}

	// Expect only metrics from ts2 and ts3
	for _, m := range results {
		if m.Timestamp.Before(ts["ts2"]) {
			t.Fatalf("unexpected old metric timestamp: %v", m.Timestamp)
		}
	}
}
