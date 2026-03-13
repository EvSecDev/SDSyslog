# Secure Diode Syslog (SDSyslog)

A program to securely transmit and receive log messages across untrusted and/or unidirectional network links.

This program prioritizes unidirectional networking and data confidentiality/integrity to improve upon the UDP syslog protocol (RFC5424).

For technical details about the protocol used (D3P), see `docs/Protocol.md`.
For technical details about this program's architecture, see `docs/Architecture.md`.

Warning: This program is early in its development and *does* contain bugs.

## Features

- Unidirectional network support
- Multi-packet payloads (for messages exceeding MTU of a single packet)
- Encrypted payloads
- Message input filtering via config driven filters
- Supported inputs:
  - Multiple files
  - Journald
- Supported Outputs:
  - File
  - Journald
  - Beats (lumberjack)

## Installation

Steps:

- Copy binary to the desired system
- Assume root privileges for the initial install commands
- For the receiving daemon (if applicable):
  - `./sdsyslog configure --install-receiver`
  - Modify the configuration file to your needs (`/etc/sdsyslog/sdsyslog.json"`)
  - Start the systemd service for the Receiver with `systemctl start sdsyslog`
  - Check for any errors with `journalctl -r -u sdsyslog`
- For the sending daemon (if applicable):
  - `./sdsyslog configure --install-sender`
  - Modify the configuration file to your needs (`/etc/sdsyslog/sdsyslog-sender.json"`)
  - Start the systemd service for the sender with `systemctl start sdsyslog-sender`
  - Check for any errors with `journalctl -r -u sdsyslog-sender`

## Updates

Downloading the new binary and running the installation again will update any files on disk.

It will overwrite and reload most things, but will *not* overwrite the configuration file unless requested.

To update the running daemon with zero interruption to traffic, a `SIGHUP` signal will trigger an in-place upgrade.

If running under systemd, you only need to run `systemctl reload sdsyslog`/`systemctl reload sdsyslog-sender`.

Otherwise, a standalone process upgrade can be triggered with the command `kill -HUP <PID>`.

## Uninstallation

Steps:

- WARNING: this PERMANENTLY removes the private key file, configuration file, and any state-saving files
- Assume root privileges for the uninstall commands
- For the receiving daemon:
  - `./sdsyslog configure --uninstall-receiver`
- For the sending daemon:
  - `./sdsyslog configure --uninstall-sender`

## SDSyslog Help Menu

```bash
Usage: ./sdsyslog [subcommand]

Secure Diode System Logger (SDSyslog)
  Encrypts and transfers messages over unidirectional networks

  Subcommands:
    configure   - Setup Actions
    receive     - Receive Messages
    send        - Send Messages
    version     - Show Version Information

  Options:
  -v, --verbosity  Increase detailed progress messages (Higher is more verbose) <0...5> [default: 1]

Report bugs to: dev@evsec.net
SDSyslog home page: <https://github.com/EvSecDev/SDSyslog>
General help using GNU software: <https://www.gnu.org/gethelp/>
```

## Metrics

This program generates and stores internal metrics that are useful for monitoring and diagnostics.
These metrics include:

- Queue sizes (depth and bytes)
- Pipeline stage worker performance (busy time, average/max processing time, in/out counts, etc.)

To access the internal metric registry, set `enableHTTPQueryServer` under `metrics` in the JSON configuration to `true`.

When the daemon is started, a limited HTTP server will also be started on localhost (default port is `18514`).

To get started with this API, grab the HTML docs by querying the root path `curl http://localhost:18514/` for the sender or `curl http://localhost:28514/` for the receiver.

## Host Identity Enforcement

An optional additional security feature is the per-host signatures.

This provides authentication for a given hostname and protects against forgery of other senders that possess the primary public key.

To use this feature, you can generate a key pair on the sender:

```bash
sdsyslog configure --create-signing-keys
```

Take the public key and add it to the receiver daemon:

```bash
sdsyslog receive --config /etc/sdsyslog/sdsyslog.json --trust-sender 'sender-hostname|base64-private-key=='
```

Note: if the receiver is not running or it fails, simply reload or restart the receiving daemon

Take the private key and add it to the sender daemon:

```bash
sdsyslog send --config /etc/sdsyslog/sdsyslog-sender.json --write-signing-key <<<"base64-private-key=="
```

Reload the sender daemon:

```bash
systemctl reload sdsyslog-sender.service
```

### Signing Notes

By default, without signatures, all hostnames show up in logs with a prefix of `[UNVERIFIED]`.

If the sender is configured with a signing key but not yet trusted by the receiver, the hostname will have prefix `[UNKNOWN]`.

No prefix will be present when the signature is present, trusted, and verified.

Any attempt by a sender to impersonate a hostname that is already trusted by the receiver will be dropped.

The timestamp of the log message is also included in the signing process.

## Input Filtering

The sender daemon JSON configuration has a section for filters.

Anything defined in this section will attempt to match messages from external sources and drop the message if it matches.

Use `sdsyslog configure -c example.json --send-config-template` to generate an example configuration file containing some of these filters.

## Notes

- Journal output requires the installation of `systemd-journal-remote` and uses the HTTP configuration of the socket.
  - Logs are written to their own journal file (separate from the main system journal), usually located under `/var/log/journal/remote/`.
- Beats output adds custom fields that are similar, but not the same, as other beats clients (like filebeat).
  - Added fields can be found in the source at `internal/externalio/beats/write.go`
  - Most of these fields will end up prefixed by `beat_` in third party log analysis software.
    - For example, code like below will end up as the field: `beat_log_id`

      ```go
      "log": map[string]interface{}{
        "id": msg.LogID,
      ```

- Maximum individual log message size is 4GB
- Due to address/port reuse across the program, during in-place upgrades or shutdowns, there is a slight chance of data loss between when packets are received by the system and when the program reads the data.
  - Essentially the program has no way of safely "draining" a go routines associated kernel-level socket buffer before it shuts down (for scaling down and hot swapping).
  - On non-Linux systems (or older non-eBPF Linux kernels), there is no guarantee that this program can make to *not* drop data during these events.
  - For *BSD systems, during shutdown, the program will attempt to time when the socket is empty to close a listener.
    - On listener shutdowns, there will be a warning log message when the OS buffer still has bytes left (byte value may or may not be accurate to the total amount left in the buffer).
