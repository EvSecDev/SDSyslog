# Secure Diode Syslog (SDSyslog)

A program to securely transmit and receive log messages across untrusted and/or unidirectional network links.

This program prioritizes unidirectional networking and data confidentiality/integrity to improve upon the UDP syslog protocol (RFC5424).

For technical details about this program's protocol, see `Protocol.md`.
For technical details about this program's architecture, see `Architecture.md`.

Warning: This program is early in its development and *does* contain bugs.

## Features

- Unidirectional network support
- Multi-packet payloads (for messages exceeding MTU of a single packet)
- Encrypted payloads

## Installation

Steps:

- Copy binary to the desired system
- For the receiving daemon:
  - `./sdsyslog configure --install-receiver`
- For the sending daemon:
  - `./sdsyslog configure --install-sender`
- Modify the configuration file to your needs (`/etc/sdsyslog.json`)

## Uninstallation

Steps:

- WARNING: this PERMANENTLY removes the private key file, configuration file, and any state-saving files
- For the receiving daemon:
  - `./sdsyslog configure --uninstall-receiver`
- For the sending daemon:
  - `./sdsyslog configure --uninstall-sender`

### SDSyslog Help Menu

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
To get started with this API, grab the HTML docs by querying the root path `curl http://localhost:18514/`

## Notes

- Maximum individual log message size is 4GB
