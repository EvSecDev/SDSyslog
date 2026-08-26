package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sdsyslog/internal/metrics"
	"testing"
)

func TestHandleDataAndAggregation(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		path       string
		handler    func(http.ResponseWriter, *http.Request)
		wantStatus int
	}{
		{
			name: "data default times",
			path: DataPath + "?name=test",
			handler: func(writer http.ResponseWriter, reader *http.Request) {
				handleData(ctx, mockDataSearcher(nil), writer, reader)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "data supplied times",
			path: DataPath + "?name=test&starttime=-5m&endttime=now",
			handler: func(writer http.ResponseWriter, reader *http.Request) {
				handleData(ctx, mockDataSearcher([]metrics.Metric{{Name: "test"}}), writer, reader)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "data invalid starttime",
			path: DataPath + "?starttime=badtime",
			handler: func(writer http.ResponseWriter, reader *http.Request) {
				handleData(ctx, mockDataSearcher(nil), writer, reader)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "aggregation supplied times",
			path: AggregationPath + "?name=test&starttime=-5m&endttime=now",
			handler: func(writer http.ResponseWriter, reader *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{Name: "test"}, nil),
					writer, reader)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "agg invalid starttime",
			path: AggregationPath + "?starttime=badtime",
			handler: func(writer http.ResponseWriter, reader *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), writer, reader,
				)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "aggregation returns error as JSON",
			path: AggregationPath + "?aggregation=sum",
			handler: func(writer http.ResponseWriter, reader *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, errors.New("boom")), writer, reader,
				)
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)

			tt.handler(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d", rr.Code, tt.wantStatus)
			}
		})
	}
}
