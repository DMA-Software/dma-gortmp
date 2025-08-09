package rtmp

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// Server represents an RTMP server that can accept client connections.
// It handles multiple concurrent client connections and provides callbacks
// for various RTMP events.
type Server struct {
	config     *ServerConfig
	listener   net.Listener
	clients    map[string]*ServerConnection
	clientsMux sync.RWMutex
	running    bool
	runMux     sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// ServerConfig holds configuration options for the RTMP server.
type ServerConfig struct {
	// Addr specifies the network address to listen on.
	Addr string

	// ReadTimeout specifies the timeout for read operations.
	ReadTimeout time.Duration

	// WriteTimeout specifies the timeout for write operations.
	WriteTimeout time.Duration

	// HandshakeTimeout specifies the timeout for handshake completion.
	HandshakeTimeout time.Duration

	// ChunkSize specifies the default chunk size for new connections.
	ChunkSize uint32

	// WindowAckSize specifies the window acknowledgement size.
	WindowAckSize uint32

	// BandwidthLimitType specifies the bandwidth limit type.
	BandwidthLimitType uint8

	// MaxConnections specifies the maximum number of concurrent connections.
	MaxConnections int

	// OnConnect is called when a client connects.
	OnConnect func(conn *ServerConnection) error

	// OnDisconnect is called when a client disconnects.
	OnDisconnect func(conn *ServerConnection)

	// OnPublish is called when a client wants to publish a stream.
	OnPublish func(conn *ServerConnection, streamName string, streamType StreamType) error

	// OnPlay is called when a client wants to play a stream.
	OnPlay func(conn *ServerConnection, streamName string) error

	// OnMessage is called when a message is received from a client.
	OnMessage func(conn *ServerConnection, msg *Message) error
}

// ServerConnection represents a client connection to the server.
type ServerConnection struct {
	conn       net.Conn
	server     *Server
	id         string
	state      ConnectionState
	handshaker *Handshaker
	chunker    *ChunkStreamer
	streams    map[string]*Stream
	streamsMux sync.RWMutex

	// Connection metadata
	clientAddr    net.Addr
	connectTime   time.Time
	lastActivity  time.Time
	bytesReceived uint64
	bytesSent     uint64
}

// ConnectionState represents the state of a server connection.
type ConnectionState int

const (
	// ConnectionStateConnected indicates the connection is established.
	ConnectionStateConnected ConnectionState = iota

	// ConnectionStateHandshaking indicates the handshake is in progress.
	ConnectionStateHandshaking

	// ConnectionStateReady indicates the connection is ready for RTMP messages.
	ConnectionStateReady

	// ConnectionStatePublishing indicates the connection is publishing a stream.
	ConnectionStatePublishing

	// ConnectionStatePlaying indicates the connection is playing a stream.
	ConnectionStatePlaying

	// ConnectionStateClosing indicates the connection is being closed.
	ConnectionStateClosing

	// ConnectionStateClosed indicates the connection is closed.
	ConnectionStateClosed

	// ConnectionStateError indicates the connection encountered an error.
	ConnectionStateError
)

// String returns the string representation of the connection state.
func (cs ConnectionState) String() string {
	switch cs {
	case ConnectionStateConnected:
		return "Connected"
	case ConnectionStateHandshaking:
		return "Handshaking"
	case ConnectionStateReady:
		return "Ready"
	case ConnectionStatePublishing:
		return "Publishing"
	case ConnectionStatePlaying:
		return "Playing"
	case ConnectionStateClosing:
		return "Closing"
	case ConnectionStateClosed:
		return "Closed"
	case ConnectionStateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Stream represents a stream being published or played.
type Stream struct {
	Name        string
	Type        StreamType
	Publisher   *ServerConnection
	Subscribers []*ServerConnection
	StartTime   time.Time
	Active      bool
	Metadata    map[string]interface{}
}

// NewServer creates a new RTMP server with the provided configuration.
func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = &ServerConfig{
			Addr:               ":1935",
			ReadTimeout:        30 * time.Second,
			WriteTimeout:       30 * time.Second,
			HandshakeTimeout:   10 * time.Second,
			ChunkSize:          128,
			WindowAckSize:      2500000,
			BandwidthLimitType: 2,
			MaxConnections:     1000,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:  config,
		clients: make(map[string]*ServerConnection),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts the RTMP server and begins accepting connections.
func (s *Server) Start() error {
	s.runMux.Lock()
	defer s.runMux.Unlock()

	if s.running {
		return fmt.Errorf("server is already running")
	}

	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Addr, err)
	}

	s.listener = listener
	s.running = true

	go s.acceptLoop()

	return nil
}

// Stop stops the RTMP server and closes all client connections.
func (s *Server) Stop() error {
	s.runMux.Lock()
	defer s.runMux.Unlock()

	if !s.running {
		return fmt.Errorf("server is not running")
	}

	s.running = false
	s.cancel()

	if s.listener != nil {
		err := s.listener.Close()
		if err != nil {
			log.Printf("failed to close listener: %s", err)
			return err
		}
	}

	// Close all client connections
	s.clientsMux.Lock()
	for _, client := range s.clients {
		err := client.Close()
		if err != nil {
			log.Printf("failed to close client connection: %s", err)
			return err
		}
	}
	s.clientsMux.Unlock()

	return nil
}

// GetConnections returns a slice of all active connections.
func (s *Server) GetConnections() []*ServerConnection {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()

	connections := make([]*ServerConnection, 0, len(s.clients))
	for _, conn := range s.clients {
		connections = append(connections, conn)
	}

	return connections
}

// GetConnectionCount returns the number of active connections.
func (s *Server) GetConnectionCount() int {
	s.clientsMux.RLock()
	defer s.clientsMux.RUnlock()
	return len(s.clients)
}

// acceptLoop accepts incoming connections in a loop.
func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				// Log error and continue
				continue
			}
		}

		// Check connection limit
		if s.config.MaxConnections > 0 && s.GetConnectionCount() >= s.config.MaxConnections {
			err := conn.Close()
			if err != nil {
				log.Printf("failed to close connection: %s", err)
				return
			}
			continue
		}

		// Handle a new connection
		go s.handleConnection(conn)
	}
}

