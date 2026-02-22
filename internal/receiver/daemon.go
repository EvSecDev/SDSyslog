// Daemon for continuous reception of log messages, processing of messages, and delivery to configured output destinations
package receiver

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sdsyslog/internal/atomics"
	"sdsyslog/internal/crypto/ecdh"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/ebpf"
	"sdsyslog/internal/externalio/server"
	"sdsyslog/internal/global"
	"sdsyslog/internal/lifecycle"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/managers/defrag"
	"sdsyslog/internal/receiver/managers/in"
	"sdsyslog/internal/receiver/managers/out"
	"sdsyslog/internal/receiver/managers/proc"
	"sdsyslog/internal/receiver/metrics"
	"sdsyslog/internal/receiver/scaling"
	"sdsyslog/internal/receiver/shard/fiprrecv"
	"sdsyslog/internal/syslog"
	"strings"
	"time"
)

// Create new receiver daemon instance
func NewDaemon(cfg Config) (new *Daemon) {
	new = &Daemon{
		cfg: cfg,
	}
	return
}

// Starts pipeline worker threads in background - gracefully shuts down if startup error is encountered
func (daemon *Daemon) Start(globalCtx context.Context, serverPriv []byte) (err error) {
	// New context for the daemon
	daemon.ctx, daemon.cancel = context.WithCancel(globalCtx)
	daemon.ctx = context.WithValue(daemon.ctx, global.CtxModeKey, globalCtx.Value(global.CtxModeKey))
	daemon.ctx = context.WithValue(daemon.ctx, global.LoggerKey, logctx.GetLogger(globalCtx))

	// Top level tag for daemon logs
	daemon.ctx = logctx.AppendCtxTag(daemon.ctx, global.NSRecv)

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog, "Starting new daemon (%s)...\n", global.ProgVersion)

	// Pre-startup
	syslog.InitBidiMaps()
	serverPub, err := ecdh.DerivePersistentPublicKey(serverPriv)
	if err != nil {
		return
	}
	err = wrappers.SetupEncryptInnerPayload(serverPub) // Specifically for shard FIPR
	if err != nil {
		err = fmt.Errorf("failed to setup encryption function: %w", err)
		return
	}
	err = wrappers.SetupDecryptInnerPayload(serverPriv)
	if err != nil {
		err = fmt.Errorf("failed to setup decryption function: %w", err)
		return
	}
	err = wrappers.SetupGetSharedSecret(serverPriv)
	if err != nil {
		err = fmt.Errorf("failed to setup shared secret function: %w", err)
		return
	}
	daemon.cfg.setDefaults()

	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		err = fmt.Errorf("failed to determine local boot id: %w", err)
		return
	}
	global.SetBootID(strings.TrimSpace(string(data)))

	// Listener socket helper - kernel-side of socket drain feature
	err = ebpf.LoadProgram()
	if err != nil {
		err = fmt.Errorf("failed to load listener helper: %w", err)
		return
	}

	// Stage 4 - Output Manager
	daemon.Mgrs.Output, err = out.NewInstanceManager(daemon.ctx, daemon.cfg.MinOutputQueueSize, daemon.cfg.MaxOutputQueueSize)
	if err != nil {
		err = fmt.Errorf("failed creating output instance manager: %w", err)
		return
	}
	err = daemon.Mgrs.Output.AddInstance(daemon.cfg.OutputFilePath, daemon.cfg.JournaldURL, daemon.cfg.BeatsEndpoint)
	if err != nil {
		err = fmt.Errorf("failed starting output: %w", err)
		return
	}
	logctx.LogEvent(daemon.ctx, global.VerbosityProgress, global.InfoLog,
		"output instance started successfully\n")

	// Stage 3 - Shard+Assembler
	daemon.Mgrs.Defrag = defrag.NewInstanceManager(daemon.ctx,
		daemon.Mgrs.Output.Queue,
		daemon.cfg.MinDefrags,
		daemon.cfg.MaxDefrags)
	for i := 0; i < daemon.cfg.MinDefrags; i++ {
		_ = daemon.Mgrs.Defrag.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, global.VerbosityProgress, global.InfoLog,
		"%d defrag instance(s) started successfully\n", daemon.cfg.MinDefrags)

	// Stage 2.9 - FIPR receiver (optional - only started under temp process during updates)
	lifecycle.TempChildActions(daemon.ctx, daemon)

	// Stage 2 - Processor
	daemon.Mgrs.Proc, err = proc.NewInstanceManager(daemon.ctx,
		daemon.Mgrs.Defrag.RoutingView,
		daemon.cfg.MinProcessors,
		daemon.cfg.MaxProcessors,
		daemon.cfg.MinProcessorQueueSize,
		daemon.cfg.MaxProcessorQueueSize)
	if err != nil {
		err = fmt.Errorf("failed adding new processor manager: %w", err)
		daemon.Shutdown()
		return
	}
	for i := 0; i < daemon.cfg.MinProcessors; i++ {
		daemon.Mgrs.Proc.AddInstance()
	}
	logctx.LogEvent(daemon.ctx, global.VerbosityProgress, global.InfoLog,
		"%d processor instance(s) started successfully\n", daemon.cfg.MinProcessors)

	// Stage 1 - Listener
	daemon.Mgrs.Input = in.NewInstanceManager(daemon.ctx,
		daemon.cfg.ListenPort,
		daemon.Mgrs.Proc.Inbox,
		daemon.cfg.MinListeners,
		daemon.cfg.MaxListeners)
	for i := 0; i < daemon.cfg.MinListeners; i++ {
		_, err = daemon.Mgrs.Input.AddInstance()
		if err != nil {
			err = fmt.Errorf("failed adding new listener instance: %w", err)
			daemon.Shutdown()
			return
		}
	}
	logctx.LogEvent(daemon.ctx, global.VerbosityProgress, global.InfoLog,
		"%d listener instance(s) started successfully\n", daemon.cfg.MinProcessors)

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
	daemon.MetricAggregator = daemon.metricsCollector.Registry.Aggregate

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
	lifecycle.PostUpdateActions(daemon.ctx, daemon)
	err = lifecycle.NotifyReady(daemon.ctx)
	if err != nil {
		err = fmt.Errorf("error sending readiness to systemd: %w", err)
		daemon.Shutdown()
		return
	}

	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog,
		"Startup complete (%s). Listening for messages on %s:%d\n", global.ProgVersion, daemon.cfg.ListenIP, daemon.cfg.ListenPort)
	return
}

