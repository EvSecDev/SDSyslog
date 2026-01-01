# Architecture/Design Specification

This document covers the architecture and design specific to this implementation of the protocol.

For details about the protocol, see `Protocol.md`.

The goal:

- Implement the protocol in a system to transport log messages securely across a network
- Prioritize message integrity and total message throughput

## Overview

![diagram](architecture_diagram.png "Architecture Diagram")

## Sending Mode

### Sending Pipeline

Stage 1 - Listener (fixed - no scaling)

- One thread started per input type
  - File
  - Journald
  - Syslog
- Reads any metadata (if any) from the specific source and attaches to message.
- Parses message text and extracts relevant metadata into common format
- Pushes to central assembly queue

Stage 2 - Assembler (dynamic scaling)

- Reads from central assembly queue
- Constructs fragmented messages conforming to output transport protocol
- Serializes and encrypts fragments
- Pushes fragments to central sender queue (non-blocking)

Stage 3 - Senders (dynamic scaling)

- Reads fragments from center sender queue
- Blocks until packet can safely be enqueued at the OS-level
- Sends fragments to configured destination

### Sending Queues

Stage 1 to Stage 2 is a shared ring buffer queue.

- Contains the common format log message text and metadata.
- Blocking for file/journal sources on producer side.
- Non-blocking for syslog sources on producer side.
- Blocking for consumer side.

Stage 2 to Stage 3 is a shared mostly-unbounded queue.

- Contains raw byte slices of actual packet payload.
- High memory usage/disk usage to ensure messages are not dropped unless absolutely necessary for system stability.
- Fragments are buffered here until they can be safely sent across the network.
  - Safe defined as interfaces are up, and OS-level buffers can accept messages.
- Messages shall be buffered in memory until reaching a high water mark, then buffered on disk.
- High-water watcher shall ensure that the program shall not consume all system memory (triggering oom killer).
  - Disk queue shall be bounded and then dropped when exceeding bounds.

## Receiving Mode

Deadline definition can be found in `Protocol.md`.

### Processing Pipeline

Stage 1 - Listeners

- Reads packets from network (scaled horizontally via port reuse)
- Conducts pre-validation checks in order:
  - Discards if payload is not the protocol's minimum length
  - Peeks first byte to validate crypto suite ID (discards immediately if invalid)
- Pushes transport payload into queue

Stage 2 - Processors

- Reads payloads from queue
- Marks processing begin time
- Deconstructs and validates byte payload:
  - Parsing/Validation of outer payload
  - Decryption of the inner payload
  - Parsing/Validation of inner payload
- Choose a destination shard: Hash of source IP, host ID, log ID modulus the current number of shards
- Fragment is pushed into a bucket within the shard
  - Processing start time for each newest fragment is attached to each bucket (every time)
  - When the seq == seqmax has been reached for a given bucket, the bucket is considered filled
  - Then it puts the 'filled' shard bucket key into a FIFO channel for that shard

Stage 3 - Assemblers

- Separate watcher thread is started per assembler/shard to evaluate bucket deadlines on a polling basis - marks buckets as complete if they exceed deadline
- Blocking read on the shards' bucket key FIFO queue waiting for 'filled' buckets
- Upon receiving a 'filled' bucket key, the assembler will defragment the messages in the bucket:
  - Validate all fragments are equal
  - Sort partial messages into order
  - Combined messages into single message inserting placeholder text where there are missing sequence numbers
  - Place final messages into central output worker queue

Stage 4 - IOWorker

- Removes events from central queue
- Copy send to each destination by external source(s):
  - File
  - Journald
  - Syslog
- Pushes events to configured external source(s)

### Receiver Queues

Stage 1 to Stage 2 will share a single queue and operate on a lock-free ring buffer.

Stage 2 to Stage 3 is a sharded queue with one assembler for each shard (and deadline evaluator).

- Shard - Containing:
  - Buckets keyed on unique identifiers
  - Assembler bucket key queue (first-in first-out)
- Bucket - Containing:
  - All message fragment objects

Stage 3 to Stage 4 will share a single queue and operate on a lock-free ring buffer.

Note: all metric variables within queue objects are atomic read/write to allow for lock-less metric gathering.

### Shutdown Sequence

If the receiver program is being shutdown (internally or external signal):

- Shutdown the listeners first
- Delay further shutdown for a fixed time until queues can flush
- If fixed wait time is exceeded, program exits immediately

## Encryption

Ephemeral private keys are generated randomly on the sender and used in conjunction with a pre-shared receiver public key to created a shared secret.
The shared secret shall be used in conjunction with a key derivation function to generate a secure symmetric encryption key.
Symmetric encryption key will be used to encrypt the payload.

Cipher suite ID, Ephemeral public key, and the symmetric encryption cipher nonce, are sent over the wire.

Receiver side will use the received ephemeral public key and it's persistent private key to recreate the shared secret.
The shared secret will be put through the same key derivation function to recreate the symmetric encryption key.
Symmetric encryption key will be used to decrypt the payload.
