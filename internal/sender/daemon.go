// Daemon for continuous sending of log messages from configured sources, encryption of messages, and delivery to configured network destinations
package sender

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/externalio/server"
	"sdsyslog/internal/global"
	"sdsyslog/internal/lifecycle"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/sender/managers/ingest"
	"sdsyslog/internal/sender/managers/out"
	"sdsyslog/internal/sender/managers/packaging"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/scaling"
	"sdsyslog/internal/syslog"
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
	// New context for the daemon
	daemon.ctx, daemon.cancel = context.WithCancel(globalCtx)
	daemon.ctx = context.WithValue(daemon.ctx, global.CtxModeKey, globalCtx.Value(global.CtxModeKey))
	daemon.ctx = context.WithValue(daemon.ctx, logctx.LoggerKey, logctx.GetLogger(globalCtx))

	// Top level tag for daemon logs (avoid duplicates)
	currentTags := logctx.GetTagList(daemon.ctx)
	if !slices.Equal(currentTags, []string{logctx.NSSend}) {
		daemon.ctx = logctx.AppendCtxTag(daemon.ctx, logctx.NSSend)
	}

	logctx.LogStdInfo(daemon.ctx, "Starting new daemon (%s)...\n", global.ProgVersion)

	// Pre-startup
	syslog.InitBidiMaps()
	err = wrappers.SetupEncryptInnerPayload(serverPub)
	if err != nil {
		err = fmt.Errorf("failed to setup encryption function: %w", err)
		return
	}
	daemon.cfg.setDefaults()

	// Stage 3 - Output Manager
	outMgrConf := &out.ManagerConfig{
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
	for i := 0; i < daemon.cfg.MinOutputs; i++ {
		_ = daemon.Mgrs.Out.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d output instance(s) started successfully\n", daemon.cfg.MinOutputs)

	// Stage 2 - Assembler Manager
	pkgMgrConf := &packaging.ManagerConfig{
		MinQueueCapacity:       daemon.cfg.MinAssemblerQueueSize,
		MaxQueueCapacity:       daemon.cfg.MaxAssemblerQueueSize,
		OverrideMaxPayloadSize: daemon.cfg.OverrideMaxPayloadSize,
		DestinationIP:          daemon.cfg.DestinationIP,
	}
	pkgMgrConf.MinInstanceCount.Store(uint32(daemon.cfg.MinAssemblers))
	pkgMgrConf.MaxInstanceCount.Store(uint32(daemon.cfg.MaxAssemblers))
	daemon.Mgrs.Assem, err = pkgMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Out.InQueue)
	if err != nil {
		err = fmt.Errorf("error creating new assembly instance manager: %w", err)
		return
	}

	// Stage 2 - Assembler Instance
	for i := 0; i < daemon.cfg.MinAssemblers; i++ {
		_ = daemon.Mgrs.Assem.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d assembler instance(s) started successfully\n", daemon.cfg.MinAssemblers)

	// Stage 1 - Listeners(Readers)
	daemon.Mgrs.In = ingest.NewManager(daemon.ctx, daemon.Mgrs.Assem.InQueue)
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
		"ingest instance started successfully\n")

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

	logctx.LogStdInfo(daemon.ctx, "Startup complete (%s). Sending messages to %s:%d\n",
		global.ProgVersion, daemon.cfg.DestinationIP, daemon.cfg.DestinationPort)
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
	logctx.LogStdInfo(daemon.ctx, "Daemon shutdown started (%s)...\n", global.ProgVersion)

	// Stop metric server
	if daemon.cfg.MetricQueryServerEnabled {
		if daemon.MetricServer != nil {
			err := daemon.MetricServer.Shutdown(daemon.ctx)
			if err != nil && err != http.ErrServerClosed {
				logctx.LogStdWarn(daemon.ctx, "metric HTTP server did not shutdown gracefully: %w\n", err)
			}
		}
	}

	// Stop ingest instances
	if daemon.Mgrs.In != nil {
		for filename := range daemon.Mgrs.In.FileSources {
			err := daemon.Mgrs.In.RemoveFileInstance(filename)
			if err != nil {
				logctx.LogStdWarn(daemon.ctx, "ingest file worker shutdown failed: %w\n", err)
			}
		}
		if daemon.Mgrs.In.JournalSource != nil {
			err := daemon.Mgrs.In.RemoveJrnlInstance()
			if err != nil {
				logctx.LogStdWarn(daemon.ctx, "ingest journal worker shutdown failed: %w\n", err)
			}
		}
	}

	// Stop assemblers
	if daemon.Mgrs.Assem != nil {
		queue := daemon.Mgrs.Assem.InQueue.ActiveWrite.Load()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogStdWarn(daemon.ctx, "assembler inbox queue did not empty in time: dropped %d messages\n", last)
		}
		for instanceID := range daemon.Mgrs.Assem.Instances {
			daemon.Mgrs.Assem.RemoveInstance(instanceID)
		}
	}

	// Stop output workers
	if daemon.Mgrs.Out != nil {
		queue := daemon.Mgrs.Out.InQueue.ActiveWrite.Load()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogStdWarn(daemon.ctx, "output inbox queue did not empty in time: dropped %d messages\n", last)
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
		logctx.LogStdInfo(daemon.ctx, "Daemon shutdown completed successfully\n")
	case <-time.After(ShutdownTimeout):
		logctx.LogStdWarn(daemon.ctx, "Timeout: send daemon did not shutdown within %v seconds",
			ShutdownTimeout.Seconds())
		return
	}
}
