package rtmp

import (
	"fmt"
	"io"
	"net"
)

// DefaultChunkSize represents the default chunk size for RTMP messages.
const DefaultChunkSize = 128

// MaxChunkSize represents the maximum allowed chunk size.
const MaxChunkSize = 65536

// ChunkStreamer handles the RTMP chunk stream protocol.
// It manages reading and writing of chunked RTMP messages over a network connection.
type ChunkStreamer struct {
	conn               net.Conn
	readChunkSize      uint32
	writeChunkSize     uint32
	readStreamStates   map[uint32]*chunkStreamState
	writeStreamStates  map[uint32]*chunkStreamState
	incompleteMessages map[uint32]*incompleteMessage
	nextWriteChunkID   uint32
	windowAckSize      uint32
	bytesReceived      uint32
	lastAckSent        uint32
}

// chunkStreamState maintains the state for a specific chunk stream.
type chunkStreamState struct {
	chunkStreamID   uint32
	timestamp       uint32
	messageLength   uint32
	messageTypeID   uint8
	messageStreamID uint32
	timestampDelta  uint32
}

// incompleteMessage tracks partially received messages.
type incompleteMessage struct {
	message     *Message
	bytesRead   uint32
	lastChunkID uint32
}

// NewChunkStreamer creates a new chunk streamer for the given connection.
func NewChunkStreamer(conn net.Conn) *ChunkStreamer {
	return &ChunkStreamer{
		conn:               conn,
		readChunkSize:      DefaultChunkSize,
		writeChunkSize:     DefaultChunkSize,
		readStreamStates:   make(map[uint32]*chunkStreamState),
		writeStreamStates:  make(map[uint32]*chunkStreamState),
		incompleteMessages: make(map[uint32]*incompleteMessage),
		nextWriteChunkID:   2,       // Start from 2 (0 and 1 are reserved for protocol control)
		windowAckSize:      2500000, // Default window acknowledgement size
	}
}

// SetReadChunkSize sets the chunk size for reading messages.
func (cs *ChunkStreamer) SetReadChunkSize(size uint32) error {
	if size > MaxChunkSize {
		return fmt.Errorf("chunk size too large: %d (max: %d)", size, MaxChunkSize)
	}
	if size == 0 {
		return fmt.Errorf("chunk size cannot be zero")
	}
	cs.readChunkSize = size
	return nil
}

// SetWriteChunkSize sets the chunk size for writing messages.
func (cs *ChunkStreamer) SetWriteChunkSize(size uint32) error {
	if size > MaxChunkSize {
		return fmt.Errorf("chunk size too large: %d (max: %d)", size, MaxChunkSize)
	}
	if size == 0 {
		return fmt.Errorf("chunk size cannot be zero")
	}
	cs.writeChunkSize = size
	return nil
}

// ReadMessage reads a complete RTMP message from the connection.
func (cs *ChunkStreamer) ReadMessage() (*Message, error) {
	for {
		chunk, err := cs.readChunk()
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk: %w", err)
		}

		msg, complete, err := cs.assembleMessage(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to assemble message: %w", err)
		}

		if complete {
			cs.updateBytesReceived(uint32(len(chunk.Data)) + cs.getChunkHeaderSize(chunk.Header.Format))
			return msg, nil
		}

		cs.updateBytesReceived(uint32(len(chunk.Data)) + cs.getChunkHeaderSize(chunk.Header.Format))
	}
}

// WriteMessage writes a complete RTMP message to the connection.
func (cs *ChunkStreamer) WriteMessage(msg *Message) error {
	if msg.Length == 0 {
		msg.Length = uint32(len(msg.Data))
	}

	chunkStreamID := msg.ChunkStreamID
	if chunkStreamID == 0 {
		chunkStreamID = cs.getNextChunkStreamID()
		msg.ChunkStreamID = chunkStreamID
	}

	chunks, err := cs.createChunks(msg)
	if err != nil {
		return fmt.Errorf("failed to create chunks: %w", err)
	}

	for _, chunk := range chunks {
		if err := cs.writeChunk(chunk); err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
	}

	return nil
}

