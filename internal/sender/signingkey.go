package sender

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sdsyslog/internal/global"
	"sdsyslog/internal/lifecycle"
)

// Overwrites JSON config signing key and writes back to config file
func WriteNewSigningKey(configPath string, jsonCfg JSONConfig) (err error) {
	if jsonCfg.SigningKeyFile != "" {
		// No-op when existing key is present
		fmt.Printf("Existing signing key already defined in %q: not overwriting\n", jsonCfg.SigningKeyFile)
		return
	}

	keyBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return
	}

	keyBytes = bytes.TrimSpace(keyBytes)
	keyBytes = bytes.Trim(keyBytes, "\n")

	newSigningKey, err := base64.StdEncoding.DecodeString(string(keyBytes))
	if err != nil {
		err = fmt.Errorf("failed to decode signing key base64: %w", err)
		return
	}

	jsonCfg.SigningKeyFile = global.DefaultSendSigningKey

	_, err = os.Stat(jsonCfg.SigningKeyFile)
	if err != nil && !os.IsNotExist(err) {
		err = fmt.Errorf("unable to check existence of existing signing key file: %w", err)
		return
	}
	if err == nil {
		fmt.Printf("Existing signing key file already present at %q: not overwriting\n", jsonCfg.SigningKeyFile)
		return
	}

	// Re-encode for storage in file
	encodedSigningKey := base64.StdEncoding.EncodeToString(newSigningKey)

	// Signing private key resides in dedicated file
	err = os.WriteFile(jsonCfg.SigningKeyFile, []byte(encodedSigningKey), 0600)
	if err != nil {
		err = fmt.Errorf("failed to write new signing key file: %w", err)
		return
	}

	// Update main config to point to signing key file
	newConfig, err := json.MarshalIndent(jsonCfg, "", "  ")
	if err != nil {
		err = fmt.Errorf("failed to marshal updated main config: %w", err)
		return
	}
	err = os.WriteFile(configPath, newConfig, 0600)
	if err != nil {
		err = fmt.Errorf("failed to write updated main config file: %w", err)
		return
	}

	// Find and issue reload signal to running receiver daemon
	err = lifecycle.IssueLiveSigningKeyReload(configPath, os.Args[0])
	if err != nil {
		err = fmt.Errorf("failed live reload of new signing key: %w", err)
		return
	}
	return
}
