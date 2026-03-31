// Daemon for continuous sending of log messages from configured sources, encryption of messages, and delivery to configured network destinations
package sender

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules/syslog"
	"sdsyslog/internal/lifecycle"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics/server"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/sender/assembler"
	"sdsyslog/internal/sender/ingest"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/output"
	"sdsyslog/internal/sender/scaling"
	"sdsyslog/pkg/crypto/registry"
	"slices"
	"time"
)

// Create new sending daemon instance
func NewDaemon(cfg Config) (new *Daemon) {
	new = &Daemon{
		cfg: cfg,
	}
	return
}

// Starts pipeline worker threads in background - gracefully shuts down if startup error is encountered
func (daemon *Daemon) Start(globalCtx context.Context, serverPub []byte) (err error) {
	startupTime := time.Now()

	// New context for the daemon
	daemon.ctx, daemon.cancel = context.WithCancel(globalCtx)
	daemon.ctx = context.WithValue(daemon.ctx, global.CtxModeKey, globalCtx.Value(global.CtxModeKey))

	// Top level tag for daemon logs (avoid duplicates)
	currentTags := logctx.GetTagList(daemon.ctx)
	if !slices.Equal(currentTags, []string{logctx.NSSend}) {
		daemon.ctx = logctx.AppendCtxTag(daemon.ctx, logctx.NSSend)
	}

	logctx.LogStdInfo(daemon.ctx, "Starting new daemon (%s)...\n", global.ProgVersion)

	// Pre-startup
	syslog.InitBidiMaps()
	daemon.cfg.setDefaults()
	err = wrappers.SetupEncryptInnerPayload(serverPub)
	if err != nil {
		err = fmt.Errorf("failed to setup encryption function: %w", err)
		return
	}
	if len(daemon.cfg.signingPrivateKey) > 0 {
		info, validID := registry.GetSignatureInfo(daemon.cfg.signatureSuiteID)
		if !validID {
			err = fmt.Errorf("invalid signature suite ID: %d", daemon.cfg.signatureSuiteID)
			return
		}
		err = info.ValidateKey(daemon.cfg.signingPrivateKey)
		if err != nil {
			err = fmt.Errorf("invalid signing key: %w", err)
			return
		}
		err = wrappers.SetupCreateSignature(daemon.cfg.signingPrivateKey)
		if err != nil {
			err = fmt.Errorf("failed to setup signing function: %w", err)
			return
		}
	}

	// Stage 3 - Output Manager
	outMgrConf := &output.ManagerConfig{
		MinQueueCapacity: daemon.cfg.MinOutputQueueSize,
		MaxQueueCapacity: daemon.cfg.MaxOutputQueueSize,
		DestinationIP:    daemon.cfg.DestinationIP,
		DestinationPort:  daemon.cfg.DestinationPort,
	}
	outMgrConf.MinInstanceCount.Store(uint32(daemon.cfg.MinOutputs))
	outMgrConf.MaxInstanceCount.Store(uint32(daemon.cfg.MaxOutputs))
	daemon.Mgrs.Out, err = outMgrConf.NewManager(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("error creating new output instance manager: %w", err)
		return
	}

	// Stage 3 - Output Instances
	for i := 0; i < int(daemon.cfg.MinOutputs); i++ {
		_ = daemon.Mgrs.Out.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d output instance(s) started successfully\n", daemon.cfg.MinOutputs)

	// Stage 2 - Assembler Manager
	pkgMgrConf := &assembler.ManagerConfig{
		MinQueueCapacity:       daemon.cfg.MinAssemblerQueueSize,
		MaxQueueCapacity:       daemon.cfg.MaxAssemblerQueueSize,
		OverrideMaxPayloadSize: daemon.cfg.OverrideMaxPayloadSize,
		DestinationIP:          daemon.cfg.DestinationIP,
		CryptoSuiteID:          daemon.cfg.transportCryptoSuiteID,
		SigSuiteID:             daemon.cfg.signatureSuiteID,
	}
	pkgMgrConf.MinInstanceCount.Store(uint32(daemon.cfg.MinAssemblers))
	pkgMgrConf.MaxInstanceCount.Store(uint32(daemon.cfg.MaxAssemblers))
	daemon.Mgrs.Assem, err = pkgMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Out.InQueue)
	if err != nil {
		err = fmt.Errorf("error creating new assembly instance manager: %w", err)
		return
	}

	// Stage 2 - Assembler Instance
	for i := 0; i < int(daemon.cfg.MinAssemblers); i++ {
		_ = daemon.Mgrs.Assem.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d assembler instance(s) started successfully\n", daemon.cfg.MinAssemblers)

	// Stage 1 - Listeners(Readers)
	inMgrConf := ingest.ManagerConfig{
		SourceDropFilters: daemon.cfg.Filters,
	}
	daemon.Mgrs.In = inMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Assem.InQueue)
	if len(daemon.cfg.FileSourcePaths) > 0 {
		for _, filePath := range daemon.cfg.FileSourcePaths {
			err = daemon.Mgrs.In.AddFileInstance(filePath, daemon.cfg.StateFilePath)
			if err != nil {
				err = fmt.Errorf("failed adding new file ingest instance: %w", err)
				daemon.Shutdown()
				return
			}
		}
	}
	if daemon.cfg.JournalSourceEnabled {
		err = daemon.Mgrs.In.AddJrnlInstance(daemon.cfg.StateFilePath)
		if err != nil {
			err = fmt.Errorf("failed creating journal ingest instance: %w", err)
			daemon.Shutdown()
			return
		}
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"1 ingest instance started successfully\n")

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
	daemon.MetricAggregator = daemon.metricsCollector.Registry.Aggregate

	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"Metric collection instance started successfully\n")

	// Autoscaler
	if daemon.cfg.AutoscaleEnabled {
		if daemon.cfg.AutoscaleCheckInterval == 0 {
			daemon.cfg.AutoscaleCheckInterval = 1 * time.Second
		}

		scaler := scaling.New(daemon.metricsCollector.Registry,
			daemon.cfg.AutoscaleCheckInterval,
			daemon.Mgrs,
			runtime.NumCPU())
		workerCtx := daemon.ctx
		daemon.wg.Add(1)
		go func() {
			defer daemon.wg.Done()
			scaler.Run(workerCtx)
		}()
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Autoscaler instance started successfully\n")
	}

	// Metric Server
	if daemon.cfg.MetricQueryServerEnabled {
		// Top level tag for metric server logs (copy so return doesn't strip ns tags)
		serverCtx := daemon.ctx
		serverCtx = logctx.AppendCtxTag(serverCtx, logctx.NSMetric)
		serverCtx = logctx.AppendCtxTag(serverCtx, logctx.NSMetricSrv)

		daemon.MetricServer, err = server.SetupListener(serverCtx,
			daemon.cfg.MetricQueryServerPort,
			daemon.MetricDataSearcher,
			daemon.MetricDiscoverer,
			daemon.MetricAggregator)
		if err != nil {
			err = fmt.Errorf("failed creating HTTP metric server: %w", err)
			daemon.Shutdown()
			return
		}

		daemon.wg.Add(1)
		go func() {
			defer daemon.wg.Done()
			server.Start(serverCtx, daemon.MetricServer)
		}()
	}

	// For update hot-swap/systemd
	err = lifecycle.ReadinessSender()
	if err != nil {
		err = fmt.Errorf("error sending readiness to parent process: %w", err)
		daemon.Shutdown()
		return
	}
	lifecycle.PostUpdateActions(daemon.ctx, daemon, 2*time.Second)
	err = lifecycle.NotifyReady(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("error sending readiness to systemd: %w", err)
		daemon.Shutdown()
		return
	}

	startupElapsed := parsing.TrimDurationPrecision(time.Since(startupTime), 2)
	logctx.LogStdInfo(daemon.ctx, "Startup complete in %s (%s). Sending messages to %s:%d\n",
		startupElapsed, global.ProgVersion, daemon.cfg.DestinationIP, daemon.cfg.DestinationPort)
	return
}

