# Fragment Inter-Process Routing Protocol

Stateful protocol designed to share message fragments between processes.

## Overview

This protocol assumes a reliable, ordered, byte-stream transport.

Use over unordered or lossy transports is not supported.

All bytes and integer values MUST be encoded in big-endian byte order.

## Definitions

`Client` represents the side of the connection that sends fragments.

`Server` represents the side of the connection that receives fragments.

## Layout

```text
----------------------------------------------------------
|   4 Bytes    | 2 Bytes  | 1 Byte  | x Bytes | 16 Bytes |
| Frame Length | Sequence | OpCode  | Payload |   HMAC   |
----------------------------------------------------------
```

### Frame Length

A 4 byte unsigned integer field representing the number of bytes following the field.

Must be at least 19 bytes and no greater than 65,535 bytes.

Any value outside of this range must close the session.

### Sequence

A 2 byte unsigned integer field representing a counter increasing by one in every frame and is shared by Client and Server.

The integer starts at 0 for a new session and is incremented every time EITHER the client or the server decode a validated frame.

A validated frame is defined as one that passed validation checks OR validation encountered a retryable error.

`Sequence` numbers that are less than expected must close the session.

`Sequence` number wraparound must not occur. If the `Sequence` counter would overflow, the session must be immediately closed.

Because all frames consume `Sequence` numbers, loss of any frame is considered fatal unless explicitly recovered via Resend before the next expected sequence is observed.

### OpCodes

A single byte field representing the purpose of the frame.

| Byte | Code        | Purpose                                           | Payload Contents    | Payload Size |
|------|-------------|---------------------------------------------------|---------------------|--------------|
| 0x00 | Start       | Mark start of session with a message ID           | Message ID          | x Bytes      |
| 0x05 | OBO         | On-Behalf-Of (Remote address of original message) | IP Address          | x Bytes      |
| 0x10 | Ack         | Acknowledge receipt of message sequence           | Sequence            | 2 Bytes      |
| 0x11 | Resend      | Send the message sequence again                   | Sequence            | 2 Bytes      |
| 0x12 | Accepted    | Fragment accepted, end session                    | Empty               | 0 Bytes      |
| 0x13 | Rejected    | Fragment rejected, end session                    | Empty               | 0 Bytes      |
| 0x20 | ShardCheck  | Check shard status                                | Empty               | 0 Bytes      |
| 0x21 | ShardStatus | Shard status                                      | Shard Status Code   | 1 Byte       |
| 0x22 | MsgCheck    | Check message ID status                           | Empty               | 0 Bytes      |
| 0x23 | MsgStatus   | Message ID status                                 | Msg Status Code     | 1 Byte       |
| 0x24 | FrgRoute    | Route Fragment                                    | Serialized Fragment | x Bytes      |

`Start` opcodes must be the first frame in a session and must have a `Sequence` number of 0.

All other opcodes with `Sequence` of zero must immediately close the session.

`OBO` or `On-Behalf-Of` is a variable length byte sequence representing the original remote address for the associated fragment.

`OBO` or `On-Behalf-Of` is opaque to this protocol and can be any piece of data that identifies the original network address of the fragment.

`Message ID` is a variable length byte sequence representing the larger message identifier for the associated fragment.

`Message ID` is opaque to this protocol and can be any unique piece of data.

`Ack` and `Resend` frames advance the `Sequence` counter like all other frames.

All reception of opcodes EXCEPT `Ack`, `Resend`, `Accepted`, and `Rejected` must send an `Ack` opcode back or a `Resend` opcode.

The payload of `Ack` and `Resend` opcodes must contain the 2-byte `Sequence` number being acknowledged or requested for retransmission inside the payload.

`Resend` must only be used to request retransmission due to incorrect payload length for the given opcode.

`Resend` shall not be used to request retransmission due to higher level program errors when internally handling payloads.

Duplicate frames shall never be allowed and any duplicate sequence number must cause the session to close.

Any transport-layer connection closure prior to receipt of `Accepted` or `Rejected` must return a sentinel error value indicating non-protocol/session error.

### Shard Status

A single byte field representing the current Server process shard state.

| Byte | Code     | Description                                   |
|------|----------|-----------------------------------------------|
| 0x01 | Running  | Shard is accepting new and existing fragments |
| 0x11 | Draining | Shard is ONLY accepting existing fragments    |
| 0x22 | Shutdown | Shard is not accepting any fragments          |

Clients must obey `Draining` status and shall not send new fragments to `Draining` shards.

Clients must obey `Shutdown` status and shall not send any fragments to `Shutdown` shards.

Servers must enforce `Draining` status by sending `Rejected` opcodes when Clients attempt to route new fragments.

Servers must enforce `Shutdown` status by sending `Rejected` opcodes when Clients attempt to route any fragments.

### Message Status

A single byte field representing the inner-process state of the provided message identifier.

| Byte | Code     | Description                                          |
|------|----------|------------------------------------------------------|
| 0x00 | New      | Message does not exist in any internal shard         |
| 0x10 | Existing | At least one message fragment is present in a shard  |

