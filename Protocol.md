# Protocol Specification

## Introduction

A unidirectional protocol supporting large fragmented messages and asymmetric cryptography operating over User Datagram Protocol (UDP) port `8514`.

Specifically, this protocol requires no server-to-client data flow and allows for messages exceeding a single UDP packet's maximum payload size.

Examples of protections offered:

- Prevent adversaries-in-the-middle from reading messages that contain sensitive data.
- Prevent adversaries-in-the-middle from conducting targeted denial of service attacks against messages matching certain text.
- Prevent compromise of a single client from compromising the confidentiality of all other clients.
- Prevent packet loss from affecting partial-message reconstruction.

### Security Considerations

This protocol is designed to minimize information leakage in the presence of an active or passive adversary.

Strict parsing, randomized padding, and fragment verification are intended to mitigate some forms of traffic analysis, message forgery, and targeted denial-of-service attacks.

Failure to enforce all MUST and MUST NOT requirements defined in this document may result in loss of confidentiality, message integrity, or availability.

## Conventions Used in This Document

The key words “MUST”, “MUST NOT”, “REQUIRED”, “SHOULD”, “SHOULD NOT”, “RECOMMENDED”, “MAY”, and “OPTIONAL” in this document are to be interpreted as described in RFC 2119 and RFC 8174.

`Fragment` refers to the data carried in the DATA section of a single UDP packet.

`Message` refers to the data prior to fragmentation on the sending side, and after defragmentation on the receiving side.

`HKDF` refers to the simple key derivation function (KDF) based on the HMAC message authentication code.

`NUL`, `Null-Byte`, or `null terminator` refers to the byte sequence represented as `\0` or `0x00`.

`NXTLEN` or `next length` refers to the field representing the total byte length of the next field not including the field's null terminator.

`SECLEN` is a special field referring to the total byte length of a section not including the section's null terminator.

`Empty character` refers to the placeholder that is used when an optional field is provided empty and is represented as `-` ASCII character.

`SEQMAX` or `sequence maximum` represents the maximum sequence ID value. For example, a message with SEQMAX = 3 consists of four packets with sequence IDs 0–3.

`ASCII` refers to the American Standard Code for Information Interchange character encoding standard.

`IEEE` refers to the organization Institute of Electrical and Electronics Engineers.

## Outer Packet Payload Layout

```text
----------------------------------------------------------
|                  Encryption Header (45B)               |
|       1 Byte        |       32 Bytes        | 12 Bytes |
| Encryption Suite ID | Ephemeral Public Key  |   Nonce  |
----------------------------------------------------------
------------------------------
| 48 Bytes - Remaining Bytes |
|  Encrypted Inner Payload   |
------------------------------
------------
| 16 bytes |
| MAC Tag  |
------------
```

### Encryption Header

A 1-byte unsigned integer representing the encryption suite used.

| Suite ID | Asymmetric Algorithm | Key Derivation | Symmetric Algorithm | Notes                                                                  |
|----------|----------------------|----------------|---------------------|------------------------------------------------------------------------|
| 0        | None                 | None           | None                | Testing (MUST be considered invalid and all packets MUST be discarded) |
| 1        | Curve25519 (X25519)  | HKDF (SHA512)  | ChaCha20-Poly1305   |                                                                        |

Persistent key pairs for generating ephemeral keys are pre-shared out-of-bounds between receiver and sender.

Symmetric encryption keys MUST not be reused for multiple fragments or messages.

Symmetric encryption keys MUST never be sent to the receiver, even if they are encrypted.

Nonce values are randomly generated and MUST not be reused for multiple fragments.

Both the cipher suite ID and ephemeral public key included in the header MUST be included as Additional Authenticated Data (AAD) for the authenticated encryption with associated data (AEAD) cipher.

## Inner Packet Payload Layout

