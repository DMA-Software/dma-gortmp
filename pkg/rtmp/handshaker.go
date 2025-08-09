package rtmp

import (
	"crypto/rand"
	"fmt"
	"net"
	"time"
)

// HandshakeVersion represents the RTMP version used in handshake.
const HandshakeVersion byte = 3

// HandshakePacketSize represents the size of C1/S1 and C2/S2 packets.
const HandshakePacketSize = 1536

// Handshaker handles the RTMP handshake process.
// The handshake consists of three parts from each peer:
// C0/S0: Version (1 byte)
// C1/S1: Time + Zero + Random (1536 bytes)
// C2/S2: Time + Time2 + Random echo (1536 bytes)
type Handshaker struct {
	conn         net.Conn
	state        HandshakeState
	isClient     bool
	clientTime   uint32
	serverTime   uint32
	clientRandom [1528]byte
	serverRandom [1528]byte
	clientTime2  uint32
	serverTime2  uint32
}

// NewHandshaker creates a new handshaker for the given connection.
func NewHandshaker(conn net.Conn, isClient bool) *Handshaker {
	return &Handshaker{
		conn:     conn,
		state:    HandshakeStateUninitialized,
		isClient: isClient,
	}
}

// State returns the current handshake state.
func (h *Handshaker) State() HandshakeState {
	return h.state
}

// DoHandshake performs the complete RTMP handshake.
func (h *Handshaker) DoHandshake() error {
	if h.isClient {
		return h.doClientHandshake()
	}
	return h.doServerHandshake()
}

// doClientHandshake performs the client side of the handshake.
func (h *Handshaker) doClientHandshake() error {
	// Send C0
	if err := h.sendC0(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to send C0: %w", err)
	}
	h.state = HandshakeStateVersionSent

	// Send C1
	if err := h.sendC1(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to send C1: %w", err)
	}

	// Receive S0
	if err := h.receiveS0(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to receive S0: %w", err)
	}

	// Receive S1
	if err := h.receiveS1(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to receive S1: %w", err)
	}
	h.state = HandshakeStateAckSent

	// Send C2
	if err := h.sendC2(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to send C2: %w", err)
	}

	// Receive S2
	if err := h.receiveS2(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to receive S2: %w", err)
	}

	h.state = HandshakeStateHandshakeDone
	return nil
}

// doServerHandshake performs the server side of the handshake.
func (h *Handshaker) doServerHandshake() error {
	// Receive C0
	if err := h.receiveC0(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to receive C0: %w", err)
	}

	// Receive C1
	if err := h.receiveC1(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to receive C1: %w", err)
	}
	h.state = HandshakeStateVersionSent

	// Send S0
	if err := h.sendS0(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to send S0: %w", err)
	}

	// Send S1
	if err := h.sendS1(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to send S1: %w", err)
	}
	h.state = HandshakeStateAckSent

	// Receive C2
	if err := h.receiveC2(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to receive C2: %w", err)
	}

	// Send S2
	if err := h.sendS2(); err != nil {
		h.state = HandshakeStateError
		return fmt.Errorf("failed to send S2: %w", err)
	}

	h.state = HandshakeStateHandshakeDone
	return nil
}

// sendC0 sends the client version byte.
func (h *Handshaker) sendC0() error {
	_, err := h.conn.Write([]byte{HandshakeVersion})
	return err
}

// sendC1 sends the client handshake packet.
func (h *Handshaker) sendC1() error {
	c1 := make([]byte, HandshakePacketSize)

	// Time (4 bytes)
	h.clientTime = uint32(time.Now().Unix())
	c1[0] = byte(h.clientTime >> 24)
	c1[1] = byte(h.clientTime >> 16)
	c1[2] = byte(h.clientTime >> 8)
	c1[3] = byte(h.clientTime)

	// Zero (4 bytes)
	// c1[4:8] are already zero

	// Random data (1528 bytes)
	if _, err := rand.Read(h.clientRandom[:]); err != nil {
		return fmt.Errorf("failed to generate random data: %w", err)
	}
	copy(c1[8:], h.clientRandom[:])

	_, err := h.conn.Write(c1)
	return err
}

// sendC2 sends the client acknowledgement packet.
func (h *Handshaker) sendC2() error {
	c2 := make([]byte, HandshakePacketSize)

	// Server time (4 bytes)
	c2[0] = byte(h.serverTime >> 24)
	c2[1] = byte(h.serverTime >> 16)
	c2[2] = byte(h.serverTime >> 8)
	c2[3] = byte(h.serverTime)

	// Client time (4 bytes)
	h.clientTime2 = uint32(time.Now().Unix())
	c2[4] = byte(h.clientTime2 >> 24)
	c2[5] = byte(h.clientTime2 >> 16)
	c2[6] = byte(h.clientTime2 >> 8)
	c2[7] = byte(h.clientTime2)

	// Echo server random data (1528 bytes)
	copy(c2[8:], h.serverRandom[:])

	_, err := h.conn.Write(c2)
	return err
}

