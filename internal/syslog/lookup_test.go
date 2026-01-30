package syslog

import "testing"

func TestSeverityMappings(t *testing.T) {
	// Ensure reverse maps are initialized
	InitBidiMaps()

	tests := []struct {
		name      string
		severity  string
		code      uint16
		expectErr bool
	}{
		{
			name:     "valid severity emerg",
			severity: "emerg",
			code:     0,
		},
		{
			name:     "valid severity info",
			severity: "info",
			code:     6,
		},
		{
			name:      "unknown severity string",
			severity:  "nope",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := SeverityToCode(tt.severity)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if code != tt.code {
				t.Fatalf("expected code %d, got %d", tt.code, code)
			}

			// Round-trip: code -> severity
			roundTrip, err := CodeToSeverity(code)
			if err != nil {
				t.Fatalf("round-trip failed: %v", err)
			}

			if roundTrip != tt.severity {
				t.Fatalf("round-trip mismatch: expected %q, got %q", tt.severity, roundTrip)
			}
		})
	}

	// Test unknown code explicitly
	t.Run("unknown severity code", func(t *testing.T) {
		_, err := CodeToSeverity(999)
		if err == nil {
			t.Fatalf("expected error for unknown severity code")
		}
	})
}

func TestFacilityMappings(t *testing.T) {
	// Ensure reverse maps are initialized
	InitBidiMaps()

	tests := []struct {
		name      string
		facility  string
		code      uint16
		expectErr bool
	}{
		{
			name:     "valid facility kern",
			facility: "kern",
			code:     0,
		},
		{
			name:     "valid facility local7",
			facility: "local7",
			code:     23,
		},
		{
			name:      "unknown facility string",
			facility:  "bogus",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, err := FacilityToCode(tt.facility)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if code != tt.code {
				t.Fatalf("expected code %d, got %d", tt.code, code)
			}

			// Round-trip: code -> facility
			roundTrip, err := CodeToFacility(code)
			if err != nil {
				t.Fatalf("round-trip failed: %v", err)
			}

			if roundTrip != tt.facility {
				t.Fatalf("round-trip mismatch: expected %q, got %q", tt.facility, roundTrip)
			}
		})
	}

	// Test unknown code explicitly
	t.Run("unknown facility code", func(t *testing.T) {
		_, err := CodeToFacility(999)
		if err == nil {
			t.Fatalf("expected error for unknown facility code")
		}
	})
}
