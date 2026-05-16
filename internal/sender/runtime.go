// Daemon for continuous sending of log messages from configured sources, encryption of messages, and delivery to configured network destinations
package sender

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules/internallogger"
	"sdsyslog/internal/lifecycle"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics/server"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/sender/assembler"
	"sdsyslog/internal/sender/ingest"
	"sdsyslog/internal/sender/metrics"
	"sdsyslog/internal/sender/output"
	"sdsyslog/internal/sender/scaling"
	"strconv"
	"time"
)

// Starts pipeline worker threads in background - gracefully shuts down if startup error is encountered
func (daemon *Daemon) Start() (err error) {
	if !daemon.initSuccess {
		err = fmt.Errorf("daemon initialization was not called, refusing to start")
		return
	}

	if daemon.dryRun {
		logctx.LogStdInfo(daemon.ctx, "Configuration test successful, exiting.\n")
		return
	}

	// Stage 3 - Output Manager
	outMgrConf := &output.ManagerConfig{
		MinQueueCapacity: daemon.opts.AutoScaling.MinOutputQueueSize,
		MaxQueueCapacity: daemon.opts.AutoScaling.MaxOutputQueueSize,
		SourceAddress:    daemon.cfg.sourceSocket,
		DestAddress:      daemon.cfg.destSocket,
	}
	outMgrConf.MinInstanceCount.Store(uint32(daemon.opts.AutoScaling.MinOutputs))
	outMgrConf.MaxInstanceCount.Store(uint32(daemon.opts.AutoScaling.MaxOutputs))
	daemon.Mgrs.Out, err = outMgrConf.NewManager(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("error creating new output instance manager: %w", err)
		return
	}

	// Stage 3 - Output Instances
	for i := 0; i < int(daemon.opts.AutoScaling.MinOutputs); i++ {
		_ = daemon.Mgrs.Out.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d output instance(s) started successfully\n", daemon.opts.AutoScaling.MinOutputs)

	// Stage 2 - Assembler Manager
	pkgMgrConf := &assembler.ManagerConfig{
		MinQueueCapacity:       daemon.opts.AutoScaling.MinAssemblerQueueSize,
		MaxQueueCapacity:       daemon.opts.AutoScaling.MaxAssemblerQueueSize,
		OverrideMaxPayloadSize: daemon.opts.Network.OverrideMaxPayloadSize,
		DestinationIP:          daemon.cfg.destSocket.IP.String(),
		CryptoSuiteName:        daemon.opts.Crypto.TransportSuite,
		SigSuiteName:           daemon.opts.Crypto.SignatureSuite,
	}
	pkgMgrConf.MinInstanceCount.Store(uint32(daemon.opts.AutoScaling.MinAssemblers))
	pkgMgrConf.MaxInstanceCount.Store(uint32(daemon.opts.AutoScaling.MaxAssemblers))
	daemon.Mgrs.Assem, err = pkgMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Out.InQueue)
	if err != nil {
		err = fmt.Errorf("error creating new assembly instance manager: %w", err)
		return
	}

	// Stage 2 - Assembler Instance
	for i := 0; i < int(daemon.opts.AutoScaling.MinAssemblers); i++ {
		_ = daemon.Mgrs.Assem.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d assembler instance(s) started successfully\n", daemon.opts.AutoScaling.MinAssemblers)

	// Swap internal logger to assembler if requested
	if daemon.opts.Inputs.SendInternalLogs {
		daemon.Mgrs.LogInjector, err = internallogger.NewSenderInjector(daemon.ctx, daemon.Mgrs.Assem.InQueue)
		if err != nil {
			err = fmt.Errorf("failed creating internal logger pipeline injector: %w", err)
			daemon.Shutdown()
			return
		}
		daemon.Mgrs.LogInjector.Start()
	}

	// Stage 1 - Listeners(Readers)
	inMgrConf := ingest.ManagerConfig{
		SourceDropFilters: daemon.opts.Inputs.DropFilters,
	}
	daemon.Mgrs.In, err = inMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Assem.InQueue)
	if err != nil {
		err = fmt.Errorf("error creating new ingest instance manager: %w", err)
		return
	}
	if len(daemon.opts.Inputs.FilePaths) > 0 {
		for _, filePath := range daemon.opts.Inputs.FilePaths {
			err = daemon.Mgrs.In.AddFileInstance(filePath, daemon.opts.State.BaseFile)
			if err != nil {
				err = fmt.Errorf("failed adding new file ingest instance: %w", err)
				daemon.Shutdown()
				return
			}
		}

		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"%d file ingest instance started successfully\n", len(daemon.opts.Inputs.FilePaths))
	}

	if daemon.opts.Inputs.JournalEnabled {
		err = daemon.Mgrs.In.AddJrnlInstance(daemon.opts.State.BaseFile)
		if err != nil {
			err = fmt.Errorf("failed creating journal ingest instance: %w", err)
			daemon.Shutdown()
			return
		}
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"1 journal ingest instance started successfully\n")
	}
	if daemon.RawInput != nil {
		err = daemon.Mgrs.In.AddRawInstance(daemon.RawInput)
		if err != nil {
			err = fmt.Errorf("failed creating raw ingest instance: %w", err)
			daemon.Shutdown()
			return
		}
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"1 raw ingest instance started successfully\n")
	}

	// Metrics Collector
	daemon.metricsCollector = metrics.New(daemon.Mgrs.In,
		daemon.Mgrs.Assem,
		daemon.Mgrs.Out,
		time.Duration(daemon.opts.Metrics.Interval),
		time.Duration(daemon.opts.Metrics.MaxAge))
	workerCtx := daemon.ctx
	daemon.wg.Go(func() {
		daemon.metricsCollector.Run(workerCtx)
	})
	daemon.MetricDataSearcher = daemon.metricsCollector.Registry.Search
	daemon.MetricDiscoverer = daemon.metricsCollector.Registry.Discover
	daemon.MetricAggregator = daemon.metricsCollector.Registry.Aggregate

	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"Metric collection instance started successfully\n")

	// Autoscaler
	if daemon.opts.AutoScaling.Enabled {
		scaler := scaling.New(daemon.metricsCollector.Registry,
			time.Duration(daemon.opts.AutoScaling.PollInterval),
			daemon.Mgrs,
			runtime.NumCPU())
		workerCtx := daemon.ctx
		daemon.wg.Go(func() {
			scaler.Run(workerCtx)
		})
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Autoscaler instance started successfully\n")
	}

	// Metric Server
	if daemon.opts.Metrics.EnableQueryServer {
		// Top level tag for metric server logs (copy so return doesn't strip ns tags)
		serverCtx := daemon.ctx
		serverCtx = logctx.AppendCtxTag(serverCtx, logctx.NSMetric)
		serverCtx = logctx.AppendCtxTag(serverCtx, logctx.NSMetricSrv)

		daemon.MetricServer, err = server.SetupListener(serverCtx,
			daemon.opts.Metrics.QueryServerPort,
			daemon.MetricDataSearcher,
			daemon.MetricDiscoverer,
			daemon.MetricAggregator)
		if err != nil {
			err = fmt.Errorf("failed creating HTTP metric server: %w", err)
			daemon.Shutdown()
			return
		}

		daemon.wg.Go(func() {
			server.Start(serverCtx, daemon.MetricServer)
		})
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

	sourceAddressParsed := net.JoinHostPort(daemon.cfg.sourceSocket.IP.String(), strconv.Itoa(daemon.cfg.sourceSocket.Port))
	destAddressParsed := net.JoinHostPort(daemon.cfg.destSocket.IP.String(), strconv.Itoa(daemon.cfg.destSocket.Port))

	startupElapsed := parsing.TrimDurationPrecision(time.Since(daemon.startTime), 2)
	logctx.LogStdInfo(daemon.ctx, "Startup complete in %s (%s)\n",
		startupElapsed, global.ProgVersion)
	logctx.LogStdInfo(daemon.ctx, "Sending messages from %s to %s\n",
		sourceAddressParsed, destAddressParsed)
	daemon.startSuccess = true
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
	if !daemon.startSuccess {
		// No signal handler, just exit
		return
	}

	// Block on signals only
	lifecycle.SignalHandler(daemon.ctx, daemon)
}

// Gracefully shutdown pipeline worker threads
func (daemon *Daemon) Shutdown() {
	shutdownTime := time.Now()
	logctx.LogStdInfo(daemon.ctx, "Daemon shutdown started (%s)...\n", global.ProgVersion)

	// Stop metric server
	if daemon.opts.Metrics.EnableQueryServer {
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
		if daemon.Mgrs.In.RawSource != nil {
			err := daemon.Mgrs.In.RemoveRawInstance()
			if err != nil {
				logctx.LogStdWarn(daemon.ctx, "ingest raw worker shutdown failed: %w\n", err)
			} else {
				logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
					"Successfully stopped ingest raw instance\n")
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

	// Stop internal logs injector
	if daemon.opts.Inputs.SendInternalLogs {
		logger := logctx.GetLogger(daemon.ctx)
		logger.SetFormattedOutput(os.Stdout) // Start writing to stdout again
		daemon.Mgrs.LogInjector.Stop()       // Stop pipeline injector and unset raw logger output
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
