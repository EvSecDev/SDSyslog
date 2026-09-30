package syslog

import "testing"

type lookupTestCase struct {
	name    string
	input   string
	code    uint16
	wantErr bool
}

func runLookupMappingTest(t *testing.T, toCode func(string) (uint16, error), toName func(uint16) (string, error), cases []lookupTestCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			code, err := toCode(tt.input)

			if tt.wantErr {
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

			roundTrip, err := toName(code)
			if err != nil {
				t.Fatalf("round-trip failed: %v", err)
			}

			if roundTrip != tt.input {
				t.Fatalf("round-trip mismatch: expected %q, got %q", tt.input, roundTrip)
			}
		})
	}
}

func TestSeverityMappings(t *testing.T) {
	runLookupMappingTest(t, SeverityToCode, CodeToSeverity, []lookupTestCase{
		{name: "valid severity emerg", input: "emerg", code: 0},
		{name: "valid severity info", input: "info", code: 6},
		{name: "unknown severity string", input: "nope", wantErr: true},
	})

	t.Run("unknown severity code", func(t *testing.T) {
		_, err := CodeToSeverity(999)
		if err == nil {
			t.Fatalf("expected error for unknown severity code")
		}
	})
}

func TestFacilityMappings(t *testing.T) {
	runLookupMappingTest(t, FacilityToCode, CodeToFacility, []lookupTestCase{
		{name: "valid facility kern", input: "kern", code: 0},
		{name: "valid facility local7", input: "local7", code: 23},
		{name: "unknown facility string", input: "bogus", wantErr: true},
	})

	t.Run("unknown facility code", func(t *testing.T) {
		_, err := CodeToFacility(999)
		if err == nil {
			t.Fatalf("expected error for unknown facility code")
		}
	})
}