// handleConnection handles a new client connection.
func (s *Server) handleConnection(conn net.Conn) {
	// Generate unique connection ID
	connID := fmt.Sprintf("%s_%d", conn.RemoteAddr().String(), time.Now().UnixNano())

	serverConn := &ServerConnection{
		conn:         conn,
		server:       s,
		id:           connID,
		state:        ConnectionStateConnected,
		clientAddr:   conn.RemoteAddr(),
		connectTime:  time.Now(),
		lastActivity: time.Now(),
		streams:      make(map[string]*Stream),
	}

	// Add to clients' map
	s.clientsMux.Lock()
	s.clients[connID] = serverConn
	s.clientsMux.Unlock()

	// Set connection timeouts
	if s.config.ReadTimeout > 0 {
		err := conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		if err != nil {
			_ = conn.Close()
			log.Printf("failed to set read timeout: %s", err)
			return
		}
	}
	if s.config.WriteTimeout > 0 {
		err := conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
		if err != nil {
			_ = conn.Close()
			log.Printf("failed to set write timeout: %s", err)
			return
		}
	}

	// Initialize handshaker and chunker
	serverConn.handshaker = NewHandshaker(conn, false) // Server side
	serverConn.chunker = NewChunkStreamer(conn)

	// Set chunk size
	if s.config.ChunkSize > 0 {
		err := serverConn.chunker.SetReadChunkSize(s.config.ChunkSize)
		if err != nil {
			_ = serverConn.Close()
			log.Printf("failed to set chunk size: %s", err)
			return
		}
		err = serverConn.chunker.SetWriteChunkSize(s.config.ChunkSize)
		if err != nil {
			_ = serverConn.Close()
			log.Printf("failed to set chunk size: %s", err)
			return
		}
	}

	// Call OnConnect callback
	if s.config.OnConnect != nil {
		if err := s.config.OnConnect(serverConn); err != nil {
			err := serverConn.Close()
			if err != nil {
				log.Printf("failed to close connection: %s", err)
				return
			}
			return
		}
	}

	// Start connection handling
	serverConn.start()
}

// ServerConnection methods

// ID returns the unique connection identifier.
func (sc *ServerConnection) ID() string {
	return sc.id
}

// State returns the current connection state.
func (sc *ServerConnection) State() ConnectionState {
	return sc.state
}

// RemoteAddr returns the remote network address.
func (sc *ServerConnection) RemoteAddr() net.Addr {
	return sc.clientAddr
}

// ConnectTime returns the time when the connection was established.
func (sc *ServerConnection) ConnectTime() time.Time {
	return sc.connectTime
}

// LastActivity returns the time of the last activity on this connection.
func (sc *ServerConnection) LastActivity() time.Time {
	return sc.lastActivity
}

// BytesReceived returns the total number of bytes received.
func (sc *ServerConnection) BytesReceived() uint64 {
	return sc.bytesReceived
}

