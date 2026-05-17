// Daemon for continuous reception of log messages, processing of messages, and delivery to configured output destinations
package receiver

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/global"
	"sdsyslog/internal/iomodules/internallogger"
	"sdsyslog/internal/lifecycle"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/metrics/server"
	"sdsyslog/internal/parsing"
	"sdsyslog/internal/receiver/assembler"
	"sdsyslog/internal/receiver/listener"
	"sdsyslog/internal/receiver/metrics"
	"sdsyslog/internal/receiver/output"
	"sdsyslog/internal/receiver/processor"
	"sdsyslog/internal/receiver/scaling"
	"sdsyslog/internal/receiver/shard/fiprrecv"
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

	// Listener socket helper - kernel-side of socket drain feature
	err = ebpf.LoadProgram(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("failed to load listener eBPF draining program: %w", err)
		return
	}

	// Stage 4 - Output Manager
	outMgrConf := &output.ManagerConfig{
		FilePath:                           daemon.opts.Outputs.FilePath,
		JournaldURL:                        daemon.opts.Outputs.JournaldURL,
		BeatsAddress:                       daemon.opts.Outputs.BeatsAddress,
		RawWriter:                          daemon.RawWriter,
		EnableDBUSNotify:                   daemon.opts.Outputs.DBUSNotify,
		ConsecutiveFailureShutdownInterval: time.Duration(daemon.opts.Outputs.MaxConsecutiveFailures),
		MinQueueCapacity:                   daemon.opts.AutoScaling.MinOutQueueSize,
		MaxQueueCapacity:                   daemon.opts.AutoScaling.MaxOutQueueSize,
	}
	daemon.Mgrs.Output, err = outMgrConf.NewManager(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("failed creating output instance manager: %w", err)
		return
	}

	// Stage 4 - Output Instance
	err = daemon.Mgrs.Output.AddWorkers()
	if err != nil {
		err = fmt.Errorf("failed starting output: %w", err)
		return
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"1 output instance started successfully\n")

	// Swap internal logger to output if requested
	if daemon.opts.Outputs.InternalLogs {
		daemon.Mgrs.LogInjector, err = internallogger.NewReceiverInjector(daemon.ctx, daemon.Mgrs.Output.Inbox, 1)
		if err != nil {
			err = fmt.Errorf("failed creating internal logger pipeline injector: %w", err)
			daemon.Shutdown()
			return
		}
		daemon.Mgrs.LogInjector.Start()
	}

	// Stage 3 - Shard+Assembler Manager
	dfrgMgrConf := &assembler.ManagerConfig{
		FIPRSocketDirectory: daemon.opts.State.IPCSocketDirectory,
	}
	dfrgMgrConf.MinInstanceCount.Store(uint32(daemon.opts.AutoScaling.MinDefrags))
	dfrgMgrConf.MaxInstanceCount.Store(uint32(daemon.opts.AutoScaling.MaxDefrags))
	daemon.Mgrs.Assembler, err = dfrgMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Output.Inbox)
	if err != nil {
		err = fmt.Errorf("failed creating defrag manager: %w", err)
		daemon.Shutdown()
		return
	}

	// Stage 3 - Shard+Assembler Instances
	for i := 0; i < int(daemon.opts.AutoScaling.MinDefrags); i++ {
		_ = daemon.Mgrs.Assembler.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d defrag instance(s) started successfully\n", daemon.opts.AutoScaling.MinDefrags)

	// Stage 2.9 - FIPR receiver (optional - only started under temp process during updates)
	lifecycle.TempChildActions(daemon.ctx, daemon)

	// Stage 2 - Processor Manager
	procMgrConf := &processor.ManagerConfig{
		MinQueueCapacity: daemon.opts.AutoScaling.MinProcQueueSize,
		MaxQueueCapacity: daemon.opts.AutoScaling.MaxProcQueueSize,
		PastMsgCutoff:    time.Duration(daemon.opts.ReplayProtection.PastValidityWindow),
		FutureMsgCutoff:  time.Duration(daemon.opts.ReplayProtection.FutureValidityWindow),
	}
	procMgrConf.MinInstanceCount.Store(uint32(daemon.opts.AutoScaling.MinProcessors))
	procMgrConf.MaxInstanceCount.Store(uint32(daemon.opts.AutoScaling.MaxProcessors))
	daemon.Mgrs.Proc, err = procMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Assembler.RoutingView)
	if err != nil {
		err = fmt.Errorf("failed creating processor manager: %w", err)
		daemon.Shutdown()
		return
	}

	// Stage 2 - Processor Instances
	for i := 0; i < int(daemon.opts.AutoScaling.MinProcessors); i++ {
		daemon.Mgrs.Proc.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d processor instance(s) started successfully\n", daemon.opts.AutoScaling.MinProcessors)

	// Stage 1 - Listener Manager
	inMgrConf := &listener.ManagerConfig{
		ListenSocket:           daemon.cfg.sourceSocket,
		ReplayProtectionWindow: time.Duration(daemon.opts.ReplayProtection.ProtectionWindow),
	}
	inMgrConf.MaxInstanceCount.Store(uint32(daemon.opts.AutoScaling.MaxListeners))
	inMgrConf.MinInstanceCount.Store(uint32(daemon.opts.AutoScaling.MinListeners))
	daemon.Mgrs.Input, err = inMgrConf.NewManager(daemon.ctx, daemon.Mgrs.Proc.Inbox)
	if err != nil {
		err = fmt.Errorf("failed creating listener manager: %w", err)
		daemon.Shutdown()
		return
	}

	// Stage 1 - Listener Instances
	for i := 0; i < int(daemon.opts.AutoScaling.MinListeners); i++ {
		_, err = daemon.Mgrs.Input.AddInstance()
		if err != nil {
			err = fmt.Errorf("failed adding new listener instance: %w", err)
			daemon.Shutdown()
			return
		}
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"%d listener instance(s) started successfully\n", daemon.opts.AutoScaling.MinProcessors)

	// Metrics Collector
	daemon.metricsCollector = metrics.New(daemon.Mgrs,
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
			daemon.Mgrs)
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
	lifecycle.PostUpdateActions(daemon.ctx, daemon, ShutdownTimeout)
	err = lifecycle.NotifyReady(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("error sending readiness to systemd: %w", err)
		daemon.Shutdown()
		return
	}

	parsedListenAddr := net.JoinHostPort(daemon.cfg.sourceSocket.IP.String(), strconv.Itoa(daemon.cfg.sourceSocket.Port))

	startupElapsed := parsing.TrimDurationPrecision(time.Since(daemon.startTime), 2)
	logctx.LogStdInfo(daemon.ctx, "Startup complete in %s (%s)\n",
		startupElapsed, global.ProgVersion)
	logctx.LogStdInfo(daemon.ctx, "Listening for messages on %s\n", parsedListenAddr)
	daemon.startSuccess = true
	return
}

