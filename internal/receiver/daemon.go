// Daemon for continuous reception of log messages, processing of messages, and delivery to configured output destinations
package receiver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/externalio/server"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/managers/defrag"
	"sdsyslog/internal/receiver/managers/in"
	"sdsyslog/internal/receiver/managers/out"
	"sdsyslog/internal/receiver/managers/proc"
	"sdsyslog/internal/receiver/metrics"
	"sdsyslog/internal/receiver/scaling"
	"sdsyslog/pkg/protocol"
	"strings"
	"syscall"
	"time"
)

// Create new receiver daemon instance
func NewDaemon(cfg Config) (new *Daemon) {
	ctx, cancel := context.WithCancel(context.Background())
	new = &Daemon{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
	return
}

// Starts pipeline worker threads in background - gracefully shuts down if startup error is encountered
func (daemon *Daemon) Start(globalCtx context.Context, serverPriv []byte) (err error) {
	// New context for the daemon
	daemon.ctx, daemon.cancel = context.WithCancel(context.Background())
	daemon.ctx = context.WithValue(daemon.ctx, global.LoggerKey, logctx.GetLogger(globalCtx))

	// Top level tag for daemon logs
	daemon.ctx = logctx.AppendCtxTag(daemon.ctx, global.NSRecv)
	defer func() { daemon.ctx = logctx.RemoveLastCtxTag(daemon.ctx) }()

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog, "Starting...\n")

	// Pre-startup
	protocol.InitBidiMaps()
	wrappers.SetupDecryptInnerPayload(serverPriv)
	daemon.cfg.setDefaults()

	global.Hostname, err = os.Hostname()
	if err != nil {
		err = fmt.Errorf("failed to determine local hostname: %v", err)
		return
	}
	global.PID = os.Getpid()

	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		err = fmt.Errorf("failed to determine local boot id: %v", err)
		return
	}
	global.BootID = strings.TrimSpace(string(data))

	// Stage 4 - Output Manager
	daemon.Mgrs.Output, err = out.NewInstanceManager(daemon.ctx, daemon.cfg.MinOutputQueueSize)
	if err != nil {
		err = fmt.Errorf("failed creating output instance manager: %v", err)
		return
	}
	err = daemon.Mgrs.Output.AddInstance(daemon.cfg.OutputFilePath, daemon.cfg.JournaldURL)
	if err != nil {
		err = fmt.Errorf("failed starting output: %v", err)
		return
	}

	// Stage 3 - Shard+Assembler
	daemon.Mgrs.Defrag = defrag.NewInstanceManager(daemon.ctx,
		daemon.Mgrs.Output.Queue,
		daemon.cfg.MinDefrags,
		daemon.cfg.MaxDefrags)
	for i := 0; i < daemon.cfg.MinDefrags; i++ {
		_ = daemon.Mgrs.Defrag.AddInstance()
	}

	// Stage 2 - Processor
	daemon.Mgrs.Proc, err = proc.NewInstanceManager(daemon.ctx,
		daemon.cfg.MinProcessorQueueSize,
		daemon.Mgrs.Defrag.Routing,
		daemon.cfg.MinProcessors,
		daemon.cfg.MaxProcessors,
		daemon.cfg.MinProcessorQueueSize,
		daemon.cfg.MaxProcessorQueueSize)
	if err != nil {
		err = fmt.Errorf("failed adding new processor manager: %v", err)
		daemon.Shutdown()
		return
	}
	for i := 0; i < daemon.cfg.MinProcessors; i++ {
		daemon.Mgrs.Proc.AddInstance()
	}

	// Start handling exit signals before listener starts ingesting messages
	go signalHandler(daemon)

	// Stage 1 - Listener
	daemon.Mgrs.Input = in.NewInstanceManager(daemon.ctx,
		daemon.cfg.ListenPort,
		daemon.Mgrs.Proc.Inbox,
		daemon.cfg.MinListeners,
		daemon.cfg.MaxListeners)
	for i := 0; i < daemon.cfg.MinListeners; i++ {
		_, err = daemon.Mgrs.Input.AddInstance()
		if err != nil {
			err = fmt.Errorf("failed adding new listener instance: %v", err)
			daemon.Shutdown()
			return
		}
	}

	// Metrics Collector
	daemon.metricsCollector = metrics.New(daemon.Mgrs,
		daemon.cfg.MetricCollectionInterval,
		daemon.cfg.MetricMaxAge)
	workerCtx := daemon.ctx
	daemon.wg.Add(1)
	go func() {
		defer daemon.wg.Done()
		daemon.metricsCollector.Run(workerCtx)
	}()
	daemon.MetricDataSearcher = daemon.metricsCollector.Registry.Search
	daemon.MetricDiscoverer = daemon.metricsCollector.Registry.Discover

	// Autoscaler
	if daemon.cfg.AutoscaleEnabled {
		if daemon.cfg.AutoscaleCheckInterval == 0 {
			daemon.cfg.AutoscaleCheckInterval = 1 * time.Second
		}

		scaler := scaling.New(daemon.metricsCollector.Registry,
			daemon.cfg.AutoscaleCheckInterval,
			daemon.Mgrs)
		workerCtx := daemon.ctx
		daemon.wg.Add(1)
		go func() {
			defer daemon.wg.Done()
			scaler.Run(workerCtx)
		}()
	}

	// Metric Server
	if daemon.cfg.MetricQueryServerEnabled {
		// Top level tag for metric server logs (copy so return doesn't strip ns tags)
		serverCtx := daemon.ctx
		serverCtx = logctx.AppendCtxTag(serverCtx, global.NSMetric)
		serverCtx = logctx.AppendCtxTag(serverCtx, global.NSMetricSrv)

		daemon.MetricServer = server.SetupListener(serverCtx,
			daemon.cfg.MetricQueryServerPort,
			daemon.MetricDataSearcher,
			daemon.MetricDiscoverer)
		daemon.wg.Add(1)
		go func() {
			defer daemon.wg.Done()
			server.Start(serverCtx, daemon.MetricServer)
		}()
	}

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog, "Startup complete.\n")
	return
}

