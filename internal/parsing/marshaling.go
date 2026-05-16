package parsing

import (
	"encoding/json"
	"time"
)

// Custom JSON type for time.Duration
type Duration time.Duration

// Custom Time duration JSON unmarshaler
func (dur *Duration) UnmarshalJSON(data []byte) (err error) {
	var durationText string
	err = json.Unmarshal(data, &durationText)
	if err != nil {
		return
	}

	parsedDuration, err := time.ParseDuration(durationText)
	if err != nil {
		return
	}

	// Must use pointer to affect original
	*dur = Duration(parsedDuration)
	return
}

// Custom Time duration JSON marshaler
func (dur Duration) MarshalJSON() (data []byte, err error) {
	// Must NOT use pointer otherwise we marshal the underlying int64
	parsedDuration := time.Duration(dur)
	data, err = json.Marshal(parsedDuration.String())
	return
}