Message status is only for the message ID given in the `Start` frame.

### Payload

Payload length shall only be zero when the associated opcode permits an empty payload.

### HMAC

The HMAC field consists of the first 16 bytes of the HMAC-SHA256 output.

All fields must be authenticated by the HMAC.

The HMAC is computed over the concatenation of frame length, `Sequence`, opcode, and payload.

Frames with invalid HMACs must immediately close the session and NOT send any response.

## Flow

### Standard Success

```text
                      == Connection Opened ==
Client -> [ Seq=0  OpCode=Start       Payload=<MessageID>   ] -> Server
Server -> [ Seq=1  OpCode=Ack         Payload=<Sequence 0>  ] -> Client

Client -> [ Seq=2  OpCode=ShardCheck  Payload=<>            ] -> Server
Server -> [ Seq=3  OpCode=Ack         Payload=<Sequence 2>  ] -> Client
Server -> [ Seq=4  OpCode=ShardStatus Payload=<Running>     ] -> Client
Client -> [ Seq=5  OpCode=Ack         Payload=<Sequence 4>  ] -> Server

Client -> [ Seq=6  OpCode=MsgCheck    Payload=<>            ] -> Server
Server -> [ Seq=7  OpCode=Ack         Payload=<Sequence 6>  ] -> Client
Server -> [ Seq=8  OpCode=MsgStatus   Payload=<New>         ] -> Client
Client -> [ Seq=9  OpCode=Ack         Payload=<Sequence 8>  ] -> Server

Client -> [ Seq=10 OpCode=OBO         Payload=<IPAddr>      ] -> Server
Server -> [ Seq=11 OpCode=Ack         Payload=<Sequence 10> ] -> Client

Client -> [ Seq=12 OpCode=FrgRoute    Payload=<Message>     ] -> Server
Server -> [ Seq=13 OpCode=Ack         Payload=<Sequence 12> ] -> Client
Server -> [ Seq=14 OpCode=Accepted    Payload=<>            ] -> Client
                      == Connection Closed ==
```

### Standard Draining

```text
                      == Connection Opened ==
Client -> [ Seq=0  OpCode=Start       Payload=<MessageID>   ] -> Server
Server -> [ Seq=1  OpCode=Ack         Payload=<Sequence 0>  ] -> Client

Client -> [ Seq=2  OpCode=ShardCheck  Payload=<>            ] -> Server
Server -> [ Seq=3  OpCode=Ack         Payload=<Sequence 2>  ] -> Client
Server -> [ Seq=4  OpCode=ShardStatus Payload=<Draining>    ] -> Client
Client -> [ Seq=5  OpCode=Ack         Payload=<Sequence 4>  ] -> Server

Client -> [ Seq=6  OpCode=MsgCheck    Payload=<>            ] -> Server
Server -> [ Seq=7  OpCode=Ack         Payload=<Sequence 6>  ] -> Client
Server -> [ Seq=8  OpCode=MsgStatus   Payload=<New>         ] -> Client
Client -> [ Seq=9  OpCode=Ack         Payload=<Sequence 8>  ] -> Server
                      == Connection Closed ==
```

### Standard Resend

```text
                      == Connection Opened ==
Client -> [ Seq=0  OpCode=Start       Payload=<MessageID>   ] -> Server
Server -> [ Seq=1  OpCode=Ack         Payload=<Sequence 0>  ] -> Client

Client -> [ Seq=2  OpCode=ShardCheck  Payload=<>            ] -> Server
Server -> [ Seq=3  OpCode=Ack         Payload=<Sequence 2>  ] -> Client
Server -> [ Seq=4  OpCode=ShardStatus Payload=<Running>     ] -> Client
Client -> [ Seq=5  OpCode=Resend      Payload=<Sequence 4>  ] -> Server
Server -> [ Seq=6  OpCode=ShardStatus Payload=<Running>     ] -> Client
Client -> [ Seq=7  OpCode=Ack         Payload=<Sequence 6>  ] -> Server

Client -> [ Seq=8  OpCode=MsgCheck    Payload=<>            ] -> Server
Server -> [ Seq=9  OpCode=Resend      Payload=<Sequence 8>  ] -> Client
Client -> [ Seq=10 OpCode=MsgCheck    Payload=<>            ] -> Server
Server -> [ Seq=11 OpCode=Ack         Payload=<Sequence 10> ] -> Client
Server -> [ Seq=12 OpCode=MsgStatus   Payload=<Existing>    ] -> Client
Client -> [ Seq=13 OpCode=Ack         Payload=<Sequence 12> ] -> Server

Client -> [ Seq=14 OpCode=OBO         Payload=<IPAddr>      ] -> Server
Server -> [ Seq=15 OpCode=Ack         Payload=<Sequence 14> ] -> Client

Client -> [ Seq=16 OpCode=FrgRoute    Payload=<Message>     ] -> Server
Server -> [ Seq=17 OpCode=Ack         Payload=<Sequence 16> ] -> Client
Server -> [ Seq=18 OpCode=Accepted    Payload=<>            ] -> Client
                      == Connection Closed ==
```
