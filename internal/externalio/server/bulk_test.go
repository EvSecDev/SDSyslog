package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"strings"
	"testing"
)

func TestHandleBulk(t *testing.T) {
	type mockInvalidJSON struct {
		Field string `json:"field"`
	}

	tests := []struct {
		name              string
		method            string
		reqBody           any
		handler           func(context.Context, http.ResponseWriter, *http.Request)
		wantStatus        int
		expectedError     bool
		expectedErrorText string
	}{
		{
			name:   "empty filter",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx, mockDataSearcher(nil), mockAggSearcher(metrics.Metric{}, nil), w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:    "invalid method",
			method:  http.MethodGet,
			reqBody: BulkRequest{},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx, mockDataSearcher(nil), mockAggSearcher(metrics.Metric{}, nil), w, r)
			},
			wantStatus:        http.StatusMethodNotAllowed,
			expectedError:     true,
			expectedErrorText: "Received invalid HTTP method GET",
		},
		{
			name:   "single data filter",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType: DataMode,
						Name:       "metric",
						Namespace:  "Receiver/Ingest",
						EndTime:    "now",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				mockMetric := metrics.Metric{
					Name: "metric",
				}
				handleBulk(ctx,
					mockDataSearcher([]metrics.Metric{mockMetric}),
					mockAggSearcher(metrics.Metric{}, fmt.Errorf("wrong call")),
					w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "single aggregation filter",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType:      AggregationMode,
						AggregationType: metrics.MetricAvg,
						Name:            "metric_",
						Namespace:       "Receiver/Ingest",
						EndTime:         "now",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				mockMetric := metrics.Metric{
					Name: "metric_",
				}
				handleBulk(ctx,
					mockDataSearcher([]metrics.Metric{}),
					mockAggSearcher(mockMetric, nil),
					w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "data and aggregation filter",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType: DataMode,
						Name:       "metric_data",
						Namespace:  "Receiver/Ingest",
						EndTime:    "now",
					},
					{
						SearchType:      AggregationMode,
						AggregationType: metrics.MetricSum,
						Name:            "metric_agg",
						Namespace:       "Receiver/Processor",
						EndTime:         "now",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				mockMetric1 := metrics.Metric{
					Name: "metric_data",
				}
				mockMetric2 := metrics.Metric{
					Name: "metric_agg",
				}
				handleBulk(ctx,
					mockDataSearcher([]metrics.Metric{mockMetric1}),
					mockAggSearcher(mockMetric2, nil),
					w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "invalid json body",
			method: http.MethodPost,
			reqBody: mockInvalidJSON{
				Field: "hello",
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx,
					mockDataSearcher([]metrics.Metric{}),
					mockAggSearcher(metrics.Metric{}, nil),
					w, r)
			},
			wantStatus:        http.StatusBadRequest,
			expectedError:     true,
			expectedErrorText: "Received invalid search JSON request",
		},
		{
			name:   "invalid search type",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType: "Discover",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx, mockDataSearcher([]metrics.Metric{}), mockAggSearcher(metrics.Metric{}, nil), w, r)
			},
			wantStatus:        http.StatusBadRequest,
			expectedError:     true,
			expectedErrorText: "Received invalid search request type \"Discover\"",
		},
		{
			name:   "no aggregation type",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType:      AggregationMode,
						AggregationType: "",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx, mockDataSearcher([]metrics.Metric{}), mockAggSearcher(metrics.Metric{}, nil), w, r)
			},
			wantStatus:        http.StatusBadRequest,
			expectedError:     true,
			expectedErrorText: "aggregation search type requires an aggregation type field 'aggregationType'",
		},
		{
			name:   "invalid time range",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType: DataMode,
						StartTime:  "-4w",
						EndTime:    "now",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx, mockDataSearcher([]metrics.Metric{}), mockAggSearcher(metrics.Metric{}, nil), w, r)
			},
			wantStatus:        http.StatusBadRequest,
			expectedError:     true,
			expectedErrorText: "Received invalid search time range",
		},
		{
			name:   "aggregation search error",
			method: http.MethodPost,
			reqBody: BulkRequest{
				SearchFilters: []MetricFilter{
					{
						SearchType:      AggregationMode,
						AggregationType: metrics.MetricAvg,
						StartTime:       "-5m",
						EndTime:         "now",
					},
				},
			},
			handler: func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				handleBulk(ctx,
					mockDataSearcher([]metrics.Metric{}),
					mockAggSearcher(metrics.Metric{}, fmt.Errorf("aggregation error")),
					w, r)
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = logctx.New(ctx, logctx.NSTest, 1, ctx.Done())

			testBody, err := json.Marshal(tt.reqBody)
			if err != nil {
				t.Fatalf("unexpected error mocking request body: %v", err)
			}
			bodyReader := bytes.NewReader(testBody)

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, BulkPath, bodyReader)

			tt.handler(ctx, rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status=%d want=%d", rr.Code, tt.wantStatus)
			}

			// Gather any logs from ctx logger
			logger := logctx.GetLogger(ctx)
			logger.Wake()
			lines := logger.GetFormattedLogLines()
			var foundErrors []string
			for _, line := range lines {
				if !strings.Contains(line, logctx.ErrorLog) && !strings.Contains(line, logctx.WarnLog) {
					continue
				}
				foundErrors = append(foundErrors, line)
			}

			if !tt.expectedError && len(foundErrors) == 0 {
				return
			} else if tt.expectedError && len(foundErrors) == 0 {
				t.Fatalf("expected error %q, but found none", tt.expectedErrorText)
			} else if !tt.expectedError && len(foundErrors) > 0 {
				t.Errorf("unexpected errors in logger:\n")
				for _, err := range foundErrors {
					t.Errorf("    %s", err)
				}
			} else if tt.expectedError && len(foundErrors) > 0 {
				var foundExpected bool
				for _, err := range foundErrors {
					if strings.Contains(err, tt.expectedErrorText) {
						foundExpected = true
						continue
					}
					t.Errorf("encountered unexpected error in logger: %s", err)
				}
				if !foundExpected {
					t.Errorf("expected error %q in logger, but found none", tt.expectedErrorText)
				}
			}
		})
	}
}
