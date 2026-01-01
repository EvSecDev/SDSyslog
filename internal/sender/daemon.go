// Daemon for continuous sending of log messages from configured sources, encryption of messages, and delivery to configured network destinations
package sender

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/crypto/random"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/externalio/server"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/sender/managers/ingest"
	"sdsyslog/internal/sender/managers/out"
	"sdsyslog/internal/sender/managers/packaging"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/scaling"
	"sdsyslog/pkg/protocol"
	"strconv"
	"syscall"
	"time"
)

// Create new sending daemon instance
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
func (daemon *Daemon) Start(globalCtx context.Context, serverPub []byte) (err error) {
	// New context for the daemon
	daemon.ctx, daemon.cancel = context.WithCancel(context.Background())
	daemon.ctx = context.WithValue(daemon.ctx, global.LoggerKey, logctx.GetLogger(globalCtx))

	// Top level tag for daemon logs
	daemon.ctx = logctx.AppendCtxTag(daemon.ctx, global.NSSend)
	defer func() { daemon.ctx = logctx.RemoveLastCtxTag(daemon.ctx) }()

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog, "Starting...\n")

	// Setup destination network "connection"
	if daemon.cfg.DestinationIP == "" {
		err = fmt.Errorf("cannot start without a destination address")
		return
	}
	if daemon.cfg.DestinationPort == 0 {
		daemon.cfg.DestinationPort = global.DefaultReceiverPort
	}

	destAddr, err := net.ResolveUDPAddr("udp", daemon.cfg.DestinationIP+":"+strconv.Itoa(daemon.cfg.DestinationPort))
	if err != nil {
		err = fmt.Errorf("failed to resolve destination: %v", err)
		return
	}

	destinationConnection, err := net.DialUDP("udp", nil, destAddr)
	if err != nil {
		err = fmt.Errorf("failed to open udp socket: %v", err)
		return
	}

	// Pre-startup
	protocol.InitBidiMaps()
	wrappers.SetupEncryptInnerPayload(serverPub)
	daemon.cfg.setDefaults()

	global.Hostname, err = os.Hostname()
	if err != nil {
		err = fmt.Errorf("failed to determine local hostname: %v", err)
		return
	}
	global.PID = os.Getpid()

	mainHostID, err := random.FourByte()
	if err != nil {
		err = fmt.Errorf("failed to generate new unique host identifier: %v", err)
		return
	}

	maxPayloadSize, err := network.FindSendingMaxUDPPayload(daemon.cfg.DestinationIP)
	if err != nil {
		err = fmt.Errorf("failed to find max payload size: %v", err)
		return
	}
	if daemon.cfg.OverrideMaxPayloadSize != 0 {
		maxPayloadSize = daemon.cfg.OverrideMaxPayloadSize
	}

	// Stage 3 - Output Manager
	daemon.Mgrs.Out, err = out.NewInstanceManager(daemon.ctx,
		daemon.cfg.MinOutputQueueSize,
		destinationConnection,
		daemon.cfg.MinOutputs,
		daemon.cfg.MaxOutputs,
		daemon.cfg.MinOutputQueueSize,
		daemon.cfg.MaxOutputQueueSize)
	if err != nil {
		err = fmt.Errorf("error creating new output instance manager: %v", err)
		return
	}
	for i := 0; i < daemon.cfg.MinOutputs; i++ {
		_ = daemon.Mgrs.Out.AddInstance()
	}

	// Stage 2 - Assemblers
	daemon.Mgrs.Assem, err = packaging.NewInstanceManager(daemon.ctx,
		daemon.cfg.MinAssemblerQueueSize,
		daemon.Mgrs.Out.InQueue,
		mainHostID,
		maxPayloadSize,
		daemon.cfg.MinAssemblers,
		daemon.cfg.MaxAssemblers,
		daemon.cfg.MinAssemblerQueueSize,
		daemon.cfg.MaxAssemblerQueueSize)
	if err != nil {
		err = fmt.Errorf("error creating new assembly instance manager: %v", err)
		return
	}
	for i := 0; i < daemon.cfg.MinAssemblers; i++ {
		_ = daemon.Mgrs.Assem.AddInstance()
	}

	// Start handling exit signals before listener starts ingesting messages
	go signalHandler(daemon)

	// Stage 1 - Listeners(Readers)
	daemon.Mgrs.In = ingest.NewInstanceManager(daemon.ctx, daemon.Mgrs.Assem.InQueue)
	if len(daemon.cfg.FileSourcePaths) > 0 {
		for _, filePath := range daemon.cfg.FileSourcePaths {
			err = daemon.Mgrs.In.AddFileInstance(filePath, daemon.cfg.StateFilePath)
			if err != nil {
				err = fmt.Errorf("failed adding new file ingest instance: %v", err)
				daemon.Shutdown()
				return
			}
		}
	}
	if daemon.cfg.JournalSourceEnabled {
		err = daemon.Mgrs.In.AddJrnlInstance(daemon.cfg.StateFilePath)
		if err != nil {
			err = fmt.Errorf("failed creating journal ingest instance: %v", err)
			daemon.Shutdown()
			return
		}
	}

	// Metrics Collector
	daemon.metricsCollector = metrics.New(daemon.Mgrs.In,
		daemon.Mgrs.Assem,
		daemon.Mgrs.Out,
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
			daemon.Mgrs,
			global.LogicalCPUCount)
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

		daemon.MetricServer = server.SetupListener(serverCtx, daemon.MetricDataSearcher, daemon.MetricDiscoverer)
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

// Gracefully shutdown pipeline worker threads
func (daemon *Daemon) Shutdown() {
	daemon.ctx = logctx.AppendCtxTag(daemon.ctx, global.NSSend)
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

	// Stop ingest instances
	if daemon.Mgrs.In != nil {
		for filename := range daemon.Mgrs.In.FileSources {
			err := daemon.Mgrs.In.RemoveFileInstance(filename)
			if err != nil {
				logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
					"ingest worker shutdown failed: %v\n", err)
			}
		}
		if daemon.Mgrs.In.JournalSource != nil {
			daemon.Mgrs.In.RemoveJrnlInstance()
		}
	}

	// Stop assemblers
	if daemon.Mgrs.Assem != nil {
		queue := daemon.Mgrs.Assem.InQueue.ActiveWrite.Load()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth)
		if !success {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"assembler inbox queue did not empty in time: dropped %d messages\n", last)
		}
		for instanceID := range daemon.Mgrs.Assem.Instances {
			daemon.Mgrs.Assem.RemoveInstance(instanceID)
		}
	}

	// Stop output workers
	if daemon.Mgrs.Out != nil {
		queue := daemon.Mgrs.Out.InQueue.ActiveWrite.Load()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth)
		if !success {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"output inbox queue did not empty in time: dropped %d messages\n", last)
		}
		for instanceID := range daemon.Mgrs.Out.Instances {
			daemon.Mgrs.Out.RemoveInstance(instanceID)
		}
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
			"Timeout: send daemon did not shutdown within %v seconds",
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
