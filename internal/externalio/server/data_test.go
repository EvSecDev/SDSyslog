package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sdsyslog/internal/global"
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
			path: global.DataPath + "?name=test",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "data invalid starttime",
			path: global.DataPath + "?starttime=badtime",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "data invalid triggers default relative start time",
			path: global.DataPath + "?starttime=-5w",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "data invalid relative end time",
			path: global.DataPath + "?endtime=+2y",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "data relative start time past",
			path: global.DataPath + "?starttime=-5m",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "data relative start time future",
			path: global.DataPath + "?starttime=+15m",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "data absolute start time",
			path: global.DataPath + "?starttime=2001-01-02T01:02:03.001Z",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleData(ctx, mockDataSearcher(nil), w, r)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "agg invalid starttime",
			path: global.AggregationPath + "?starttime=badtime",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), w, r,
				)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "agg invalid triggers default relative start time",
			path: global.AggregationPath + "?starttime=-5w",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), w, r,
				)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "agg invalid relative end time",
			path: global.AggregationPath + "?endtime=+2y",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), w, r,
				)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "agg relative start time past",
			path: global.AggregationPath + "?starttime=-5m",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), w, r,
				)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "agg relative start time future",
			path: global.AggregationPath + "?starttime=+15m",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), w, r,
				)
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "agg absolute start time",
			path: global.AggregationPath + "?starttime=2001-01-02T01:02:03.001Z",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, nil), w, r,
				)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "aggregation returns error as JSON",
			path: global.AggregationPath + "?aggregation=sum",
			handler: func(w http.ResponseWriter, r *http.Request) {
				handleAggregation(ctx,
					mockAggSearcher(metrics.Metric{}, errors.New("boom")), w, r,
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