// readChunk reads a single chunk from the connection.
func (cs *ChunkStreamer) readChunk() (*Chunk, error) {
	// Read basic header (1-3 bytes)
	basicHeader, err := cs.readBasicHeader()
	if err != nil {
		return nil, err
	}

	format := (basicHeader[0] & 0xC0) >> 6
	chunkStreamID := cs.parseChunkStreamID(basicHeader)

	// Get or create a chunk stream state
	state, exists := cs.readStreamStates[chunkStreamID]
	if !exists {
		state = &chunkStreamState{chunkStreamID: chunkStreamID}
		cs.readStreamStates[chunkStreamID] = state
	}

	// Read message header based on format
	header := &ChunkHeader{
		Format:        format,
		ChunkStreamID: chunkStreamID,
	}

	if err := cs.readMessageHeader(header, state); err != nil {
		return nil, err
	}

	// Calculate data size to read
	dataSize := cs.calculateChunkDataSize(header, state)

	// Read chunk data
	data := make([]byte, dataSize)
	if _, err := io.ReadFull(cs.conn, data); err != nil {
		return nil, fmt.Errorf("failed to read chunk data: %w", err)
	}

	return &Chunk{
		Header: header,
		Data:   data,
	}, nil
}

// writeChunk writes a single chunk to the connection.
func (cs *ChunkStreamer) writeChunk(chunk *Chunk) error {
	// Write a basic header
	basicHeader := cs.createBasicHeader(chunk.Header.Format, chunk.Header.ChunkStreamID)
	if _, err := cs.conn.Write(basicHeader); err != nil {
		return fmt.Errorf("failed to write basic header: %w", err)
	}

	// Write message header
	messageHeader := cs.createMessageHeader(chunk.Header)
	if len(messageHeader) > 0 {
		if _, err := cs.conn.Write(messageHeader); err != nil {
			return fmt.Errorf("failed to write message header: %w", err)
		}
	}

	// Write an extended timestamp if needed
	if chunk.Header.ExtendedTimestamp > 0 {
		extTimestamp := make([]byte, 4)
		extTimestamp[0] = byte(chunk.Header.ExtendedTimestamp >> 24)
		extTimestamp[1] = byte(chunk.Header.ExtendedTimestamp >> 16)
		extTimestamp[2] = byte(chunk.Header.ExtendedTimestamp >> 8)
		extTimestamp[3] = byte(chunk.Header.ExtendedTimestamp)
		if _, err := cs.conn.Write(extTimestamp); err != nil {
			return fmt.Errorf("failed to write extended timestamp: %w", err)
		}
	}

	// Write chunk data
	if len(chunk.Data) > 0 {
		if _, err := cs.conn.Write(chunk.Data); err != nil {
			return fmt.Errorf("failed to write chunk data: %w", err)
		}
	}

	return nil
}

