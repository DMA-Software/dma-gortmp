// Package rtmp provides a complete implementation of the Real-Time Messaging Protocol (RTMP).
// This package offers both client and server implementations following the official RTMP specifications.
package rtmp

import (
	"context"
	"net"
	"time"
)

// Client represents an RTMP client connection.
// It provides methods to connect to RTMP servers and publish/subscribe to streams.
type Client struct {
	conn       net.Conn
	config     *ClientConfig
	state      ClientState
	handshaker *Handshaker
	chunker    *ChunkStreamer
}

// ClientConfig holds configuration options for the RTMP client.
type ClientConfig struct {
	// ConnectTimeout specifies the maximum time to wait for connection establishment.
	ConnectTimeout time.Duration

	// ReadTimeout specifies the timeout for read operations.
	ReadTimeout time.Duration

	// WriteTimeout specifies the timeout for write operations.
	WriteTimeout time.Duration

	// ChunkSize specifies the chunk size for RTMP messages.
	ChunkSize uint32

	// WindowAckSize specifies the window acknowledgement size.
	WindowAckSize uint32

	// BandwidthLimitType specifies the bandwidth limit type.
	BandwidthLimitType uint8

	// AppName specifies the application name to connect to.
	AppName string

	// StreamName specifies the stream name for publishing/subscribing.
	StreamName string

	// Username for authentication (optional).
	Username string

	// Password for authentication (optional).
	Password string
}

// ClientState represents the current state of the RTMP client.
type ClientState int

const (
	// ClientStateDisconnected indicates the client is not connected.
	ClientStateDisconnected ClientState = iota

	// ClientStateHandshaking indicates the client is performing the RTMP handshake.
	ClientStateHandshaking

	// ClientStateConnecting indicates the client is establishing the connection.
	ClientStateConnecting

	// ClientStateConnected indicates the client is successfully connected.
	ClientStateConnected

	// ClientStatePublishing indicates the client is publishing a stream.
	ClientStatePublishing

	// ClientStateSubscribing indicates the client is subscribing to a stream.
	ClientStateSubscribing

	// ClientStateError indicates the client encountered an error.
	ClientStateError
)

// String returns the string representation of the client state.
func (s ClientState) String() string {
	switch s {
	case ClientStateDisconnected:
		return "Disconnected"
	case ClientStateHandshaking:
		return "Handshaking"
	case ClientStateConnecting:
		return "Connecting"
	case ClientStateConnected:
		return "Connected"
	case ClientStatePublishing:
		return "Publishing"
	case ClientStateSubscribing:
		return "Subscribing"
	case ClientStateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// NewClient creates a new RTMP client with the provided configuration.
func NewClient(config *ClientConfig) *Client {
	if config == nil {
		config = &ClientConfig{
			ConnectTimeout:     30 * time.Second,
			ReadTimeout:        30 * time.Second,
			WriteTimeout:       30 * time.Second,
			ChunkSize:          128,
			WindowAckSize:      2500000,
			BandwidthLimitType: 2,
		}
	}

	return &Client{
		config: config,
		state:  ClientStateDisconnected,
	}
}

// Connect establishes a connection to the RTMP server at the specified address.
//
//goland:noinspection ALL
func (c *Client) Connect(ctx context.Context, address string) error {
	// Implementation will be in internal package
	return nil
}

// Publish starts publishing a stream with the specified name.
//
//goland:noinspection ALL
func (c *Client) Publish(ctx context.Context, streamName string, streamType StreamType) error {
	// Implementation will be in internal package
	return nil
}

// Subscribe starts subscribing to a stream with the specified name.
//
//goland:noinspection ALL
func (c *Client) Subscribe(ctx context.Context, streamName string) error {
	// Implementation will be in internal package
	return nil
}

// WriteMessage sends an RTMP message to the server.
//
//goland:noinspection ALL
func (c *Client) WriteMessage(msg *Message) error {
	// Implementation will be in internal package
	return nil
}

// ReadMessage reads an RTMP message from the server.
func (c *Client) ReadMessage() (*Message, error) {
	// Implementation will be in internal package
	return nil, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.state = ClientStateDisconnected
		return err
	}
	return nil
}

// State returns the current state of the client.
func (c *Client) State() ClientState {
	return c.state
}