// No-op to satisfy DaemonLike type
func (daemon *Daemon) StartFIPR() (err error) {
	return
}

// No-op to satisfy DaemonLike type
func (daemon *Daemon) StopFIPR() {
}

// Blocking daemon waiter
func (daemon *Daemon) Run() {
	// Block on signals only
	lifecycle.SignalHandler(daemon.ctx, daemon)
}

// Gracefully shutdown pipeline worker threads
func (daemon *Daemon) Shutdown() {
	shutdownTime := time.Now()
	logctx.LogStdInfo(daemon.ctx, "Daemon shutdown started (%s)...\n", global.ProgVersion)

	// Stop metric server
	if daemon.cfg.MetricQueryServerEnabled {
		if daemon.MetricServer != nil {
			err := daemon.MetricServer.Shutdown(daemon.ctx)
			if err != nil && err != http.ErrServerClosed {
				logctx.LogStdWarn(daemon.ctx, "metric HTTP server did not shutdown gracefully: %w\n", err)
			} else {
				logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
					"Metric server stopped successfully\n")
			}
		}
	}

	// Stop ingest instances
	if daemon.Mgrs.In != nil {
		for filename := range daemon.Mgrs.In.FileSources {
			err := daemon.Mgrs.In.RemoveFileInstance(filename)
			if err != nil {
				logctx.LogStdWarn(daemon.ctx, "ingest file worker shutdown failed: %w\n", err)
			} else {
				logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
					"Successfully stopped ingest file instance\n")
			}
		}
		if daemon.Mgrs.In.JournalSource != nil {
			err := daemon.Mgrs.In.RemoveJrnlInstance()
			if err != nil {
				logctx.LogStdWarn(daemon.ctx, "ingest journal worker shutdown failed: %w\n", err)
			} else {
				logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
					"Successfully stopped ingest journald instance\n")
			}
		}
	}

	// Stop assemblers
	if daemon.Mgrs.Assem != nil {
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Draining assembler worker queue...\n")

		queue := daemon.Mgrs.Assem.InQueue.ActiveWrite.Load()
		queue.ResyncDepthMetric()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogStdWarn(daemon.ctx, "assembler inbox queue did not empty in time: dropped %d messages\n", last)
		}

		for {
			instanceList := daemon.Mgrs.Assem.Instances.Load()
			if len(*instanceList) == 0 {
				break
			}
			removedID := daemon.Mgrs.Assem.RemoveLastInstance()
			logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
				"Successfully stopped assembler instance %d\n", removedID)
		}
	}

	// Stop output workers
	if daemon.Mgrs.Out != nil {
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Draining output worker queue...\n")

		queue := daemon.Mgrs.Out.InQueue.ActiveWrite.Load()
		queue.ResyncDepthMetric()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogStdWarn(daemon.ctx, "output inbox queue did not empty in time: dropped %d messages\n", last)
		}

		for {
			instanceList := daemon.Mgrs.Out.Instances.Load()
			if len(*instanceList) == 0 {
				break
			}
			removedID := daemon.Mgrs.Out.RemoveLastInstance()
			logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
				"Successfully stopped output instance %d\n", removedID)
		}
	}

	// Stop the run loop after instances are drained and stopped
	daemon.cancel()
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"Issued cancel to miscellaneous worker instances, waiting for workers to exit...\n")

	// Wait for all workers to finish (with timeout)
	done := make(chan struct{})
	go func() {
		daemon.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logctx.LogStdInfo(daemon.ctx, "Daemon shutdown completed successfully in %s\n",
			parsing.TrimDurationPrecision(time.Since(shutdownTime), 2))
	case <-time.After(ShutdownTimeout):
		logctx.LogStdWarn(daemon.ctx, "Timeout: send daemon component did not shutdown within %v seconds (total elapsed: %s)",
			ShutdownTimeout.Seconds(), parsing.TrimDurationPrecision(time.Since(shutdownTime), 2))
		return
	}
}