```text
------------------------------------------
|              HEADER (12B)              |
| 4 Bytes  | 4 Bytes | 2 Bytes | 2 Bytes |
| HOSTID   | MSGID   | MSGSEQ  | SEQMAX  |
------------------------------------------
-------------------------------------------------
|            METADATA (11B - 265B)              |
| 8 Bytes   | 1 Byte | 1-255 Bytes | 1 Byte     |
| Timestamp | NXTLEN | Hostname    | NUL (0x00) |
-------------------------------------------------
---------------------------------------------------------------------------------------------------------------------------
|                                                     CONTEXT (3B - XB)                                                   |
|         |                                 Key-Value Pair                                  |                |            |
| 2 Bytes | 1 Byte | 1-32 Bytes | 1 Byte      | 1 Byte | 1 Byte | 1-255 Bytes | 1 Byte      | n KeyVal Pairs | 1 Byte     |
| SECLEN  | NXTLEN |    Key     | NUL (0x00)  |  Type  | NXTLEN |   Value     | NUL (0x00)  | ....           | NUL (0x00) | 
---------------------------------------------------------------------------------------------------------------------------
------------------------------------------
|            DATA (4B - XB)              |
| 2 Bytes | Remaining Bytes |  1 Byte    |
| NXTLEN  |  Fragment Text   | NUL (0x00) |
------------------------------------------
---------------
|   TRAILER   |
| 10-60 Bytes |
|   Padding   |
---------------
```

### Header

#### Host ID / Msg ID

An 8-byte sequence consisting of two parts:

- Random 4-byte sequence generated once at program startup and used as the host identifier.
- Random 4-byte sequence generated per message originating from an external source.

The Host ID is ephemeral and MUST NOT be used for long-term host identification.

Message identifiers MUST be discarded immediately upon message reconstruction.

#### Sequence ID

A 2-byte unsigned integer counter that is only unique per message ID and is incremented per packet for the given message ID.

Counter MUST start at 0.

#### Sequence Maximum

A 2-byte unsigned integer representing the total number of packets for a given message.

Counter MUST start at 0.

### Metadata

#### Timestamp

A 64-bit unsigned integer representing epoch timestamp.
Value is stored in an 8-byte field.

Epoch MUST be in milliseconds.

#### Hostname

A string denoting the machine’s hostname or fully qualified domain name (FQDN) up to and including 255 characters in length.

This value SHOULD be the value returned by the sender operating system.
Failed lookups result in the use of the empty character.

Only ASCII characters are allowed.

Unsupported characters (non-ASCII) returned from the lookup SHALL be removed and remaining characters used as-is.

Longer names MUST be truncated by removing the suffix so the remaining prefix is the field's maximum length.

### Context (Custom fields)

#### Context Length (SECLEN)

A 2-byte length field denoting the total length in bytes of all key-value pairs (including the key-value pairs `NXTLEN`, NUL, and type fields).

Does NOT include the section null terminator.

When no context is present, this field MUST be set to 0x0001 and does not represent an actual length value.

Fragments MUST be discarded if a `SECLEN` value of 7 or less bytes is present (excluding marker byte of `0x0001`), as this cannot encode a valid key-value pair.

#### Key-Value Pair

A 7-field section to hold a custom key and type-specific value.

Both key and value are `NXTLEN` prefixed and null terminated fields.
NUL (0x00) characters present in input key/value data SHALL be removed.

Key field is an ASCII string value up to and including 32 characters in length.
Non-ASCII characters are removed and remaining characters SHALL be used as the key.
Minimum length is 1 byte.
Empty keys MUST not be permitted.

Type field is a single byte representing what the value data represents.

Value field is a type-specific value up to and including 255 bytes in length.
Minimum length is 1 byte.
If the supplied data is empty, the empty character MUST be used to satisfy 1 byte minimum length.

Type byte table:

| Byte | Type    | Matching `NXTLEN` length | Notes                     |
|------|---------|------------------------|---------------------------|
| 0x01 | int8    | 1 byte                 | Signed 8-bit integer      |
| 0x02 | int16   | 2 bytes                | Signed 16-bit integer     |
| 0x03 | int32   | 4 bytes                | Signed 32-bit integer     |
| 0x04 | int64   | 8 bytes                | Signed 64-bit integer     |
| 0x05 | float32 | 4 bytes                | IEEE 754 single-precision |
| 0x06 | float64 | 8 bytes                | IEEE 754 double-precision |
| 0x07 | bool    | 1 byte                 | 0x00=false, 0x01=true     |
| 0x08 | string  | 1-255 bytes            | UTF-8 string              |

For fixed-width types, `NXTLEN` MUST exactly match the required length. If the length does not match, the entire fragment MUST be discarded.

For fixed-width numeric and boolean types, the minimum length requirement does not apply; the length MUST exactly match the type width.

Unknown types MUST be treated as an indication of a malformed fragment and the fragment discarded entirely.

#### Context Terminator (NUL (0x00))

An extra NUL (0x00) character MUST be placed at the end of the key-value pair set to denote the end of the context section.

