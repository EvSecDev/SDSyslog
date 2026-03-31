package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"strings"
)

// Handles metric search requests based on time for data
func handleBulk(baseCtx context.Context, search DataSearcher, aggregate AggSearcher, serverResponder http.ResponseWriter, clientRequest *http.Request) {
	baseCtx = logctx.AppendCtxTag(baseCtx, logctx.NSMetricBulk)
	baseCtx = logctx.AppendCtxTag(baseCtx, clientRequest.RemoteAddr)

	if clientRequest.Method != http.MethodPost {
		logctx.LogStdErr(baseCtx, "Received invalid HTTP method %s\n", clientRequest.Method)
		serverResponder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(clientRequest.Body)
	if err != nil {
		logctx.LogStdErr(baseCtx, "Received invalid search body: %w\n", err)
		serverResponder.WriteHeader(http.StatusInternalServerError)
		return
	}

	var request BulkRequest

	// Use a json.Decoder with Strict mode enabled
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(&request)
	if err != nil {
		logctx.LogStdErr(baseCtx, "Received invalid search JSON request: %w\n", err)
		serverResponder.WriteHeader(http.StatusBadRequest)
		return
	}

	allResults := make([][]metrics.JMetric, len(request.SearchFilters))
	for index, searchFilter := range request.SearchFilters {
		if searchFilter.SearchType != AggregationMode && searchFilter.SearchType != DataMode {
			logctx.LogStdErr(baseCtx, "Received invalid search request type %q\n", searchFilter.SearchType)
			serverResponder.WriteHeader(http.StatusBadRequest)
			return
		}

		if searchFilter.SearchType == AggregationMode && searchFilter.AggregationType == "" {
			logctx.LogStdErr(baseCtx, "%s search type requires an aggregation type field 'aggregationType'\n", searchFilter.SearchType)
			serverResponder.WriteHeader(http.StatusBadRequest)
			return
		}

		reqStartTime, reqEndTime, err := parseTimeRangeNow(searchFilter.StartTime, searchFilter.EndTime)
		if err != nil {
			logctx.LogStdErr(baseCtx, "Received invalid search time range: %w\n", err)
			serverResponder.WriteHeader(http.StatusBadRequest)
			return
		}

		reqNamespace := strings.Split(searchFilter.Namespace, "/")

		// Query internal metric registry
		var rawResults []metrics.Metric
		if searchFilter.AggregationType != "" {
			agg, err := aggregate(searchFilter.AggregationType, searchFilter.Name, reqNamespace, reqStartTime, reqEndTime)
			if err != nil {
				jResp(baseCtx, serverResponder, Jerror{Msg: err.Error()})
				return
			}
			rawResults = append(rawResults, agg)
		} else {
			rawResults = search(searchFilter.Name, reqNamespace, reqStartTime, reqEndTime)
		}

		for _, rawResult := range rawResults {
			allResults[index] = append(allResults[index], rawResult.Convert())
		}
	}

	if len(allResults) == 0 {
		jResp(baseCtx, serverResponder, Jerror{Msg: "Search returned no results"})
	} else {
		jResp(baseCtx, serverResponder, allResults)
	}
}