// BytesSent returns the total number of bytes sent.
func (sc *ServerConnection) BytesSent() uint64 {
	return sc.bytesSent
}

// WriteMessage sends an RTMP message to the client.
func (sc *ServerConnection) WriteMessage(msg *Message) error {
	if sc.state == ConnectionStateClosed || sc.state == ConnectionStateClosing {
		return fmt.Errorf("connection is closed")
	}

	err := sc.chunker.WriteMessage(msg)
	if err == nil {
		sc.bytesSent += uint64(msg.Length)
		sc.lastActivity = time.Now()
	}
	return err
}

// ReadMessage reads an RTMP message from the client.
func (sc *ServerConnection) ReadMessage() (*Message, error) {
	if sc.state == ConnectionStateClosed || sc.state == ConnectionStateClosing {
		return nil, fmt.Errorf("connection is closed")
	}

	msg, err := sc.chunker.ReadMessage()
	if err == nil && msg != nil {
		sc.bytesReceived += uint64(msg.Length)
		sc.lastActivity = time.Now()
	}
	return msg, err
}

// Close closes the connection.
func (sc *ServerConnection) Close() error {
	if sc.state == ConnectionStateClosed {
		return nil
	}

	sc.state = ConnectionStateClosing

	// Remove from the server's clients' map
	sc.server.clientsMux.Lock()
	delete(sc.server.clients, sc.id)
	sc.server.clientsMux.Unlock()

	// Call OnDisconnect callback
	if sc.server.config.OnDisconnect != nil {
		sc.server.config.OnDisconnect(sc)
	}

	// Close network connection
	err := sc.conn.Close()
	sc.state = ConnectionStateClosed

	return err
}

// start begins processing messages for this connection.
func (sc *ServerConnection) start() {
	defer func(sc *ServerConnection) {
		err := sc.Close()
		if err != nil {
			log.Printf("failed to close connection: %s", err)
			return
		}
	}(sc)

	// Perform handshake
	sc.state = ConnectionStateHandshaking

	handshakeCtx := context.Background()
	if sc.server.config.HandshakeTimeout > 0 {
		var cancel context.CancelFunc
		handshakeCtx, cancel = context.WithTimeout(handshakeCtx, sc.server.config.HandshakeTimeout)
		defer cancel()
	}

	handshakeDone := make(chan error, 1)
	go func() {
		handshakeDone <- sc.handshaker.DoHandshake()
	}()

	select {
	case err := <-handshakeDone:
		if err != nil {
			sc.state = ConnectionStateError
			return
		}
	case <-handshakeCtx.Done():
		sc.state = ConnectionStateError
		return
	}

	sc.state = ConnectionStateReady

	// Message processing loop
	for {
		select {
		case <-sc.server.ctx.Done():
			return
		default:
		}

		msg, err := sc.ReadMessage()
		if err != nil {
			sc.state = ConnectionStateError
			return
		}

		// Call OnMessage callback
		if sc.server.config.OnMessage != nil {
			if err := sc.server.config.OnMessage(sc, msg); err != nil {
				// Log error but continue processing
			}
		}

		// Process message based on type
		if err := sc.processMessage(msg); err != nil {
			// Log error but continue processing
		}
	}
}

// processMessage processes an incoming RTMP message.
func (sc *ServerConnection) processMessage(msg *Message) error {
	switch msg.Type {
	case MessageTypeSetChunkSize:
		// Handle chunk size change
		if len(msg.Data) >= 4 {
			chunkSize := uint32(msg.Data[0])<<24 | uint32(msg.Data[1])<<16 |
				uint32(msg.Data[2])<<8 | uint32(msg.Data[3])
			return sc.chunker.SetReadChunkSize(chunkSize)
		}

	case MessageTypeWindowAckSize:
		// Handle window acknowledgement size change
		// Implementation would go here

	case MessageTypeSetPeerBandwidth:
		// Handle peer bandwidth setting
		// Implementation would go here

	case MessageTypeCommandMessageAMF0:
		// Handle AMF0 command messages
		// Implementation would parse AMF0 and handle commands like connect, publish, play

	case MessageTypeCommandMessageAMF3:
		// Handle AMF3 command messages
		// Implementation would parse AMF3 and handle commands

	case MessageTypeAudio, MessageTypeVideo:
		// Handle audio/video data
		// Implementation would forward to subscribers

	case MessageTypeDataMessageAMF0, MessageTypeDataMessageAMF3:
		// Handle data messages
		// Implementation would process metadata

	default:
		// Unknown message type, log and ignore
	}

	return nil
}
