// HTTP server to expose discovery and querying of metric data to other programs only on the local system
package server

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"strconv"
	"strings"
)

// Read in web static files at compile time
//
//go:embed static-files/metric-help.html
var webFiles embed.FS

// Sets up HTTP listener configuration for metric querying
func SetupListener(ctx context.Context, port int, search DataSearcher, discover Discoverer, aggregation AggSearcher) (server *http.Server, err error) {
	requestMultiplexer := http.NewServeMux()

	helpPage, err := webFiles.ReadFile("static-files/metric-help.html")
	if err != nil {
		err = fmt.Errorf("failed reading metric help html page from internal fs: %v\n", err)
		return
	}

	// Replace variables in html with globals
	helpPage = bytes.Replace(helpPage, []byte("@@LISTEN_ADDR@@"), []byte(global.HTTPListenAddr), 1)
	helpPage = bytes.Replace(helpPage, []byte("@@LISTEN_PORT@@"), []byte(strconv.Itoa(port)), 1)
	helpPage = bytes.Replace(helpPage, []byte("@@DATA_PATH@@"), []byte(global.DataPath), 1)
	helpPage = bytes.Replace(helpPage, []byte("@@DISCOVER_PATH@@"), []byte(global.DiscoveryPath), 1)
	helpPage = bytes.Replace(helpPage, []byte("@@AGGREGATION_PATH@@"), []byte(global.AggregationPath), 1)

	// Root help page
	requestMultiplexer.HandleFunc("/", func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		if clientRequest.Method != http.MethodGet {
			serverResponder.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if clientRequest.URL.Path != "/" {
			serverResponder.WriteHeader(http.StatusNotFound)
			return
		}

		serverResponder.Header().Set("Content-Type", "text/html; charset=utf-8")
		serverResponder.WriteHeader(http.StatusOK)
		serverResponder.Write(helpPage)
	})

	// Metric Discovery Requests
	requestMultiplexer.HandleFunc(global.DiscoveryPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		if clientRequest.Method != http.MethodGet {
			serverResponder.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleDiscovery(ctx, discover, serverResponder, clientRequest)
	})

	// Metric Data Requests
	requestMultiplexer.HandleFunc(global.DataPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		if clientRequest.Method != http.MethodGet {
			serverResponder.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleData(ctx, search, serverResponder, clientRequest)
	})

	// Metric Aggregation Requests
	requestMultiplexer.HandleFunc(global.AggregationPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		if clientRequest.Method != http.MethodGet {
			serverResponder.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleAggregation(ctx, aggregation, serverResponder, clientRequest)
	})

	// Server configuration
	server = &http.Server{
		Addr:         global.HTTPListenAddr + ":" + strconv.Itoa(port),
		Handler:      requestMultiplexer,
		ReadTimeout:  global.HTTPReadTimeout,
		WriteTimeout: global.HTTPWriteTimeout,
		IdleTimeout:  global.HTTPIdleTimeout,
		ErrorLog:     log.New(httpLogWriter{ctx: ctx}, "", 0),
	}

	return
}

// Starts the metric HTTP server and waits for requests
func Start(ctx context.Context, server *http.Server) {
	logctx.LogEvent(ctx, global.VerbosityStandard, global.InfoLog, "Metric query server starting on %s (http://%s/)\n",
		server.Addr,
		server.Addr,
	)
	err := server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Metric query server failed to start: %v\n", err)
	}
}

// Encodes JSON and sends as response body
func jResp(ctx context.Context, serverResponder http.ResponseWriter, content any) {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(content); err != nil {
		serverResponder.WriteHeader(http.StatusInternalServerError)
		logctx.LogEvent(ctx, global.VerbosityStandard, global.ErrorLog, "Failed marshaling metric results: %v\n", err)
		return
	}
	serverResponder.Header().Set("Content-Type", "application/json")
	serverResponder.WriteHeader(http.StatusOK)
	serverResponder.Write(buf.Bytes())
}

// Logs HTTP server errors to internal program buffer (via context logger)
func (logWriter httpLogWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return
	}
	logctx.LogEvent(
		logWriter.ctx,
		global.VerbosityStandard,
		global.ErrorLog,
		"%s\n", strings.TrimSpace(string(p)),
	)
	return
}