// Dedicated entry point for starting inter-process fragment routing
// Accept fragments that ended up in other processes that are partial fragments for this process
func (daemon *Daemon) StartFIPR() (err error) {
	daemon.fipr, err = fiprrecv.New(daemon.ctx, daemon.opts.State.IPCSocketDirectory, daemon.Mgrs.Assembler.RoutingView)
	if err != nil {
		return
	}
	daemon.Mgrs.FIPR = daemon.fipr
	daemon.Mgrs.Assembler.FIPRRunning.Store(true)
	err = daemon.fipr.Start()
	if err != nil {
		err = fmt.Errorf("failed to start FIPR receiver: %w", err)
		return
	}
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"Fragment Inter-Process Router receiver instance started successfully\n")
	return
}

// Dedicated entry point for stopping inter-process fragment routing (while daemon is still running)
func (daemon *Daemon) StopFIPR() {
	if daemon.fipr == nil {
		return
	}

	// After the packet deadline, there should be no more existing fragments that we could consume from other processes
	// Assuming other processes are already killed.
	currentPacketDeadline := daemon.Mgrs.Assembler.Config.PacketDeadline.Load()
	drainingPeriod := time.Duration(currentPacketDeadline)
	time.Sleep(drainingPeriod)

	daemon.fipr.Stop()
	daemon.Mgrs.Assembler.FIPRRunning.Store(false)
	daemon.Mgrs.FIPR = nil
	daemon.fipr = nil
	logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
		"Successfully stopped Fragment Inter-Process Router receiver instance\n")
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

