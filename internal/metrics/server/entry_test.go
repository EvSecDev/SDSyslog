package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"sdsyslog/internal/tests/utils"
	"strings"
	"testing"
)

func TestSetupListener_RoutingAndHTML(t *testing.T) {
	tests := []struct {
		name              string
		method            string
		path              string
		wantStatus        int
		expectedError     bool
		expectedErrorText string
		checkHTML         bool
	}{
		{
			name:       "root GET returns HTML with replacements",
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusOK,
			checkHTML:  true,
		},
		{
			name:              "root POST rejected",
			method:            http.MethodPost,
			path:              "/",
			wantStatus:        http.StatusMethodNotAllowed,
			expectedError:     true,
			expectedErrorText: "Received invalid HTTP method POST",
		},
		{
			name:              "data incorrect method",
			method:            http.MethodPost,
			path:              DataPath,
			wantStatus:        http.StatusMethodNotAllowed,
			expectedError:     true,
			expectedErrorText: "Received invalid HTTP method POST",
		},
		{
			name:              "discover incorrect method",
			method:            http.MethodPatch,
			path:              DiscoveryPath,
			wantStatus:        http.StatusMethodNotAllowed,
			expectedError:     true,
			expectedErrorText: "Received invalid HTTP method PATCH",
		},
		{
			name:              "aggregation incorrect method",
			method:            http.MethodDelete,
			path:              AggregationPath,
			wantStatus:        http.StatusMethodNotAllowed,
			expectedError:     true,
			expectedErrorText: "Received invalid HTTP method DELETE",
		},
		{
			name:              "bulk root path",
			method:            http.MethodPost,
			path:              BulkPath,
			wantStatus:        http.StatusBadRequest, // request handler should return for empty body
			expectedError:     true,
			expectedErrorText: "Received invalid search JSON request: EOF",
		},
		{
			name:              "bulk root path no trailing slash",
			method:            http.MethodPost,
			path:              "/" + BulkMode,
			wantStatus:        http.StatusBadRequest, // request handler should return for empty body
			expectedError:     true,
			expectedErrorText: "Received invalid search JSON request: EOF",
		},
		{
			name:              "unknown path",
			method:            http.MethodGet,
			path:              "/unknown",
			wantStatus:        http.StatusNotFound,
			expectedError:     true,
			expectedErrorText: "Received invalid request path \"/unknown\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = logctx.New(ctx, logctx.NSTest, 1, ctx.Done())

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

			req, _ := http.NewRequest(tt.method, ts.URL+tt.path, nil)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("http request failed: %v", err)
			}
			defer func() {
				err = resp.Body.Close()
				if err != nil {
					t.Fatalf("failed closing response body: %v", err)
				}
			}()

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
					"{BULK_PATH}",
				}

				for _, ph := range placeholders {
					if strings.Contains(html, ph) {
						t.Fatalf("HTML placeholder not replaced: %s", ph)
					}
				}
			}

			// Check logs
			_, err = utils.MatchLogCtxErrors(ctx, tt.expectedErrorText, nil)
			if err != nil {
				t.Errorf("%v", err)
			}
		})
	}
}
