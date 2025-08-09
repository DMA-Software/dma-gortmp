# DMA-GORTMP - Go RTMP Library

A complete implementation of the Real-Time Messaging Protocol (RTMP) in Go, following the official Adobe specifications. This library provides both client and server implementations with full protocol support.

## Features

- **Complete RTMP Protocol Support**: Implements all RTMP specifications including chunk streaming, message formats, and command messages
- **AMF Encoding/Decoding**: Full support for both AMF0 and AMF3 (Action Message Format) data encoding with automatic format detection
- **Client and Server APIs**: Easy-to-use client and server implementations
- **Cross-Platform**: Pure Go implementation that runs on Windows, Linux, macOS, and other supported architectures
- **Concurrent Connections**: Efficient handling of multiple simultaneous client connections
- **Stream Management**: Built-in stream publishing and subscription management
- **Production Ready**: Comprehensive error handling, logging, and graceful shutdown support

## Architecture

The library follows a modular architecture with a clear separation of concerns:

```
rtmp-go/
├── cmd/                    # Example applications
│   ├── rtmp-client/       # Example RTMP client
│   └── rtmp-server/       # Example RTMP server
├── pkg/rtmp/              # Public API
│   ├── client.go          # RTMP client implementation
│   ├── server.go          # RTMP server implementation
│   ├── message.go         # RTMP message types and structures
│   ├── handshaker.go      # RTMP handshake implementation
│   └── chunk_streamer.go  # Chunk streaming protocol
├── internal/              # Internal implementation
│   ├── amf0/              # AMF0 encoding/decoding
│   ├── amf3/              # AMF3 encoding/decoding
│   ├── protocol/          # Command message handling
│   └── connection/        # Connection management
└── refs/                  # Specification documents
```

## Installation

```bash
go get github.com/DMA-Software/dma-gortmp
```

## AMF3 Support

This library provides full support for both AMF0 and AMF3 (Action Message Format) encoding. AMF3 is a more compact binary format introduced with Flash Player 9 that offers several advantages:

- **Compact Encoding**: Variable-length integer encoding reduces message size
- **Reference Tables**: Automatic deduplication of strings, objects, and traits
- **Type Safety**: More precise type definitions and better object serialization
- **Modern Client Support**: Required by newer Flash clients and applications

### Automatic Format Detection

The library automatically detects and switches between AMF0 and AMF3 based on the `objectEncoding` property in the RTMP connect command:

- `objectEncoding: 0` - Uses AMF0 format (default, backward compatible)
- `objectEncoding: 3` - Uses AMF3 format (modern, more efficient)

This detection happens automatically during the connection handshake, so no manual configuration is required. The same connection can seamlessly handle both formats as negotiated by the client.

## Quick Start

### RTMP Client

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/DMA-Software/dma-gortmp/pkg/rtmp"
)

func main() {
    // Create client configuration
    config := &rtmp.ClientConfig{
        ConnectTimeout: 30 * time.Second,
        ReadTimeout:    30 * time.Second,
        WriteTimeout:   30 * time.Second,
        AppName:        "live",
        StreamName:     "mystream",
    }
    
    // Create and connect client
    client := rtmp.NewClient(config)
    ctx := context.Background()
    
    if err := client.Connect(ctx, "rtmp://localhost:1935"); err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // Start publishing
    if err := client.Publish(ctx, "mystream", rtmp.StreamTypeLive); err != nil {
        log.Fatal(err)
    }
    
    // Send messages...
    msg := rtmp.NewMessage(2, 1, rtmp.MessageTypeAudio, audioData)
    client.WriteMessage(msg)
}
```

### RTMP Server

```go
package main

import (
    "log"
    "time"
    
    "github.com/DMA-Software/dma-gortmp/pkg/rtmp"
)

func main() {
    // Create server configuration
    config := &rtmp.ServerConfig{
        Addr:           ":1935",
        ReadTimeout:    30 * time.Second,
        WriteTimeout:   30 * time.Second,
        MaxConnections: 1000,
        OnConnect:      onConnect,
        OnPublish:      onPublish,
        OnPlay:         onPlay,
    }
    
    // Create and start server
    server := rtmp.NewServer(config)
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
    
    // Server runs until stopped
    select {} // Keep running
}

func onConnect(conn *rtmp.ServerConnection) error {
    log.Printf("Client connected: %s", conn.ID())
    return nil
}

