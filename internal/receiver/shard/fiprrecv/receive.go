package fiprrecv

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime/debug"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/receiver/shard"
	"sdsyslog/pkg/fipr"
	"sdsyslog/pkg/protocol"
	"sync"
	"time"
)

func (instance *Instance) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			instance.wgConn.Wait()
			return
		default:
		}

		conn, err := instance.listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				logctx.LogStdErr(ctx,
					"failed accepting connection: %w\n", err)
			}
			continue
		}
		instance.Metrics.Connections.Add(1)
		instance.wgConn.Add(1)
		go instance.handleConnection(ctx, &instance.wgConn, conn)
	}
}

func (instance *Instance) handleConnection(ctx context.Context, wg *sync.WaitGroup, conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			logctx.LogStdErr(ctx, "failed closing connection: %w\n", err)
		}
		wg.Done()
	}()
	defer func() {
		// Record panics and continue listening
		if fatalError := recover(); fatalError != nil {
			stack := debug.Stack()
			logctx.LogStdErr(ctx,
				"panic in shard inter-process fragment instance receiver thread: %v\n%s", fatalError, stack)
			return
		}
	}()

	session, err := fipr.New(conn, instance.hmacSecret)
	if err != nil {
		logctx.LogStdErr(ctx,
			"session creation failed: %w\n", err)
		return
	}

	err = session.WaitStart()
	if err != nil {
		logctx.LogStdErr(ctx,
			"failed handshake with new client: %w\n", err)
		return
	}

	err = session.WaitOnBehalfOf()
	if err != nil {
		logctx.LogStdErr(ctx,
			"error waiting for original sender address: %w\n", err)
		return
	}

	origAddr, err := netip.ParseAddr(session.OriginalSender())
	if err != nil {
		logctx.LogStdErr(ctx, "invalid original sender address received from remote: %w\n", err)
		return
	}

	err = session.WaitShardCheck()
	if err != nil {
		logctx.LogStdErr(ctx,
			"error waiting for shard check: %w\n", err)
		return
	}

	var running, draining bool
	if instance.localRoutingView == nil {
		// Treat as shutdown
		running = false
		draining = false
	} else {
		running = true
		// For our purposes, we should only ever accept existing fragments
		draining = true
	}
	err = session.SendShardStatus(running, draining)
	if err != nil {
		logctx.LogStdErr(ctx,
			"error sending shard status: %w\n", err)
		return
	}
	if !running {
		// Transaction complete - enforcing 'client cannot do anything with a shutdown shard'
		return
	}

	err = session.WaitMessageCheck()
	if err != nil {
		logctx.LogStdErr(ctx,
			"error waiting for message check: %w\n", err)
		return
	}

	exists := instance.localRoutingView.BucketExistsAnywhere(session.MessageID())
	err = session.SendMessageStatus(exists)
	if err != nil {
		logctx.LogStdErr(ctx,
			"error preparing message status: %w\n", err)
		return
	}

	rawFragment, err := session.WaitFragment()
	if err != nil {
		if errors.Is(err, fipr.ErrTransportWasClosed) {
			// Client decided not to route and closed connection - no error
			return
		}
		logctx.LogStdErr(ctx,
			"error waiting for fragment: %w\n", err)
		return
	}

	fragProcessingStartTime := time.Now()
	payload, err := protocol.DeconstructInnerPayload(rawFragment)
	if err != nil {
		err = fmt.Errorf("error deserializing fragment: %w", err)

		lerr := session.SendReject()
		if lerr != nil && lerr != fipr.ErrSessionClosed {
			logctx.LogStdErr(ctx,
				"failed encoding reject response frame: %w (original error: %w)\n", lerr, err)
		} else {
			logctx.LogStdErr(ctx, "%w\n", err)
		}

		instance.Metrics.RejectedFragments.Add(1)
		return
	}
	fragment, err := protocol.DeconstructPayload(payload)
	if err != nil {
		err = fmt.Errorf("error validating fragment: %w", err)

		lerr := session.SendReject()
		if lerr != nil && lerr != fipr.ErrSessionClosed {
			logctx.LogStdErr(ctx,
				"failed encoding reject response frame: %w (original error: %w)\n", lerr, err)
		} else {
			logctx.LogStdErr(ctx, "%w\n", err)
		}

		instance.Metrics.RejectedFragments.Add(1)
		return
	}

	// Push to local shard router - using new processing start time for bucket deadline
	success := shard.RouteFragment(ctx, instance.localRoutingView, origAddr, fragment, fragProcessingStartTime)
	if !success {
		err = fmt.Errorf("failed to route fragment to shard queue")

		lerr := session.SendReject()
		if lerr != nil && lerr != fipr.ErrSessionClosed {
			logctx.LogStdErr(ctx,
				"failed to send rejection: %w (original error: %w)\n", lerr, err)
		} else {
			logctx.LogStdErr(ctx, "%w\n", err)
		}

		instance.Metrics.RejectedFragments.Add(1)
		return
	}

	err = session.SendAccept()
	if err != nil {
		logctx.LogStdErr(ctx,
			"error sending accept message: %w\n", err)
	}

	instance.Metrics.AcceptedFragments.Add(1)
}