Field is present even when no context fields are present.

### Data

`NXTLEN` field is the byte length of the fragment in each packet.
It is presented by a 16-bit unsigned integer.

Empty fragments processed from client programs should be discarded and no packets sent.

Fragment text MUST be valid UTF-8. NUL (0x00) bytes encountered in fragment text MUST be removed prior to transmission.

### Padding

A random length (within the defined range) of random bytes MUST be included at the end of every inner payload and MUST be varied per packet.

Both the length and the content of the padding MUST be obtained from a cryptographic pseudorandom source.

## Error Handling

Unless otherwise specified, any violation of a MUST or MUST NOT requirement defined in this specification SHALL result in immediate discard of the entire fragment or message without further processing.

Implementations MUST NOT attempt partial recovery from malformed packets.

## Serialization/Deserialization

`NXTLEN` and `NUL` termination MUST be mutually consistent, disagreement is a fatal error.

`NXTLEN` represents the length of the immediate next field as represented by a single 8 bit integer (or 16 bit integer for the fragment text).
This field does not include the null terminator of the field.
The field should be read up to the specified length and the next byte in the payload MUST be checked for the null terminator.

If the null terminator is not present immediately following the field, the receiver MUST discard the entire fragment.

If any encountered `NXTLEN` field is zero, the fragment MUST be discarded.

The Context section MUST be parsed by reading `SECLEN` bytes, after which exactly one NUL (0x00) byte MUST follow as the section terminator.
Parsers MUST NOT read beyond `SECLEN` when processing key-value pairs.

Context (custom fields) in total (`SECLEN`) MUST not exceed the header/metadata/data sections minimum lengths.

At minimum the size for variable length fields, the only byte that MUST be present is the `-` ASCII character, known as the "empty character".

All integer and floating-point values MUST be encoded in big-endian byte order.
Signed-ness is determined solely by the associated Type field, if applicable.

UDP Payloads less than the sum of the minimum protocol field lengths MUST be considered invalid and immediately discarded.

## Fragmentation

This section describes fragmentation behavior for messages exceeding the maximum allowable payload size.

Creation:

- Protocol header (inner) MUST be sent with every packet.
- Total transport payload length MUST not exceed the transport layer MTU minus the total maximum protocol field lengths minus maximum trailer length.
- Creation of individual packets from a singular message MUST start fragment sequences at 0 and increment up by 1 each packet.
- Individual fragments MUST fill the maximum inner payload size, excluding the trailer.

Extraction:

- All the following per-fragment fields MUST be checked for equality and cease further processing if any one payload is not identical:
  - Host ID
  - Msg ID
  - Timestamp
  - Hostname
- Any missing fragments (as determined by missing sequence IDs) MUST have a known static placeholder string inserted into the missing position inside the reconstructed message to clearly indicate the missing section.

The placeholder string MUST be the ASCII sequence `[missing fragment]`.
The placeholder string MUST NOT be subject to further parsing or interpretation.

## Multi-packet reconstruction (and deadline)

By default, multi-packet messages SHOULD be considered fully delivered after `50` milliseconds of idle wait time after the last reception of a given msg ID.
Packets for the same msg ID received within `50` milliseconds MUST be considered part of the same message and each new received packet MUST reset the idle wait timer.

Multi-packet messages that contain an identical msg ID that are received more than `50` milliseconds apart MUST be considered separate messages and processed as such.

Encountered multi-packet messages that overlap in sequence ID MUST both be stored and the later message fragment included in brackets in final message text.
Overlaps in message sequence ID more than once (i.e. more than 3 of the same sequence within deadline) MUST discard all further duplicates and consider that sequence ID finalized.

Allowances for shorter idle wait time or longer idle wait time is permitted only once a sufficiently large dataset average is calculated based on previous multi-packet message arrival times.

Runtime calculated idle wait times MUST never be cached to disk and are only to be used for the duration of program uptime.

Arrival time MUST be calculated based on the time between the reception of one packet and another packet for a given message ID regardless of sequence ID.

Idle wait times MUST be bypassed when the received packet sequence maximum is `0`.

Upon fragment reception, the source IP address MUST be used as a namespace identifier for scoping all received messages.

## Implementation Limits

Total ingested per-message size MUST not exceed 4GB.

## IANA Considerations

This document makes no request of IANA.

The UDP port number `8514` is used by convention and is not registered with IANA.
