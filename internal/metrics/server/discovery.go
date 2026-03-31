package server

import (
	"context"
	"net/http"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics"
	"strings"
)

// Handles metric search to discover metrics (returns no actual data, only sample metric per individual metric)
func handleDiscovery(baseCtx context.Context, discover Discoverer, serverResponder http.ResponseWriter, clientRequest *http.Request) {
	baseCtx = logctx.AppendCtxTag(baseCtx, logctx.NSMetricDiscovery)
	baseCtx = logctx.AppendCtxTag(baseCtx, clientRequest.RemoteAddr)

	if clientRequest.Method != http.MethodGet {
		logctx.LogStdErr(baseCtx, "Received invalid HTTP method %s\n", clientRequest.Method)
		serverResponder.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	rawNamespace := strings.TrimPrefix(clientRequest.URL.Path, DiscoveryPath)

	var reqNamespace []string
	if rawNamespace != "" {
		reqNamespace = strings.Split(rawNamespace, "/")
	} else {
		reqNamespace = nil
	}

	reqName := clientRequest.FormValue("name")
	reqDescription := clientRequest.FormValue("description")
	reqUnit := clientRequest.FormValue("unit")

	rawType := clientRequest.FormValue("type")

	var reqType metrics.MetricType
	switch metrics.MetricType(strings.ToLower(rawType)) {
	case metrics.Counter:
		reqType = metrics.Counter
	case metrics.Gauge:
		reqType = metrics.Gauge
	case metrics.Summary:
		reqType = metrics.Summary
	default:
		// Empty is valid
		if rawType != "" {
			logctx.LogStdErr(baseCtx, "Received unsupported metric type %q\n", rawType)
			serverResponder.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	// Query internal metric registry
	rawResults := discover(reqName, reqDescription, reqNamespace, reqUnit, reqType)

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