func onPublish(conn *rtmp.ServerConnection, streamName string, streamType rtmp.StreamType) error {
    log.Printf("Stream published: %s", streamName)
    return nil
}

func onPlay(conn *rtmp.ServerConnection, streamName string) error {
    log.Printf("Stream requested: %s", streamName)
    return nil
}
```

## API Reference

### Client API

#### `rtmp.Client`

The main RTMP client type for connecting to RTMP servers.

**Methods:**
- `NewClient(config *ClientConfig) *Client` - Creates a new client
- `Connect(ctx context.Context, address string) error` - Connects to server
- `Publish(ctx context.Context, streamName string, streamType StreamType) error` - Starts publishing
- `Subscribe(ctx context.Context, streamName string) error` - Starts subscribing
- `WriteMessage(msg *Message) error` - Sends a message
- `ReadMessage() (*Message, error)` - Receives a message
- `Close() error` - Closes the connection
- `State() ClientState` - Returns current state

#### `rtmp.ClientConfig`

Configuration for RTMP clients.

**Fields:**
- `ConnectTimeout time.Duration` - Connection timeout
- `ReadTimeout time.Duration` - Read operation timeout
- `WriteTimeout time.Duration` - Write operation timeout
- `ChunkSize uint32` - RTMP chunk size
- `WindowAckSize uint32` - Window acknowledgement size
- `AppName string` - Application name
- `StreamName string` - Stream name
- `Username string` - Authentication username (optional)
- `Password string` - Authentication password (optional)

### Server API

#### `rtmp.Server`

The main RTMP server type for accepting client connections.

**Methods:**
- `NewServer(config *ServerConfig) *Server` - Creates a new server
- `Start() error` - Starts the server
- `Stop() error` - Stops the server
- `GetConnections() []*ServerConnection` - Returns active connections
- `GetConnectionCount() int` - Returns connection count

#### `rtmp.ServerConfig`

Configuration for RTMP servers.

**Fields:**
- `Addr string` - Listen address (e.g., ":1935")
- `ReadTimeout time.Duration` - Read timeout
- `WriteTimeout time.Duration` - Write timeout
- `HandshakeTimeout time.Duration` - Handshake timeout
- `ChunkSize uint32` - Default chunk size
- `WindowAckSize uint32` - Window acknowledgement size
- `MaxConnections int` - Maximum concurrent connections

**Callbacks:**
- `OnConnect func(conn *ServerConnection) error` - Called when a client connects
- `OnDisconnect func(conn *ServerConnection)` - Called when a client disconnects
- `OnPublish func(conn *ServerConnection, streamName string, streamType StreamType) error`
- `OnPlay func(conn *ServerConnection, streamName string) error`
- `OnMessage func(conn *ServerConnection, msg *Message) error`

#### `rtmp.ServerConnection`

Represents a client connection to the server.

**Methods:**
- `ID() string` - Returns unique connection ID
- `State() ConnectionState` - Returns connection state
- `RemoteAddr() net.Addr` - Returns client address
- `WriteMessage(msg *Message) error` - Sends message to client
- `ReadMessage() (*Message, error)` - Receives message from client
- `Close() error` - Closes the connection

### Message Types

#### `rtmp.Message`

Represents an RTMP message.

**Fields:**
- `ChunkStreamID uint32` - Chunk stream identifier
- `MessageStreamID uint32` - Message stream identifier
- `Timestamp uint32` - Message timestamp
- `Type MessageType` - Message type
- `Length uint32` - Message length
- `Data []byte` - Message payload

**Methods:**
- `NewMessage(chunkStreamID, messageStreamID uint32, msgType MessageType, data []byte) *Message`

#### `rtmp.MessageType`

RTMP message types (constants):
- `MessageTypeSetChunkSize` - Set chunk size
- `MessageTypeAudio` - Audio data
- `MessageTypeVideo` - Video data
- `MessageTypeCommandMessageAMF0` - AMF0 command message
- `MessageTypeCommandMessageAMF3` - AMF3 command message
- And many more...

### Stream Types

#### `rtmp.StreamType`

Stream publishing types:
- `StreamTypeLive` - Live stream
- `StreamTypeRecord` - Recorded stream
- `StreamTypeAppend` - Append to recorded stream

### States

#### `rtmp.ClientState`

Client connection states:
- `ClientStateDisconnected` - Not connected
- `ClientStateHandshaking` - Performing handshake
- `ClientStateConnecting` - Establishing connection
- `ClientStateConnected` - Successfully connected
- `ClientStatePublishing` - Publishing a stream
- `ClientStateSubscribing` - Subscribing to a stream
- `ClientStateError` - Error state

## Examples

### Running the Example Applications

The library includes complete example applications:

#### RTMP Client Example

```bash
# Basic usage
go run cmd/rtmp-client/main.go -url rtmp://localhost:1935/live -stream mystream