// receiveS0 receives the server version byte.
func (h *Handshaker) receiveS0() error {
	version := make([]byte, 1)
	if _, err := h.conn.Read(version); err != nil {
		return err
	}

	if version[0] != HandshakeVersion {
		return fmt.Errorf("unsupported RTMP version: %d", version[0])
	}

	return nil
}

// receiveS1 receives the server handshake packet.
func (h *Handshaker) receiveS1() error {
	s1 := make([]byte, HandshakePacketSize)
	if _, err := h.conn.Read(s1); err != nil {
		return err
	}

	// Extract server time (4 bytes)
	h.serverTime = uint32(s1[0])<<24 | uint32(s1[1])<<16 | uint32(s1[2])<<8 | uint32(s1[3])

	// Extract server random data (1528 bytes)
	copy(h.serverRandom[:], s1[8:])

	return nil
}

// receiveS2 receives the server acknowledgement packet.
func (h *Handshaker) receiveS2() error {
	s2 := make([]byte, HandshakePacketSize)
	if _, err := h.conn.Read(s2); err != nil {
		return err
	}

	// Verify that S2 echoes our C1 data
	expectedTime := uint32(s2[0])<<24 | uint32(s2[1])<<16 | uint32(s2[2])<<8 | uint32(s2[3])
	if expectedTime != h.clientTime {
		return fmt.Errorf("s2 time mismatch: expected %d, got %d", h.clientTime, expectedTime)
	}

	// Verify random data echo
	if !compareBytes(s2[8:], h.clientRandom[:]) {
		return fmt.Errorf("s2 random data mismatch")
	}

	return nil
}

// Server-side methods
func (h *Handshaker) receiveC0() error {
	version := make([]byte, 1)
	if _, err := h.conn.Read(version); err != nil {
		return err
	}

	if version[0] != HandshakeVersion {
		return fmt.Errorf("unsupported RTMP version: %d", version[0])
	}

	return nil
}

func (h *Handshaker) receiveC1() error {
	c1 := make([]byte, HandshakePacketSize)
	if _, err := h.conn.Read(c1); err != nil {
		return err
	}

	// Extract client time (4 bytes)
	h.clientTime = uint32(c1[0])<<24 | uint32(c1[1])<<16 | uint32(c1[2])<<8 | uint32(c1[3])

	// Extract client random data (1528 bytes)
	copy(h.clientRandom[:], c1[8:])

	return nil
}

func (h *Handshaker) sendS0() error {
	_, err := h.conn.Write([]byte{HandshakeVersion})
	return err
}

func (h *Handshaker) sendS1() error {
	s1 := make([]byte, HandshakePacketSize)

	// Time (4 bytes)
	h.serverTime = uint32(time.Now().Unix())
	s1[0] = byte(h.serverTime >> 24)
	s1[1] = byte(h.serverTime >> 16)
	s1[2] = byte(h.serverTime >> 8)
	s1[3] = byte(h.serverTime)

	// Zero (4 bytes)
	// s1[4:8] are already zero

	// Random data (1528 bytes)
	if _, err := rand.Read(h.serverRandom[:]); err != nil {
		return fmt.Errorf("failed to generate random data: %w", err)
	}
	copy(s1[8:], h.serverRandom[:])

	_, err := h.conn.Write(s1)
	return err
}

func (h *Handshaker) receiveC2() error {
	c2 := make([]byte, HandshakePacketSize)
	if _, err := h.conn.Read(c2); err != nil {
		return err
	}

	// Verify that C2 echoes our S1 data
	expectedTime := uint32(c2[0])<<24 | uint32(c2[1])<<16 | uint32(c2[2])<<8 | uint32(c2[3])
	if expectedTime != h.serverTime {
		return fmt.Errorf("c2 time mismatch: expected %d, got %d", h.serverTime, expectedTime)
	}

	// Extract client time2
	h.clientTime2 = uint32(c2[4])<<24 | uint32(c2[5])<<16 | uint32(c2[6])<<8 | uint32(c2[7])

	// Verify random data echo
	if !compareBytes(c2[8:], h.serverRandom[:]) {
		return fmt.Errorf("c2 random data mismatch")
	}

	return nil
}

func (h *Handshaker) sendS2() error {
	s2 := make([]byte, HandshakePacketSize)

	// Client time (4 bytes)
	s2[0] = byte(h.clientTime >> 24)
	s2[1] = byte(h.clientTime >> 16)
	s2[2] = byte(h.clientTime >> 8)
	s2[3] = byte(h.clientTime)

	// Server time (4 bytes)
	h.serverTime2 = uint32(time.Now().Unix())
	s2[4] = byte(h.serverTime2 >> 24)
	s2[5] = byte(h.serverTime2 >> 16)
	s2[6] = byte(h.serverTime2 >> 8)
	s2[7] = byte(h.serverTime2)

	// Echo client random data (1528 bytes)
	copy(s2[8:], h.clientRandom[:])

	_, err := h.conn.Write(s2)
	return err
}

// compareBytes compares two byte slices for equality.
func compareBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