// Blocking daemon waiter
func (daemon *Daemon) Run() {
	<-daemon.ctx.Done()
}

// Gracefully shutdown pipeline worker threads (errors are printed to program log buffer)
func (daemon *Daemon) Shutdown() {
	daemon.ctx = logctx.AppendCtxTag(daemon.ctx, global.NSRecv)
	defer func() { daemon.ctx = logctx.RemoveLastCtxTag(daemon.ctx) }()

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog,
		"Daemon shutdown started...\n")

	// Stop metric server
	if daemon.cfg.MetricQueryServerEnabled {
		err := daemon.MetricServer.Shutdown(daemon.ctx)
		if err != nil && err != http.ErrServerClosed {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"metric HTTP server did not shutdown gracefully: %v\n", err)
		}
	}

	// Stop listener instances
	if daemon.Mgrs.Input != nil {
		for instanceID := range daemon.Mgrs.Input.Instances {
			daemon.Mgrs.Input.RemoveInstance(instanceID)
		}
	}

	// Stop processor instances
	if daemon.Mgrs.Proc != nil {
		queue := daemon.Mgrs.Proc.Inbox.ActiveWrite.Load()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth)
		if !success {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"assembler inbox queue did not empty in time: dropped %d messages\n", last)
		}
		for instanceID := range daemon.Mgrs.Proc.Instances {
			daemon.Mgrs.Proc.RemoveInstance(instanceID)
		}
	}

	// Stop defrag and shards
	if daemon.Mgrs.Defrag != nil {
		for instanceID := range daemon.Mgrs.Defrag.InstancePairs {
			daemon.Mgrs.Defrag.RemoveInstance(instanceID)
		}
	}

	// Stop output worker
	if daemon.Mgrs.Output != nil {
		queue := daemon.Mgrs.Output.Queue.ActiveWrite.Load()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth) // Wait here before shutting down (active write always has newest data)
		if !success {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"assembler inbox queue did not empty in time: dropped %d messages\n", last)
		}
		daemon.Mgrs.Output.RemoveInstance()
	}

	// Stop the run loop after instances are drained and stopped
	daemon.cancel()

	// Wait for all workers to finish (with timeout)
	done := make(chan struct{})
	go func() {
		daemon.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog,
			"Daemon shutdown completed successfully\n")
	case <-time.After(global.ReceiveShutdownTimeout):
		logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog,
			"Timeout: receive daemon did not shutdown within %v seconds",
			global.ReceiveShutdownTimeout.Seconds())
		return
	}
}

// Handle exit requests and initiate graceful shutdown on signal reception
func signalHandler(daemon *Daemon) {
	// Channel for handling interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-sigChan

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog,
		"Received signal: %v\n", sig)

	// Start daemon shutdown
	daemon.Shutdown()
	logger := logctx.GetLogger(daemon.ctx)
	logger.Wake()
	logger.Wait()
	os.Exit(0)
}
