package receiver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sdsyslog/internal/crypto/wrappers"
	"sdsyslog/internal/global"
	"sdsyslog/internal/logctx"
	"sdsyslog/internal/network"
	"sdsyslog/internal/parsing"
	"sdsyslog/pkg/crypto/registry"
	"slices"
	"time"
)

// Create new receiver daemon instance
func NewDaemon(globalCtx context.Context, dryRun bool) (new *Daemon) {
	// New context for the daemon
	daemonCtx, daemonCancel := context.WithCancel(globalCtx)
	daemonCtx = context.WithValue(daemonCtx, global.CtxModeKey, globalCtx.Value(global.CtxModeKey))

	// Top level tag for daemon logs (avoid duplicates)
	currentTags := logctx.GetTagList(daemonCtx)
	if !slices.Equal(currentTags, []string{logctx.NSRecv}) {
		daemonCtx = logctx.AppendCtxTag(daemonCtx, logctx.NSRecv)
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
func (daemon *Daemon) Init(serverPriv []byte) (err error) {
	daemon.startTime = time.Now()
	logctx.LogStdInfo(daemon.ctx, "Starting new daemon (%s)...\n", global.ProgVersion)

	err = parsing.VerifyWholeDuration(time.Duration(daemon.opts.Metrics.Interval))
	if err != nil {
		err = fmt.Errorf("metric collection interval is not supported: %w", err)
		return
	}

	daemon.opts.setDefaults()

	if daemon.opts.PinnedSigningKeysPath == "" && daemon.opts.Crypto.SignatureSuite != registry.NoSigName {
		err = fmt.Errorf("signing enabled but no pinned signing keys path was provided")
		return
	}

	// Load pinned keys
	if daemon.opts.PinnedSigningKeysPath != "" {
		var data []byte
		data, err = os.ReadFile(daemon.opts.PinnedSigningKeysPath)
		if err != nil {
			err = fmt.Errorf("failed to read pinned signing keys file: %w", err)
			return
		}

		var raw map[string]string
		err = json.Unmarshal(data, &raw)
		if err != nil {
			err = fmt.Errorf("invalid pinned signing keys file format: %w", err)
			return
		}

		daemon.cfg.PinnedSigningKeys = make(map[string][]byte, len(raw))
		for host, b64 := range raw {
			var key []byte
			key, err = base64.StdEncoding.DecodeString(b64)
			if err != nil {
				err = fmt.Errorf("invalid key for sender hostname %s (must be base64): %w", host, err)
				return
			}
			daemon.cfg.PinnedSigningKeys[host] = key
		}
	} else {
		daemon.cfg.PinnedSigningKeys = make(map[string][]byte)
	}

	transportSuiteID, validName := registry.SuiteNameToID(daemon.opts.Crypto.TransportSuite)
	if !validName {
		err = fmt.Errorf("invalid transport suite name %s", daemon.opts.Crypto.TransportSuite)
		return
	}
	info, validID := registry.GetSuiteInfo(transportSuiteID)
	if !validID {
		err = fmt.Errorf("invalid transport suite ID %d", transportSuiteID)
		return
	}
	err = info.ValidateKey(serverPriv)
	if err != nil {
		return
	}
	serverPub, err := info.DerivePublicKey(serverPriv)
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
	err = wrappers.SetupVerifySignature(daemon.cfg.PinnedSigningKeys)
	if err != nil {
		err = fmt.Errorf("failed to setup signature verification function: %w", err)
		return
	}
	daemon.cfg.sourceSocket, err = network.ParseUDPAddress(daemon.opts.Network.Address, daemon.opts.Network.Port)
	if err != nil {
		err = fmt.Errorf("invalid network address: %w", err)
		return
	}
	daemon.initSuccess = true
	return
}
