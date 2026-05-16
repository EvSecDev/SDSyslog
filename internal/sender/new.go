package sender

import (
	"context"
	"fmt"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/parsing"
	"sdsyslog/pkg/crypto/registry"
	"slices"
	"time"
)

// Create new sending daemon instance
func NewDaemon(globalCtx context.Context, dryRun bool) (new *Daemon) {
	// New context for the daemon
	daemonCtx, daemonCancel := context.WithCancel(globalCtx)
	daemonCtx = context.WithValue(daemonCtx, global.CtxModeKey, globalCtx.Value(global.CtxModeKey))

	// Top level tag for daemon logs (avoid duplicates)
	currentTags := logctx.GetTagList(daemonCtx)
	if !slices.Equal(currentTags, []string{logctx.NSSend}) {
		daemonCtx = logctx.AppendCtxTag(daemonCtx, logctx.NSSend)
	}

	new = &Daemon{
		dryRun: dryRun,
		cfg:    Config{},
		opts:   JSONOptions{},
		ctx:    daemonCtx,
		cancel: daemonCancel,
	}
	return
}

// Sets up daemon prior to start
func (daemon *Daemon) Init(serverPub []byte) (err error) {
	daemon.startTime = time.Now()
	logctx.LogStdInfo(daemon.ctx, "Starting new daemon (%s)...\n", global.ProgVersion)

	// Signatures
	if daemon.opts.SigningKeyFile != "" {
		daemon.cfg.signingPrivateKey, err = loadSigningKey(daemon.opts.SigningKeyFile)
		if err != nil {
			err = fmt.Errorf("failed to decode signing key: %w", err)
			return
		}
	}

	// Input settings
	err = daemon.opts.loadInputs() // Pull from file(s)
	if err != nil {
		err = fmt.Errorf("failed loading input configuration: %w", err)
		return
	}

	// Metric settings
	err = parsing.VerifyWholeDuration(time.Duration(daemon.opts.Metrics.Interval))
	if err != nil {
		err = fmt.Errorf("metric collection interval is not supported: %w", err)
		return
	}

	daemon.opts.setDefaults()

	err = wrappers.SetupEncryptInnerPayload(serverPub)
	if err != nil {
		err = fmt.Errorf("failed to setup encryption function: %w", err)
		return
	}

	if daemon.opts.SigningKeyFile == "" && daemon.opts.Crypto.SignatureSuite != registry.NoSigName {
		err = fmt.Errorf("no signing key file provided but real signature suite was specified")
		return
	}

	if len(daemon.cfg.signingPrivateKey) > 0 {
		signatureSuiteID, validName := registry.SignatureNameToID(daemon.opts.Crypto.SignatureSuite)
		if !validName {
			err = fmt.Errorf("invalid signature suite name %s", daemon.opts.Crypto.SignatureSuite)
			return
		}
		info, validID := registry.GetSignatureInfo(signatureSuiteID)
		if !validID {
			err = fmt.Errorf("invalid signature suite ID: %d", signatureSuiteID)
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

	daemon.cfg.destSocket, err = network.ParseUDPAddress(daemon.opts.Network.Address, daemon.opts.Network.Port)
	if err != nil {
		err = fmt.Errorf("invalid destination: %w", err)
		return
	}

	if daemon.opts.Network.SourceAddress != "" {
		daemon.cfg.sourceSocket, err = network.ParseUDPAddress(daemon.opts.Network.SourceAddress, daemon.opts.Network.SourcePort)
		if err != nil {
			err = fmt.Errorf("invalid source: %w", err)
			return
		}
	} else {
		daemon.cfg.sourceSocket, err = network.GetLocalIPForDestination(daemon.cfg.destSocket.IP)
		if err != nil {
			err = fmt.Errorf("failed to find local address for destination: %w", err)
			return
		}
	}

	daemon.initSuccess = true
	return
}
