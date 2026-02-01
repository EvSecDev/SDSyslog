package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sdsyslog/internal/global"
	"sdsyslog/internal/metrics"
	"testing"
)

func TestHandleDiscovery(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		query      string
		results    []metrics.Metric
		wantStatus int
		wantError  bool
	}{
		{
			name:       "empty results returns JSON error",
			query:      "",
			results:    nil,
			wantStatus: http.StatusOK,
			wantError:  true,
		},
		{
			name:  "valid results name only",
			query: "?name=test",
			results: []metrics.Metric{
				{
					Name: "test",
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:  "valid results with namespace",
			query: "Receiver/Defrag/?name=test",
			results: []metrics.Metric{
				{
					Name:      "test",
					Namespace: []string{"Receiver", "Defrag"},
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:  "valid results with type",
			query: "Receiver/Defrag/?name=test&type=counter",
			results: []metrics.Metric{
				{
					Name:      "test",
					Namespace: []string{"Receiver", "Defrag"},
					Type:      metrics.Counter,
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid metric type",
			query:      "?type=invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(
				http.MethodGet,
				global.DiscoveryPath+tt.query,
				nil,
			)

			handleDiscovery(ctx, mockDiscoverer(tt.results), rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d", rr.Code, tt.wantStatus)
			}

			if tt.wantError {
				var je Jerror
				if err := json.NewDecoder(rr.Body).Decode(&je); err != nil {
					t.Fatalf("failed decoding JSON error: %v", err)
				}
				if je.Msg == "" {
					t.Fatal("expected non-empty error message")
				}
			}
		})
	}
}
