# Protocol Specification

A unidirectional Syslog-like protocol supporting large fragmented messages and asymmetric cryptography operating over UDP Port `8514`.

Objectives:

- Prevent adversaries-in-the-middle from reading log messages that contain sensitive data.
- Prevent adversaries-in-the-middle from conducting targeted denial of service attacks against logs matching certain text.
- Prevent compromise of a single client from compromising the confidentiality of all other clients.
- Allow log messages larger than a single UDP packet's maximum payload size.
- Require no server-to-client data flow.
- Prevent packet loss from affecting partial-message reconstruction.

## Packet Payload Layout

`================ OUTER PAYLOAD ================`

```text
---------------------------------------------------
|               Encryption Header (45B)           |
|    1 Byte    |       32 Bytes        | 12 Bytes |
| Enc Suite ID | Ephemeral Public Key  |   Nonce  |
---------------------------------------------------
------------------------------
| 48 Bytes - Remaining Bytes |
|     Encrypted Payload      |
------------------------------
------------
| 16 bytes |
| MAC Tag  |
------------
```

`================ INNER PAYLOAD ================`

```text
------------------------------------------
|              HEADER (12B)              |
| 4 Bytes  | 4 Bytes | 2 Bytes | 2 Bytes |
| HOSTID   | LOGID   | MSGSEQ  | SEQMAX  |
------------------------------------------
--------------------------------------------------
|                 METADATA (22B)                 |
| 2 Bytes   | 2 Bytes  | 8 Bytes   | 4 Bytes     |
| Facility  | Severity | Timestamp | Process ID  |
--------------------------------------------------
------------------------------------------------------------------------------
|                            CONTEXT (6B - 307B)                             |
| 1 Byte | 1-255 Bytes | 1 Byte     | 1 Byte | 1-48 Bytes       | 1 Byte     |
| NXTLEN | Hostname    | Null Char  | NXTLEN | Application Name | Null Char  |
------------------------------------------------------------------------------
-----------------------------------------
|            DATA (4B - XB)             |
| 2 Bytes | Remaining Bytes |  1 Byte   |
| NXTLEN  |     Log Text    | Null Char |
-----------------------------------------
---------------
|   TRAILER   |
| 10-60 Bytes |
|   Padding   |
---------------
```

### Encryption Header

A 1 byte unsigned integer representing the encryption suite used.

- 0 = Testing (should be considering invalid and all packets discarded)
- 1 = X25519 Asymmetric and HMAC Key Derivation with ChaCha20Poly1305 Symmetric

Symmetric encryption keys shall not be reused for multiple log messages.

Symmetric encryption keys shall never be send to the receiver, even if they are encrypted.

Both the cipher suite ID and ephemeral public key included in the header should be included as part of the additional data for the AEAD cipher.

### Header

#### Host ID / Log ID

A 8 byte sequence consisting of two parts:

- Random 4 byte sequence generate once at program startup and used as the host identifier.
- Random 4 byte sequence generated per log line read from external sources.

Note: The Host ID is a ephemeral value that can change at any point and shall not be relied upon for long term host identification.

Log identifiers shall be discarded immediately upon message reconstruction.

#### Sequence ID

A 2 byte unsigned integer counter that is only unique per Message ID and is incremented per packet per log.

Counter always starts at 0.

#### Sequence Maximum

A 2 byte unsigned integer representing the total number of packets for a given log line.

Counter always starts at 0.

#### Facility

Follows facility numeric codes as described in RFC5424 section 6.2.1 table 1.
Codes are stored in a 2 byte field.

#### Severity

Follows severity numeric codes as described in RFC5424 section 6.2.1 table 2.
Codes are stored in a 2 byte field.

#### Timestamp

A 64-bit unsigned integer representing epoch timestamp.
Value is stored in 8 byte field.

Epoch shall be in milliseconds.

#### Process ID

A 32-bit unsigned integer presenting the process ID for the program that generated the log line.
Value is stored in 4 byte field.

This value should be retrieved by the following methods (first match used):

