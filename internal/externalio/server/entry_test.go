package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sdsyslog/internal/global"
	"sdsyslog/internal/metrics"
	"strings"
	"testing"
)

func TestSetupListener_RoutingAndHTML(t *testing.T) {
	ctx := context.Background()

	server, err := SetupListener(
		ctx,
		8080,
		mockDataSearcher(nil),
		mockDiscoverer(nil),
		mockAggSearcher(metrics.Metric{}, nil),
	)
	if err != nil {
		t.Fatalf("SetupListener error: %v", err)
	}

	ts := httptest.NewServer(server.Handler)
	defer ts.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		checkHTML  bool
	}{
		{
			name:       "root GET returns HTML with replacements",
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusOK,
			checkHTML:  true,
		},
		{
			name:       "root POST rejected",
			method:     http.MethodPost,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "data incorrect method",
			method:     http.MethodPost,
			path:       global.DataPath,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "discover incorrect method",
			method:     http.MethodPatch,
			path:       global.DiscoveryPath,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "aggregation incorrect method",
			method:     http.MethodDelete,
			path:       global.AggregationPath,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "unknown path",
			method:     http.MethodGet,
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, ts.URL+tt.path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("http request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status=%d want=%d", resp.StatusCode, tt.wantStatus)
			}

			if tt.checkHTML {
				body, _ := io.ReadAll(resp.Body)
				html := string(body)

				placeholders := []string{
					"{LISTEN_ADDR}",
					"{LISTEN_PORT}",
					"{DATA_PATH}",
					"{DISCOVER_PATH}",
					"{AGGREGATION_PATH}",
				}

				for _, ph := range placeholders {
					if strings.Contains(html, ph) {
						t.Fatalf("HTML placeholder not replaced: %s", ph)
					}
				}
			}
		})
	}
}