// createChunks splits a message into chunks.
func (cs *ChunkStreamer) createChunks(msg *Message) ([]*Chunk, error) {
	var chunks []*Chunk
	data := msg.Data
	bytesRemaining := uint32(len(data))
	offset := uint32(0)
	isFirstChunk := true

	state, exists := cs.writeStreamStates[msg.ChunkStreamID]
	if !exists {
		state = &chunkStreamState{chunkStreamID: msg.ChunkStreamID}
		cs.writeStreamStates[msg.ChunkStreamID] = state
	}

	for bytesRemaining > 0 {
		chunkSize := cs.writeChunkSize
		if bytesRemaining < chunkSize {
			chunkSize = bytesRemaining
		}

		var format uint8
		var timestamp uint32

		if isFirstChunk {
			// Determine format based on state
			if state.messageStreamID != msg.MessageStreamID ||
				state.messageLength != msg.Length ||
				state.messageTypeID != uint8(msg.Type) {
				format = 0 // Type 0: Full header
				timestamp = msg.Timestamp
			} else if state.timestamp != msg.Timestamp {
				format = 1 // Type 1: Same stream ID
				timestamp = msg.Timestamp - state.timestamp
			} else {
				format = 2 // Type 2: Same delta
				timestamp = 0
			}
			isFirstChunk = false
		} else {
			format = 3 // Type 3: No header
			timestamp = 0
		}

		header := &ChunkHeader{
			Format:          format,
			ChunkStreamID:   msg.ChunkStreamID,
			Timestamp:       timestamp,
			MessageLength:   msg.Length,
			MessageTypeID:   uint8(msg.Type),
			MessageStreamID: msg.MessageStreamID,
		}

		// Handle extended timestamp
		if timestamp >= 0xFFFFFF {
			header.ExtendedTimestamp = timestamp
			header.Timestamp = 0xFFFFFF
		}

		chunkData := make([]byte, chunkSize)
		copy(chunkData, data[offset:offset+chunkSize])

		chunks = append(chunks, &Chunk{
			Header: header,
			Data:   chunkData,
		})

		offset += chunkSize
		bytesRemaining -= chunkSize
	}

	// Update state
	state.timestamp = msg.Timestamp
	state.messageLength = msg.Length
	state.messageTypeID = uint8(msg.Type)
	state.messageStreamID = msg.MessageStreamID

	return chunks, nil
}

// Helper methods
func (cs *ChunkStreamer) readBasicHeader() ([]byte, error) {
	basicHeader := make([]byte, 1)
	if _, err := io.ReadFull(cs.conn, basicHeader); err != nil {
		return nil, fmt.Errorf("failed to read basic header: %w", err)
	}

	chunkStreamID := basicHeader[0] & 0x3F

	if chunkStreamID == 0 {
		// 2-byte form
		extraByte := make([]byte, 1)
		if _, err := io.ReadFull(cs.conn, extraByte); err != nil {
			return nil, fmt.Errorf("failed to read basic header extra byte: %w", err)
		}
		return append(basicHeader, extraByte...), nil
	} else if chunkStreamID == 1 {
		// 3-byte form
		extraBytes := make([]byte, 2)
		if _, err := io.ReadFull(cs.conn, extraBytes); err != nil {
			return nil, fmt.Errorf("failed to read basic header extra bytes: %w", err)
		}
		return append(basicHeader, extraBytes...), nil
	}

	return basicHeader, nil
}

func (cs *ChunkStreamer) parseChunkStreamID(basicHeader []byte) uint32 {
	chunkStreamID := uint32(basicHeader[0] & 0x3F)

	if chunkStreamID == 0 {
		return uint32(basicHeader[1]) + 64
	} else if chunkStreamID == 1 {
		return uint32(basicHeader[1]) + uint32(basicHeader[2])*256 + 64
	}

	return chunkStreamID
}

