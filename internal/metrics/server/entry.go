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
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
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
		err = fmt.Errorf("failed reading metric help html page from internal fs: %w", err)
		return
	}

	// Replace variables in html with globals
	helpPage = bytes.ReplaceAll(helpPage, []byte("{LISTEN_ADDR}"), []byte(ListenAddr))
	helpPage = bytes.ReplaceAll(helpPage, []byte("{LISTEN_PORT}"), []byte(strconv.Itoa(port)))
	helpPage = bytes.ReplaceAll(helpPage, []byte("{DATA_PATH}"), []byte(DataPath))
	helpPage = bytes.ReplaceAll(helpPage, []byte("{DISCOVER_PATH}"), []byte(DiscoveryPath))
	helpPage = bytes.ReplaceAll(helpPage, []byte("{AGGREGATION_PATH}"), []byte(AggregationPath))
	helpPage = bytes.ReplaceAll(helpPage, []byte("{BULK_PATH}"), []byte(BulkPath))

	// Root help page
	requestMultiplexer.HandleFunc("/", func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		if clientRequest.Method != http.MethodGet {
			logctx.LogStdErr(ctx, "Received invalid HTTP method %s\n", clientRequest.Method)
			serverResponder.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if clientRequest.URL.Path != "/" {
			logctx.LogStdErr(ctx, "Received invalid request path %q\n", clientRequest.URL.Path)
			serverResponder.WriteHeader(http.StatusNotFound)
			return
		}

		serverResponder.Header().Set("Content-Type", "text/html; charset=utf-8")
		serverResponder.WriteHeader(http.StatusOK)
		_, _ = serverResponder.Write(helpPage)
	})

	// Metric Discovery Requests
	requestMultiplexer.HandleFunc(DiscoveryPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		handleDiscovery(ctx, discover, serverResponder, clientRequest)
	})

	// Metric Data Requests
	requestMultiplexer.HandleFunc(DataPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		handleData(ctx, search, serverResponder, clientRequest)
	})

	// Metric Aggregation Requests
	requestMultiplexer.HandleFunc(AggregationPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		handleAggregation(ctx, aggregation, serverResponder, clientRequest)
	})

	// Metric Bulk Requests
	requestMultiplexer.HandleFunc("/"+BulkMode, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		handleBulk(ctx, search, aggregation, serverResponder, clientRequest)
	})
	requestMultiplexer.HandleFunc(BulkPath, func(serverResponder http.ResponseWriter, clientRequest *http.Request) {
		handleBulk(ctx, search, aggregation, serverResponder, clientRequest)
	})

	// Server configuration
	server = &http.Server{
		Addr:         ListenAddr + ":" + strconv.Itoa(port),
		Handler:      requestMultiplexer,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
		ErrorLog:     log.New(httpLogWriter{ctx: ctx}, "", 0),
	}

	return
}

// Starts the metric HTTP server and waits for requests
func Start(ctx context.Context, server *http.Server) {
	// Reuse existing port in case we are starting under a parent process (updating)
	conn, err := network.ReuseTCPPort(server.Addr)
	if err != nil {
		logctx.LogStdErr(ctx,
			"Metric query server failed to bind: %w\n", err)
		return
	}

	logctx.LogStdInfo(ctx, "Starting metric query server at http://%s/\n",
		server.Addr,
	)

	err = server.Serve(conn)
	if err != nil && err != http.ErrServerClosed {
		logctx.LogStdErr(ctx, "Metric query server failed to start: %w\n", err)
	}
}

// Encodes JSON and sends as response body
func jResp(ctx context.Context, serverResponder http.ResponseWriter, content any) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(content)
	if err != nil {
		serverResponder.WriteHeader(http.StatusInternalServerError)
		logctx.LogStdErr(ctx, "Failed marshaling metric results: %w\n", err)
		return
	}
	serverResponder.Header().Set("Content-Type", "application/json")
	serverResponder.WriteHeader(http.StatusOK)
	_, _ = serverResponder.Write(buf.Bytes())
}

// Logs HTTP server errors to internal program buffer (via context logger)
func (logWriter httpLogWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	if n == 0 {
		return
	}
	message := strings.TrimSpace(string(p))
	logctx.LogStdErr(logWriter.ctx, "%s\n", message)
	return
}
