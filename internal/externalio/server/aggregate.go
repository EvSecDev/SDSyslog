package server

import (
	"context"
	"net/http"
	"sdsyslog/internal/logctx"
	"strings"
)

// Handles metric search requests based on time and aggregation type
func handleAggregation(baseCtx context.Context, search AggSearcher, serverResponder http.ResponseWriter, clientRequest *http.Request) {
	baseCtx = logctx.AppendCtxTag(baseCtx, logctx.NSMetricAgg)
	baseCtx = logctx.AppendCtxTag(baseCtx, clientRequest.RemoteAddr)

	if clientRequest.Method != http.MethodGet {
		logctx.LogStdErr(baseCtx, "Received invalid HTTP method %s\n", clientRequest.Method)
		serverResponder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rawNamespace := strings.TrimPrefix(clientRequest.URL.Path, AggregationPath)
	reqNamespace := strings.Split(rawNamespace, "/")

	reqName := clientRequest.FormValue("name")
	aggType := clientRequest.FormValue("aggregation")

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
	result, err := search(aggType, reqName, reqNamespace, reqStartTime, reqEndTime)
	if err != nil {
		jResp(baseCtx, serverResponder, Jerror{Msg: err.Error()})
		return
	}
	jResp(baseCtx, serverResponder, result.Convert())
}