func (cs *ChunkStreamer) readMessageHeader(header *ChunkHeader, state *chunkStreamState) error {
	switch header.Format {
	case 0: // Type 0: 11 bytes
		headerBytes := make([]byte, 11)
		if _, err := io.ReadFull(cs.conn, headerBytes); err != nil {
			return fmt.Errorf("failed to read type 0 header: %w", err)
		}

		header.Timestamp = uint32(headerBytes[0])<<16 | uint32(headerBytes[1])<<8 | uint32(headerBytes[2])
		header.MessageLength = uint32(headerBytes[3])<<16 | uint32(headerBytes[4])<<8 | uint32(headerBytes[5])
		header.MessageTypeID = headerBytes[6]
		header.MessageStreamID = uint32(headerBytes[7]) | uint32(headerBytes[8])<<8 |
			uint32(headerBytes[9])<<16 | uint32(headerBytes[10])<<24

		if header.Timestamp == 0xFFFFFF {
			extBytes := make([]byte, 4)
			if _, err := io.ReadFull(cs.conn, extBytes); err != nil {
				return fmt.Errorf("failed to read extended timestamp: %w", err)
			}
			header.ExtendedTimestamp = uint32(extBytes[0])<<24 | uint32(extBytes[1])<<16 |
				uint32(extBytes[2])<<8 | uint32(extBytes[3])
			header.Timestamp = header.ExtendedTimestamp
		}

	case 1: // Type 1: 7 bytes
		headerBytes := make([]byte, 7)
		if _, err := io.ReadFull(cs.conn, headerBytes); err != nil {
			return fmt.Errorf("failed to read type 1 header: %w", err)
		}

		header.TimestampDelta = uint32(headerBytes[0])<<16 | uint32(headerBytes[1])<<8 | uint32(headerBytes[2])
		header.MessageLength = uint32(headerBytes[3])<<16 | uint32(headerBytes[4])<<8 | uint32(headerBytes[5])
		header.MessageTypeID = headerBytes[6]
		header.MessageStreamID = state.messageStreamID
		header.Timestamp = state.timestamp + header.TimestampDelta

	case 2: // Type 2: 3 bytes
		headerBytes := make([]byte, 3)
		if _, err := io.ReadFull(cs.conn, headerBytes); err != nil {
			return fmt.Errorf("failed to read type 2 header: %w", err)
		}

		header.TimestampDelta = uint32(headerBytes[0])<<16 | uint32(headerBytes[1])<<8 | uint32(headerBytes[2])
		header.MessageLength = state.messageLength
		header.MessageTypeID = state.messageTypeID
		header.MessageStreamID = state.messageStreamID
		header.Timestamp = state.timestamp + header.TimestampDelta

	case 3: // Type 3: 0 bytes
		header.TimestampDelta = state.timestampDelta
		header.MessageLength = state.messageLength
		header.MessageTypeID = state.messageTypeID
		header.MessageStreamID = state.messageStreamID
		header.Timestamp = state.timestamp + header.TimestampDelta
	}

	// Update state
	state.timestamp = header.Timestamp
	state.messageLength = header.MessageLength
	state.messageTypeID = header.MessageTypeID
	state.messageStreamID = header.MessageStreamID
	state.timestampDelta = header.TimestampDelta

	return nil
}

func (cs *ChunkStreamer) createBasicHeader(format uint8, chunkStreamID uint32) []byte {
	if chunkStreamID < 64 {
		return []byte{(format << 6) | byte(chunkStreamID)}
	} else if chunkStreamID < 320 {
		return []byte{format << 6, byte(chunkStreamID - 64)}
	} else {
		id := chunkStreamID - 64
		return []byte{(format << 6) | 1, byte(id & 0xFF), byte(id >> 8)}
	}
}

func (cs *ChunkStreamer) createMessageHeader(header *ChunkHeader) []byte {
	switch header.Format {
	case 0: // Type 0: 11 bytes
		msgHeader := make([]byte, 11)
		timestamp := header.Timestamp
		if timestamp >= 0xFFFFFF {
			timestamp = 0xFFFFFF
		}

		msgHeader[0] = byte(timestamp >> 16)
		msgHeader[1] = byte(timestamp >> 8)
		msgHeader[2] = byte(timestamp)
		msgHeader[3] = byte(header.MessageLength >> 16)
		msgHeader[4] = byte(header.MessageLength >> 8)
		msgHeader[5] = byte(header.MessageLength)
		msgHeader[6] = header.MessageTypeID
		msgHeader[7] = byte(header.MessageStreamID)
		msgHeader[8] = byte(header.MessageStreamID >> 8)
		msgHeader[9] = byte(header.MessageStreamID >> 16)
		msgHeader[10] = byte(header.MessageStreamID >> 24)

		return msgHeader

	case 1: // Type 1: 7 bytes
		msgHeader := make([]byte, 7)
		msgHeader[0] = byte(header.TimestampDelta >> 16)
		msgHeader[1] = byte(header.TimestampDelta >> 8)
		msgHeader[2] = byte(header.TimestampDelta)
		msgHeader[3] = byte(header.MessageLength >> 16)
		msgHeader[4] = byte(header.MessageLength >> 8)
		msgHeader[5] = byte(header.MessageLength)
		msgHeader[6] = header.MessageTypeID

		return msgHeader

	case 2: // Type 2: 3 bytes
		msgHeader := make([]byte, 3)
		msgHeader[0] = byte(header.TimestampDelta >> 16)
		msgHeader[1] = byte(header.TimestampDelta >> 8)
		msgHeader[2] = byte(header.TimestampDelta)

		return msgHeader

	case 3: // Type 3: 0 bytes
		return []byte{}

	default:
		return []byte{}
	}
}