// Dedicated entry point for starting inter-process fragment routing
func (daemon *Daemon) StartFIPR() (err error) {
	daemon.fipr = fiprrecv.New(daemon.ctx, global.DefaultSocketDir, daemon.Mgrs.Defrag.RoutingView)
	daemon.Mgrs.FIPR = daemon.fipr
	err = daemon.fipr.Start()
	if err != nil {
		err = fmt.Errorf("failed to start FIPR receiver: %w\n", err)
		return
	}
	return
}

// Dedicated entry point for stopping inter-process fragment routing (while daemon is still running)
func (daemon *Daemon) StopFIPR() {
	if daemon.fipr == nil {
		return
	}

	// After the packet deadline, there should be no more existing fragments that we could consume from other processes
	// Assuming other processes are already killed.
	currentPacketDeadline := daemon.Mgrs.Defrag.PacketDeadline.Load()
	drainingPeriod := time.Duration(currentPacketDeadline)
	time.Sleep(drainingPeriod)

	daemon.fipr.Stop()
	daemon.Mgrs.FIPR = nil
	daemon.fipr = nil
}

// Blocking daemon waiter
func (daemon *Daemon) Run() {
	// Block on signals only
	lifecycle.SignalHandler(daemon.ctx, daemon)
}

// Gracefully shutdown pipeline worker threads (errors are printed to program log buffer)
func (daemon *Daemon) Shutdown() {
	logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.InfoLog,
		"Daemon shutdown started (%s)...\n", global.ProgVersion)

	// Stop metric server
	if daemon.cfg.MetricQueryServerEnabled {
		err := daemon.MetricServer.Shutdown(daemon.ctx)
		if err != nil && err != http.ErrServerClosed {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"metric HTTP server did not shutdown gracefully: %w\n", err)
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
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
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
		for {
			removedID := daemon.Mgrs.Defrag.RemoveOldestInstance()
			if removedID == "" {
				break
			}
		}
	}

	// Stop Fragment Inter-Process Routing
	daemon.StopFIPR()

	// Stop output worker
	if daemon.Mgrs.Output != nil {
		queue := daemon.Mgrs.Output.Queue.ActiveWrite.Load()

		// Wait here before shutting down (active write always has newest data)
		success, last := atomics.WaitUntilZero(&queue.Metrics.Depth, 10*time.Second)
		if !success {
			logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
				"output inbox queue did not empty in time: dropped %d messages\n", last)
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
		logctx.LogEvent(daemon.ctx, global.VerbosityStandard, global.WarnLog,
			"Timeout: receive daemon did not shutdown within %v seconds",
			global.ReceiveShutdownTimeout.Seconds())
		return
	}
}
