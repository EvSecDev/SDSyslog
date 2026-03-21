package server

import (
	"context"
	"net/http"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"strings"
)

// Handles metric search requests based on time for data
func handleData(baseCtx context.Context, search DataSearcher, serverResponder http.ResponseWriter, clientRequest *http.Request) {
	baseCtx = logctx.AppendCtxTag(baseCtx, logctx.NSMetricData)
	baseCtx = logctx.AppendCtxTag(baseCtx, clientRequest.RemoteAddr)

	if clientRequest.Method != http.MethodGet {
		logctx.LogStdErr(baseCtx, "Received invalid HTTP method %s\n", clientRequest.Method)
		serverResponder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rawNamespace := strings.TrimPrefix(clientRequest.URL.Path, DataPath)
	reqNamespace := strings.Split(rawNamespace, "/")

	reqName := clientRequest.FormValue("name")

	var err error

	rawStartTime := clientRequest.FormValue("starttime")
	rawEndTime := clientRequest.FormValue("endtime")

	reqStartTime, reqEndTime, err := parseTimeRangeNow(rawStartTime, rawEndTime)
	if err != nil {
		logctx.LogStdErr(baseCtx, "Received invalid search time range: %w\n", err)
		serverResponder.WriteHeader(http.StatusBadRequest)
		return
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