func (cs *ChunkStreamer) calculateChunkDataSize(header *ChunkHeader, state *chunkStreamState) uint32 {
	messageLength := header.MessageLength
	if messageLength == 0 {
		messageLength = state.messageLength
	}

	incomplete, exists := cs.incompleteMessages[header.ChunkStreamID]
	if !exists {
		// First chunk of message
		if messageLength <= cs.readChunkSize {
			return messageLength
		}
		return cs.readChunkSize
	}

	// Subsequent chunk
	remaining := messageLength - incomplete.bytesRead
	if remaining <= cs.readChunkSize {
		return remaining
	}
	return cs.readChunkSize
}

func (cs *ChunkStreamer) assembleMessage(chunk *Chunk) (*Message, bool, error) {
	chunkStreamID := chunk.Header.ChunkStreamID

	incomplete, exists := cs.incompleteMessages[chunkStreamID]
	if !exists {
		// First chunk of message
		msg := &Message{
			ChunkStreamID:   chunkStreamID,
			MessageStreamID: chunk.Header.MessageStreamID,
			Timestamp:       chunk.Header.Timestamp,
			Type:            MessageType(chunk.Header.MessageTypeID),
			Length:          chunk.Header.MessageLength,
			Data:            make([]byte, chunk.Header.MessageLength),
		}

		copy(msg.Data, chunk.Data)

		if uint32(len(chunk.Data)) >= chunk.Header.MessageLength {
			// Message complete
			return msg, true, nil
		}

		// Message incomplete
		cs.incompleteMessages[chunkStreamID] = &incompleteMessage{
			message:   msg,
			bytesRead: uint32(len(chunk.Data)),
		}
		return nil, false, nil
	}

	// Subsequent chunk
	copy(incomplete.message.Data[incomplete.bytesRead:], chunk.Data)
	incomplete.bytesRead += uint32(len(chunk.Data))

	if incomplete.bytesRead >= incomplete.message.Length {
		// Message complete
		msg := incomplete.message
		delete(cs.incompleteMessages, chunkStreamID)
		return msg, true, nil
	}

	// Message still incomplete
	return nil, false, nil
}

func (cs *ChunkStreamer) getChunkHeaderSize(format uint8) uint32 {
	switch format {
	case 0:
		return 12 // 1 + 11
	case 1:
		return 8 // 1 + 7
	case 2:
		return 4 // 1 + 3
	case 3:
		return 1 // 1 + 0
	default:
		return 1
	}
}

func (cs *ChunkStreamer) getNextChunkStreamID() uint32 {
	id := cs.nextWriteChunkID
	cs.nextWriteChunkID++
	if cs.nextWriteChunkID > 65535 {
		cs.nextWriteChunkID = 2 // Reset but skip reserved IDs
	}
	return id
}

func (cs *ChunkStreamer) updateBytesReceived(bytes uint32) {
	cs.bytesReceived += bytes

	// Send acknowledgement if needed
	if cs.windowAckSize > 0 && cs.bytesReceived-cs.lastAckSent >= cs.windowAckSize {
		// This would send an acknowledgement message
		// Implementation depends on the message sender
		cs.lastAckSent = cs.bytesReceived
	}
}