- Journald `_PID` field
- Journald `SYSLOG_PID` field
- Log line `PROCID` field
- Process ID of the program (self)

#### Hostname

A string denoting the machines hostname/fqdn up to and including 255 characters in length.

This value should be the value returned by the client OS kernel.
Failed lookups result in the use of the empty character.

Unsupported characters (non-ASCII) returned from the lookup should be removed and remaining characters used as-is.

Longer names are truncated to maximum length.

Only ASCII characters are allowed.

#### Application Name

A string denoting the program that generated the log up to and including 48 characters in length.

This value should be retrieved by the following methods (first match used):

- Journald `SYSLOG_IDENTIFIER`, `_SYSTEMD_USER_UNIT`, `_SYSTEMD_UNIT` fields (in order)
- Log line tag, i.e. `Dec 1 11:01:01 Server program1[1000]:` => `program1`
- Empty character

Unsupported characters (non-ASCII) returned from the lookup should be removed and remaining characters used as-is.

Longer names are truncated to maximum length.

Only ASCII characters are allowed.

#### Message

NXTLEN field is the byte length of the log message in each packet.
It is presented by a 16 bit unsigned integer.

Empty messages processed from client programs should be discarded and no packets sent.

Only UTF-8 characters are allowed. Null-Byte sequences are removed entirely from messages.

#### Padding

A random length (within the defined range) of random bytes shall be included at the end of every inner payload and shall be varied per packet.

Both the length and the content of the padding shall be obtained from a cryptographic pseudorandom source.

## Serialization/Deserialization

Null char always represents the byte sequence `\0`.

`NXTLEN` represents the length of the immediate next field as represented by a single 8 bit integer (or 16 bit integer for the log text).

At minimum message size for variable length fields, the only byte that should be present is the `-` ASCII character, known as the "empty character".

Numeric/Unsigned Integers are always encoded to bytes in big-endian form.

UDP Payloads less than the sum of the minimum protocol field lengths should be considered invalid and immediately discarded.

## Fragmentation

This section assumes the context to be a log message that is larger than the allowed payload size and will be fragmented.

Creation:

- Protocol header (inner) shall be sent with every packet.
- Total transport payload length shall not exceed the transport layer MTU minus the total maximum protocol field lengths minus maximum trailer length.
- Creation of individual packets from a singular log message shall start message sequences at 0 and increment up by 1 each packet.
- Individual fragments should fill the maximum transport inner payload size minus the maximum trailer size.

Extraction:

- All the following fragment headers must be checked for equality and cease further processing if any one payload is not identical:
  - Host ID
  - Log ID
  - Facility
  - Severity
  - Timestamp
  - Process ID
  - Hostname
  - Application Name
- Any missing fragments (as determined by missing sequence numbers) shall have a known static placeholder string inserted into the missing position inside the log message to clearly indicate the missing section.

## Multi-packet reconstruction (and deadline)

By default, multi-packet log messages should be considered fully delivered after `50` milliseconds of idle wait time after the last reception of a given log ID.
Packets for the same log ID received within `50` milliseconds shall be considered part of the same log message and each new received packet shall reset the idle wait timer.

Multi-packet log messages that contain an identical log ID that are received more than `50` milliseconds apart shall be considered separate logs and processed as such.

Encountered multi-packet log messages that overlap in message sequence number shall both be stored and the later message fragment included in brackets in final log text.
Overlaps in message sequence number more than once (i.e. more than 3 of the same sequence within deadline) shall discard all further duplicates and consider that sequence ID finalized.

Allowances for shorter idle wait time or longer idle wait time is permitted only once a sufficiently large dataset average is calculated based on previous multi-packet log message arrival times.

Runtime calculated idle wait times shall never be cached to disk and are only to be used for the duration of program uptime.

Arrival time shall be calculated based on the time between the reception of one packet and another packet for a given message ID irregardless of sequence ID.

Idle wait times shall be bypassed when the received packet sequence maximum is `0`.

Upon fragment reception, the source address shall be used as a namespace identifier for scoping all received messages.

## Notes

Total ingested per-log size shall not exceed 4GB.
