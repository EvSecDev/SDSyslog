package server

import (
	"context"
	"net/http"
	"sdsyslog/internal/metrics"
	"strings"
	"time"
)

// Handles metric search requests based on time for data
func handleData(baseCtx context.Context, search DataSearcher, serverResponder http.ResponseWriter, clientRequest *http.Request) {
	rawNamespace := strings.TrimPrefix(clientRequest.URL.Path, "/data/")
	reqNamespace := strings.Split(rawNamespace, "/")

	reqName := clientRequest.FormValue("name")

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
	rawResults := search(reqName, reqNamespace, reqStartTime, reqEndTime)

	var results []metrics.JMetric
	for _, rawResult := range rawResults {
		results = append(results, rawResult.Convert())
	}

	if len(results) == 0 {
		jResp(baseCtx, serverResponder, Jerror{Msg: "Search returned no results"})
	} else {
		jResp(baseCtx, serverResponder, results)
	}
}
