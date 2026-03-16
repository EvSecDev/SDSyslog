package receiver

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sdsyslog/internal/crypto/certificate"
	"sdsyslog/internal/global"
	"sdsyslog/internal/lifecycle"
	"strings"
)

// Adds a sender pinned key to configuration file.
// addRequest in format of <hostname><PinedKeysReqSeparator><base64 public key|pem file path|HTTPs URL>
// Only one hostname is allowed in pinned keys, if it already exists, the public key will be overridden with supplied key.
// If the pinned key map JSON file (separate from main config) does not exist, it will be created and main config will be updated to point to its path.
func AddPinnedKey(confPath, addRequest string) (err error) {
	if addRequest == "" {
		err = fmt.Errorf("pinned key add request (hostname+key) cannot be empty")
		return
	}
	if confPath == "" {
		err = fmt.Errorf("receiver configuration file must be specified to add a pinned sender key")
		return
	}
	cfg, err := LoadConfig(confPath)
	if err != nil {
		err = fmt.Errorf("failed to load main config file: %w", err)
		return
	}

	// Parse request
	fields := strings.Split(addRequest, PinedKeysReqSeparator)
	if len(fields) != 2 {
		err = fmt.Errorf("key add request must be in format <hostname>%s<key>", PinedKeysReqSeparator)
		return
	}
	hostname := fields[0]
	if hostname == "" {
		err = fmt.Errorf("key add request must have hostname string before '%s' symbol", PinedKeysReqSeparator)
		return
	}
	keyLocation := fields[1]
	if keyLocation == "" {
		err = fmt.Errorf("key add request must have key location or exact key after '%s' symbol", PinedKeysReqSeparator)
		return
	}

	publicKey, err := retrieveKeyBytes(keyLocation)
	if err != nil {
		return
	}

	var pinKeyFileMissing bool
	if cfg.PinnedSigningKeysPath == "" {
		pinKeyFileMissing = true
	} else {
		_, lerr := os.Stat(cfg.PinnedSigningKeysPath)
		if lerr != nil && os.IsNotExist(lerr) {
			pinKeyFileMissing = true
		}
	}

	var pinnedKeys map[string][]byte

	if !pinKeyFileMissing {
		// Read in existing map
		pinKeyFile, lerr := os.ReadFile(cfg.PinnedSigningKeysPath)
		if lerr != nil {
			err = fmt.Errorf("failed reading pinned key JSON file: %w", lerr)
			return
		}
		err = json.Unmarshal(pinKeyFile, &pinnedKeys)
		if err != nil {
			err = fmt.Errorf("invalid pinned keys JSON format: %w", err)
			return
		}

		pinnedKeys[hostname] = publicKey
	}
	if pinKeyFileMissing {
		// Brand new map
		pinnedKeys = make(map[string][]byte)
		pinnedKeys[hostname] = publicKey

		confDir := filepath.Dir(confPath)
		defaultKeyFile := filepath.Base(global.DefaultConfigPinKeys)
		cfg.PinnedSigningKeysPath = filepath.Join(confDir, defaultKeyFile)
	}

	// Write updated map back to pinned keys config
	newPinKeyFile, err := json.MarshalIndent(pinnedKeys, "", "  ")
	if err != nil {
		err = fmt.Errorf("failed to marshal new pinned key map: %w", err)
		return
	}
	err = os.WriteFile(cfg.PinnedSigningKeysPath, newPinKeyFile, 0600)
	if err != nil {
		err = fmt.Errorf("failed to write new pinned keys: %w", err)
		return
	}
	// When missing from main config, update main config
	if pinKeyFileMissing {
		var newConfFile []byte
		newConfFile, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			err = fmt.Errorf("failed to marshal updated config: %w", err)
			return
		}
		err = os.WriteFile(confPath, newConfFile, 0600)
		if err != nil {
			err = fmt.Errorf("failed to write updated config: %w", err)
			return
		}
	}

	// Find and issue reload signal to running receiver daemon
	err = lifecycle.IssueLiveSigningKeyReload(confPath, os.Args[0])
	if err != nil {
		err = fmt.Errorf("failed live reload of new pinned keys: %w", err)
		return
	}
	return
}

// Retrieves public key bytes for given location (can be base64 public key, PEM file path, HTTPs URL)
func retrieveKeyBytes(location string) (publicKey []byte, err error) {
	publicKey, lerr := base64.StdEncoding.DecodeString(location)
	if lerr == nil {
		// Direct key
		return
	}

	_, lerr = os.Stat(location)
	if lerr == nil {
		// PEM (maybe)
		var algo x509.PublicKeyAlgorithm
		publicKey, algo, err = certificate.LoadPublicKeyFile(location)
		if err != nil {
			err = fmt.Errorf("failed retrieving public key: %w", err)
			return
		}
		if algo != x509.Ed25519 {
			err = fmt.Errorf("file at %q is not ed25519", location)
			return
		}
		return
	}

	// Unknown
	err = fmt.Errorf("unknown location type %q: cannot determine if base64 or path", location)
	return
}

// Removes a sender pinned key in configuration file.
// Only one hostname is allowed in pinned keys, so the entry corresponding to the given hostname will be removed.
// Returns nil if it doesn't exist in the config, target pin key file doesn't exist, or pinned key map inside file is empty.
func RemovePinnedKey(confPath, removeHostname string) (err error) {
	if removeHostname == "" {
		err = fmt.Errorf("hostname to remove cannot be empty")
		return
	}
	if confPath == "" {
		err = fmt.Errorf("receiver configuration file must be specified to remove a pinned sender key")
		return
	}
	cfg, err := LoadConfig(confPath)
	if err != nil {
		err = fmt.Errorf("failed loading main config: %w", err)
		return
	}
	if cfg.PinnedSigningKeysPath == "" {
		// No-op
		return
	}
	pinKeyFile, err := os.ReadFile(cfg.PinnedSigningKeysPath)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("failed reading pinned key JSON file: %w", err)
		return
	} else if err != nil && os.IsNotExist(err) {
		// No-op
		return
	}

	var pinnedKeys map[string][]byte
	err = json.Unmarshal(pinKeyFile, &pinnedKeys)
	if err != nil {
		err = fmt.Errorf("invalid pinned keys JSON format: %w", err)
		return
	}

	if len(pinnedKeys) == 0 {
		// No-op
		return
	}

	_, knownHost := pinnedKeys[removeHostname]
	if !knownHost {
		// No-op
		return
	}

	delete(pinnedKeys, removeHostname)

	// Write back to config
	newPinKeyFile, err := json.MarshalIndent(pinnedKeys, "", "  ")
	if err != nil {
		err = fmt.Errorf("failed to marshal new pinned key map: %w", err)
		return
	}
	err = os.WriteFile(cfg.PinnedSigningKeysPath, newPinKeyFile, 0600)
	if err != nil {
		err = fmt.Errorf("failed to write new pinned keys: %w", err)
		return
	}

	// Find and issue reload signal to running receiver daemon
	err = lifecycle.IssueLiveSigningKeyReload(confPath, os.Args[0])
	if err != nil {
		err = fmt.Errorf("failed live reload of new pinned keys: %w", err)
		return
	}
	return
}
