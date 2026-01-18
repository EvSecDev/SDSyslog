package server

import (
	"context"
	"net/http"
	"sdsyslog/internal/global"
	"strings"
	"time"
)

// Handles metric search requests based on time and aggregation type
func handleAggregation(baseCtx context.Context, search AggSearcher, serverResponder http.ResponseWriter, clientRequest *http.Request) {
	rawNamespace := strings.TrimPrefix(clientRequest.URL.Path, global.AggregationPath)
	reqNamespace := strings.Split(rawNamespace, "/")

	reqName := clientRequest.FormValue("name")
	aggType := clientRequest.FormValue("aggregation")

	var err error

	rawStartTime := clientRequest.FormValue("starttime")
	var reqStartTime time.Time
	if rawStartTime == "" {
		// Default start is last minute
		reqStartTime = time.Now().Add(-1 * time.Minute)
	} else if rawStartTime[0] == '-' || rawStartTime[0] == '+' {
		dur, err := time.ParseDuration(rawStartTime)
		if err == nil {
			reqStartTime = time.Now().Add(dur)
		} else {
			// Default start is last minute
			reqStartTime = time.Now().Add(-1 * time.Minute)
		}
	} else {
		reqStartTime, err = time.Parse(time.RFC3339Nano, rawStartTime)
		if err != nil {
			serverResponder.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	rawEndTime := clientRequest.FormValue("endtime")
	var reqEndTime time.Time
	if rawEndTime == "now" || rawEndTime == "" {
		reqEndTime = time.Now() // Default end is now
	} else {
		reqEndTime, err = time.Parse(time.RFC3339Nano, rawEndTime)
		if err != nil {
			serverResponder.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// Query internal metric registry
	result, err := search(aggType, reqName, reqNamespace, reqStartTime, reqEndTime)
	if err != nil {
		jResp(baseCtx, serverResponder, Jerror{Msg: err.Error()})
		return
	}
	jResp(baseCtx, serverResponder, result.Convert())
}