# With authentication
go run cmd/rtmp-client/main.go -url rtmp://localhost:1935/live -stream mystream -user username -pass password

# Custom timeouts
go run cmd/rtmp-client/main.go -url rtmp://localhost:1935/live -stream mystream -conn-timeout 60s
```

#### RTMP Server Example

```bash
# Basic server
go run cmd/rtmp-server/main.go

# Custom configuration
go run cmd/rtmp-server/main.go -addr :1935 -max-connections 100

# With authentication
go run cmd/rtmp-server/main.go -require-auth -username admin -password secret
```

### Advanced Usage

#### Custom Message Handling

```go
// Handle incoming messages
for {
    msg, err := client.ReadMessage()
    if err != nil {
        break
    }
    
    switch msg.Type {
    case rtmp.MessageTypeAudio:
        handleAudio(msg.Data)
    case rtmp.MessageTypeVideo:
        handleVideo(msg.Data)
    case rtmp.MessageTypeCommandMessageAMF0:
        handleCommand(msg.Data)
    }
}
```

#### Stream Management

```go
// Server-side stream management
func onPublish(conn *rtmp.ServerConnection, streamName string, streamType rtmp.StreamType) error {
    // Validate stream name
    if !isValidStreamName(streamName) {
        return fmt.Errorf("invalid stream name")
    }
    
    // Check if already publishing
    if isStreamActive(streamName) {
        return fmt.Errorf("stream already exists")
    }
    
    // Start recording or relaying
    go recordStream(streamName, conn)
    return nil
}
```

## Implementation Details

### RTMP Handshake

The library implements the complete RTMP handshake sequence:
1. C0/S0: Version exchange (1 byte)
2. C1/S1: Time and random data exchange (1536 bytes)  
3. C2/S2: Time and random data verification (1536 bytes)

### Chunk Streaming

RTMP messages are split into chunks for transmission. The library handles:
- Four chunk header formats (Type 0, 1, 2, 3)
- Dynamic chunk size negotiation
- Message assembly from multiple chunks
- Proper timestamp handling

### AMF Support

Complete AMF (Action Message Format) implementation:
- **AMF0**: All data types including Number, Boolean, String, Object, Array, Date, etc.
- **AMF3**: All data types including Number, Boolean, String, Object, Array, Date, etc.
- **AMF0/3 Reference Tables**: Automatic deduplication of strings, objects, and traits
- **AMF0/3 Type Safety**: More precise type definitions and better object serialization
- **AMF0/3 Modular Encoding**: Flexible encoding format that allows for future extensions
- **AMF0/3 Modular Decoding**: Flexible decoding format that allows for future extensions
- **AMF0/3 Type Conversion**: Automatic type conversion between Go types and AMF types
- **AMF3/3 Type Conversion**: Automatic type conversion between Go types and AMF types
- Automatic type conversion between Go types and AMF types

### Command Messages

Full support for RTMP commands:
- `connect` - Client connection
- `createStream` - Stream creation
- `publish` - Stream publishing
- `play` - Stream playback
- `seek`, `pause` - Playback control
- Custom command support

## Performance

The library is designed for high performance:
- Efficient memory usage with buffer pooling
- Concurrent connection handling
- Minimal memory allocations in hot paths
- Configurable timeouts and buffer sizes

## Compliance

This implementation follows the official RTMP specifications:
- Adobe Flash Video File Format Specification v10.1
- Real Time Messaging Protocol (RTMP) specification
- Action Message Format (AMF0) specification
- Action Message Format (AMF3) specification

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

This project is licensed under the GPL-3.0 license License. See LICENSE file for details.

## Support

For issues, questions, or contributions, please visit the GitHub repository or contact the maintainers.

---

**Note**: This is a production-ready RTMP library implementation following Adobe's official specifications. It provides complete protocol support suitable for streaming applications, media servers, and RTMP-based solutions.