// Gracefully shutdown pipeline worker threads (errors are printed to program log buffer)
func (daemon *Daemon) Shutdown() {
	shutdownTime := time.Now()
	logctx.LogStdInfo(daemon.ctx, "Daemon shutdown started (%s)...\n", global.ProgVersion)

	// Stop metric server
	if daemon.opts.Metrics.EnableQueryServer && daemon.MetricServer != nil {
		err := daemon.MetricServer.Shutdown(daemon.ctx)
		if err != nil && err != http.ErrServerClosed {
			logctx.LogStdWarn(daemon.ctx, "metric HTTP server did not shutdown gracefully: %w\n", err)
		} else {
			logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
				"Metric server stopped successfully\n")
		}
	}

	// Stop listener instances
	if daemon.Mgrs.Input != nil {
		for {
			instanceList := daemon.Mgrs.Input.Instances.Load()
			if len(*instanceList) == 0 {
				break
			}
			removedID := daemon.Mgrs.Input.RemoveLastInstance()
			logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
				"Successfully stopped listener instance %d\n", removedID)
		}
	}

	// Stop processor instances
	if daemon.Mgrs.Proc != nil {
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Draining processor worker queue...\n")

		queue := daemon.Mgrs.Proc.Inbox.ActiveWrite.Load()
		queue.ResyncDepthMetric()
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogStdWarn(daemon.ctx, "assembler inbox queue did not empty in time: dropped %d messages\n", last)
		}

		for {
			instanceList := daemon.Mgrs.Proc.Instances.Load()
			if len(*instanceList) == 0 {
				break
			}
			removedID := daemon.Mgrs.Proc.RemoveLastInstance()
			logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
				"Successfully stopped processor instance %d\n", removedID)
		}
	}

	// Stop defrag and shards
	if daemon.Mgrs.Assembler != nil {
		for {
			removedID := daemon.Mgrs.Assembler.RemoveOldestInstance()
			if removedID == "" {
				break
			}
			logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
				"Successfully stopped assembler instance %s\n", removedID)
		}
	}

	// Stop Fragment Inter-Process Routing
	daemon.StopFIPR()

	// Stop internal logs injector
	if daemon.opts.Outputs.InternalLogs {
		logger := logctx.GetLogger(daemon.ctx)
		logger.SetFormattedOutput(os.Stdout) // Start writing to stdout again
		daemon.Mgrs.LogInjector.Stop()       // Stop pipeline injector and unset raw logger output
	}

	// Stop output worker
	if daemon.Mgrs.Output != nil {
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Draining output worker queue\n")

		queue := daemon.Mgrs.Output.Inbox.ActiveWrite.Load()
		queue.ResyncDepthMetric()

		// Wait here before shutting down (active write always has newest data)
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogStdWarn(daemon.ctx, "output inbox queue did not empty in time: dropped %d messages\n", last)
		}
		daemon.Mgrs.Output.RemoveWorkers()
		logctx.LogEvent(daemon.ctx, logctx.VerbosityProgress, logctx.InfoLog,
			"Successfully stopped output instance\n")
	}

	// Stop any other workers after instances are drained and stopped
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
		logctx.LogStdWarn(daemon.ctx, "Timeout: receive daemon component did not shutdown within %v seconds (total elapsed: %s)",
			ShutdownTimeout.Seconds(), parsing.TrimDurationPrecision(time.Since(shutdownTime), 2))
		return
	}
}
