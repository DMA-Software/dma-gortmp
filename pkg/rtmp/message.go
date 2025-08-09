package rtmp

import (
	"time"
)

// MessageType represents the type of an RTMP message.
type MessageType uint8

const (
	// MessageTypeSetChunkSize sets the chunk size for subsequent chunks.
	MessageTypeSetChunkSize MessageType = 1

	// MessageTypeAbortMessage aborts a message.
	MessageTypeAbortMessage MessageType = 2

	// MessageTypeAcknowledgement sends an acknowledgement.
	MessageTypeAcknowledgement MessageType = 3

	// MessageTypeUserControlMessage sends user control messages.
	MessageTypeUserControlMessage MessageType = 4

	// MessageTypeWindowAckSize sets the window acknowledgement size.
	MessageTypeWindowAckSize MessageType = 5

	// MessageTypeSetPeerBandwidth sets the peer bandwidth.
	MessageTypeSetPeerBandwidth MessageType = 6

	// MessageTypeAudio sends audio data.
	MessageTypeAudio MessageType = 8

	// MessageTypeVideo sends video data.
	MessageTypeVideo MessageType = 9

	// MessageTypeDataMessageAMF3 sends data in AMF3 format.
	MessageTypeDataMessageAMF3 MessageType = 15

	// MessageTypeSharedObjectAMF3 sends shared object in AMF3 format.
	MessageTypeSharedObjectAMF3 MessageType = 16

	// MessageTypeCommandMessageAMF3 sends command in AMF3 format.
	MessageTypeCommandMessageAMF3 MessageType = 17

	// MessageTypeDataMessageAMF0 sends data in AMF0 format.
	MessageTypeDataMessageAMF0 MessageType = 18

	// MessageTypeSharedObjectAMF0 sends shared object in AMF0 format.
	MessageTypeSharedObjectAMF0 MessageType = 19

	// MessageTypeCommandMessageAMF0 sends command in AMF0 format.
	MessageTypeCommandMessageAMF0 MessageType = 20

	// MessageTypeAggregateMessage sends aggregate message.
	MessageTypeAggregateMessage MessageType = 22
)

// String returns the string representation of the message type.
func (mt MessageType) String() string {
	switch mt {
	case MessageTypeSetChunkSize:
		return "SetChunkSize"
	case MessageTypeAbortMessage:
		return "AbortMessage"
	case MessageTypeAcknowledgement:
		return "Acknowledgement"
	case MessageTypeUserControlMessage:
		return "UserControlMessage"
	case MessageTypeWindowAckSize:
		return "WindowAckSize"
	case MessageTypeSetPeerBandwidth:
		return "SetPeerBandwidth"
	case MessageTypeAudio:
		return "Audio"
	case MessageTypeVideo:
		return "Video"
	case MessageTypeDataMessageAMF3:
		return "DataMessageAMF3"
	case MessageTypeSharedObjectAMF3:
		return "SharedObjectAMF3"
	case MessageTypeCommandMessageAMF3:
		return "CommandMessageAMF3"
	case MessageTypeDataMessageAMF0:
		return "DataMessageAMF0"
	case MessageTypeSharedObjectAMF0:
		return "SharedObjectAMF0"
	case MessageTypeCommandMessageAMF0:
		return "CommandMessageAMF0"
	case MessageTypeAggregateMessage:
		return "AggregateMessage"
	default:
		return "Unknown"
	}
}

// Message represents an RTMP message.
type Message struct {
	// ChunkStreamID identifies the chunk stream.
	ChunkStreamID uint32

	// MessageStreamID identifies the message stream.
	MessageStreamID uint32

	// Timestamp represents the message timestamp.
	Timestamp uint32

	// Type represents the message type.
	Type MessageType

	// Length represents the message length.
	Length uint32

	// Data contains the message payload.
	Data []byte

	// AbsoluteTimestamp represents the absolute timestamp.
	AbsoluteTimestamp time.Time
}

// NewMessage creates a new RTMP message.
func NewMessage(chunkStreamID, messageStreamID uint32, msgType MessageType, data []byte) *Message {
	return &Message{
		ChunkStreamID:     chunkStreamID,
		MessageStreamID:   messageStreamID,
		Type:              msgType,
		Data:              data,
		Length:            uint32(len(data)),
		AbsoluteTimestamp: time.Now(),
	}
}

// StreamType represents the type of stream for publishing.
type StreamType string

const (
	// StreamTypeLive indicates a live stream.
	StreamTypeLive StreamType = "live"

	// StreamTypeRecord indicates a recorded stream.
	StreamTypeRecord StreamType = "record"

	// StreamTypeAppend indicates appending to a recorded stream.
	StreamTypeAppend StreamType = "append"
)

// ChunkHeader represents the header of an RTMP chunk.
type ChunkHeader struct {
	// Format specifies the chunk header format (0-3).
	Format uint8

	// ChunkStreamID identifies the chunk stream.
	ChunkStreamID uint32

	// Timestamp represents the chunk timestamp.
	Timestamp uint32

	// MessageLength represents the message length.
	MessageLength uint32

	// MessageTypeID represents the message type.
	MessageTypeID uint8

	// MessageStreamID identifies the message stream.
	MessageStreamID uint32

	// TimestampDelta represents the timestamp delta.
	TimestampDelta uint32

	// ExtendedTimestamp represents the extended timestamp.
	ExtendedTimestamp uint32
}

// Chunk represents an RTMP chunk.
type Chunk struct {
	// Header contains the chunk header.
	Header *ChunkHeader

	// Data contains the chunk data.
	Data []byte
}

// HandshakeState represents the state of the RTMP handshake.
type HandshakeState int

const (
	// HandshakeStateUninitialized indicates the handshake hasn't started.
	HandshakeStateUninitialized HandshakeState = iota

	// HandshakeStateVersionSent indicates C0/S0 has been sent.
	HandshakeStateVersionSent

	// HandshakeStateAckSent indicates C1/S1 has been sent.
	HandshakeStateAckSent

	// HandshakeStateHandshakeDone indicates the handshake is complete.
	HandshakeStateHandshakeDone

	// HandshakeStateError indicates an error occurred during handshake.
	HandshakeStateError
)

// String returns the string representation of the handshake state.
func (hs HandshakeState) String() string {
	switch hs {
	case HandshakeStateUninitialized:
		return "Uninitialized"
	case HandshakeStateVersionSent:
		return "VersionSent"
	case HandshakeStateAckSent:
		return "AckSent"
	case HandshakeStateHandshakeDone:
		return "HandshakeDone"
	case HandshakeStateError:
		return "Error"
	default:
		return "Unknown"
	}
}
