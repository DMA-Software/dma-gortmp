// Package main provides a simple RTMP server example.
// This demonstrates how to use the dma-gortmp library to create an RTMP server
// that can accept client connections, handle publish/play requests, and manage streams.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/DMA-Software/dma-gortmp/pkg/rtmp"
)

// Config holds the server configuration
type Config struct {
	Addr             string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	HandshakeTimeout time.Duration
	MaxConnections   int
	AllowedApps      []string
	RequireAuth      bool
	Username         string
	Password         string
}

// StreamManager manages active streams
type StreamManager struct {
	streams map[string]*StreamInfo
	mutex   sync.RWMutex
}

// StreamInfo holds information about an active stream
type StreamInfo struct {
	Name        string
	Publisher   *rtmp.ServerConnection
	Subscribers []*rtmp.ServerConnection
	StartTime   time.Time
	Active      bool
	Metadata    map[string]interface{}
}

// NewStreamManager creates a new stream manager
func NewStreamManager() *StreamManager {
	return &StreamManager{
		streams: make(map[string]*StreamInfo),
	}
}

// AddStream adds a new stream
func (sm *StreamManager) AddStream(name string, publisher *rtmp.ServerConnection) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.streams[name] = &StreamInfo{
		Name:        name,
		Publisher:   publisher,
		Subscribers: make([]*rtmp.ServerConnection, 0),
		StartTime:   time.Now(),
		Active:      true,
		Metadata:    make(map[string]interface{}),
	}

	log.Printf("Stream added: %s (publisher: %s)", name, publisher.ID())
}

// RemoveStream removes a stream
func (sm *StreamManager) RemoveStream(name string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if stream, exists := sm.streams[name]; exists {
		stream.Active = false
		// Notify all subscribers that stream ended
		for _, subscriber := range stream.Subscribers {
			log.Printf("Notifying subscriber %s that stream %s ended", subscriber.ID(), name)
		}
		delete(sm.streams, name)
		log.Printf("Stream removed: %s", name)
	}
}

// GetStream gets stream information
func (sm *StreamManager) GetStream(name string) (*StreamInfo, bool) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	stream, exists := sm.streams[name]
	return stream, exists
}

// AddSubscriber adds a subscriber to a stream
func (sm *StreamManager) AddSubscriber(streamName string, subscriber *rtmp.ServerConnection) bool {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.streams[streamName]
	if !exists || !stream.Active {
		return false
	}

	stream.Subscribers = append(stream.Subscribers, subscriber)
	log.Printf("Subscriber added to stream %s: %s", streamName, subscriber.ID())
	return true
}

// RemoveSubscriber removes a subscriber from a stream
func (sm *StreamManager) RemoveSubscriber(streamName string, subscriber *rtmp.ServerConnection) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	stream, exists := sm.streams[streamName]
	if !exists {
		return
	}

	// Remove subscriber from list
	for i, sub := range stream.Subscribers {
		if sub.ID() == subscriber.ID() {
			stream.Subscribers = append(stream.Subscribers[:i], stream.Subscribers[i+1:]...)
			log.Printf("Subscriber removed from stream %s: %s", streamName, subscriber.ID())
			break
		}
	}
}

// ListStreams returns all active streams
func (sm *StreamManager) ListStreams() map[string]*StreamInfo {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	result := make(map[string]*StreamInfo)
	for name, stream := range sm.streams {
		if stream.Active {
			result[name] = stream
		}
	}
	return result
}

// Global stream manager
var streamManager = NewStreamManager()

func main() {
	// Parse command line arguments
	config := parseFlags()

	// Create server configuration
	serverConfig := &rtmp.ServerConfig{
		Addr:             config.Addr,
		ReadTimeout:      config.ReadTimeout,
		WriteTimeout:     config.WriteTimeout,
		HandshakeTimeout: config.HandshakeTimeout,
		ChunkSize:        128,
		WindowAckSize:    2500000,
		MaxConnections:   config.MaxConnections,
		OnConnect:        onConnect,
		OnDisconnect:     onDisconnect,
		OnPublish:        onPublish,
		OnPlay:           onPlay,
		OnMessage:        onMessage,
	}

	// Create RTMP server
	server := rtmp.NewServer(serverConfig)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, stopping server...")
		cancel()
		err := server.Stop()
		if err != nil {
			return
		}
	}()

	// Start the server
	log.Printf("Starting RTMP server on %s", config.Addr)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Print server status
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				connections := server.GetConnections()
				streams := streamManager.ListStreams()
				log.Printf("Server status: %d connections, %d active streams",
					len(connections), len(streams))

				for name, stream := range streams {
					log.Printf("  Stream: %s, Publisher: %s, Subscribers: %d, Duration: %s",
						name, stream.Publisher.ID(), len(stream.Subscribers),
						time.Since(stream.StartTime).Truncate(time.Second))
				}
			}
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	log.Println("Server stopped")
}

// parseFlags parses command line arguments
func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.Addr, "addr", ":1935", "Server listen address")
	flag.DurationVar(&config.ReadTimeout, "read-timeout", 30*time.Second, "Read timeout")
	flag.DurationVar(&config.WriteTimeout, "write-timeout", 30*time.Second, "Write timeout")
	flag.DurationVar(&config.HandshakeTimeout, "handshake-timeout", 10*time.Second, "Handshake timeout")
	flag.IntVar(&config.MaxConnections, "max-connections", 1000, "Maximum concurrent connections")

	var allowedAppsStr string
	flag.StringVar(&allowedAppsStr, "allowed-apps", "live,play", "Comma-separated list of allowed application names")

	flag.BoolVar(&config.RequireAuth, "require-auth", false, "Require authentication")
	flag.StringVar(&config.Username, "username", "admin", "Username for authentication")
	flag.StringVar(&config.Password, "password", "password", "Password for authentication")

	flag.Parse()

	// Parse allowed apps
	if allowedAppsStr != "" {
		config.AllowedApps = strings.Split(allowedAppsStr, ",")
		for i, app := range config.AllowedApps {
			config.AllowedApps[i] = strings.TrimSpace(app)
		}
	}

	return config
}

// Connection event handlers

func onConnect(conn *rtmp.ServerConnection) error {
	log.Printf("Client connected: %s from %s", conn.ID(), conn.RemoteAddr().String())
	return nil
}

func onDisconnect(conn *rtmp.ServerConnection) {
	log.Printf("Client disconnected: %s", conn.ID())

	// Remove from any streams
	streams := streamManager.ListStreams()
	for name, stream := range streams {
		if stream.Publisher != nil && stream.Publisher.ID() == conn.ID() {
			streamManager.RemoveStream(name)
		} else {
			streamManager.RemoveSubscriber(name, conn)
		}
	}
}

func onPublish(conn *rtmp.ServerConnection, streamName string, streamType rtmp.StreamType) error {
	log.Printf("Publish request: %s wants to publish stream '%s' (type: %s)",
		conn.ID(), streamName, streamType)

	// Check if stream already exists
	if stream, exists := streamManager.GetStream(streamName); exists {
		if stream.Active {
			log.Printf("Stream '%s' is already being published by %s",
				streamName, stream.Publisher.ID())
			return fmt.Errorf("stream already exists")
		}
	}

	// Add the stream
	streamManager.AddStream(streamName, conn)

	log.Printf("Stream '%s' publish started by %s", streamName, conn.ID())
	return nil
}

func onPlay(conn *rtmp.ServerConnection, streamName string) error {
	log.Printf("Play request: %s wants to play stream '%s'", conn.ID(), streamName)

	// Check if stream exists
	stream, exists := streamManager.GetStream(streamName)
	if !exists || !stream.Active {
		log.Printf("Stream '%s' not found or inactive", streamName)
		return fmt.Errorf("stream not found")
	}

	// Add subscriber
	if !streamManager.AddSubscriber(streamName, conn) {
		log.Printf("Failed to add subscriber to stream '%s'", streamName)
		return fmt.Errorf("failed to subscribe to stream")
	}

	log.Printf("Stream '%s' play started by %s", streamName, conn.ID())
	return nil
}

func onMessage(conn *rtmp.ServerConnection, msg *rtmp.Message) error {
	// Log message for debugging (in production, you might want to be more selective)
	if msg.Type == rtmp.MessageTypeAudio || msg.Type == rtmp.MessageTypeVideo {
		// For audio/video messages, just log periodically to avoid spam
		if time.Now().Unix()%10 == 0 { // Log every 10 seconds
			log.Printf("Received %s message from %s: %d bytes",
				msg.Type.String(), conn.ID(), msg.Length)
		}

		// Forward to subscribers
		return forwardToSubscribers(conn, msg)
	}

	// For other message types, log them
	log.Printf("Received message from %s: Type=%s, Length=%d, ChunkStreamID=%d, MessageStreamID=%d",
		conn.ID(), msg.Type.String(), msg.Length, msg.ChunkStreamID, msg.MessageStreamID)

	// Handle specific message types
	switch msg.Type {
	case rtmp.MessageTypeDataMessageAMF0, rtmp.MessageTypeDataMessageAMF3:
		// Handle metadata
		return handleMetadata(conn, msg)
	default:
		// For other message types, just acknowledge receipt
		return nil
	}
}

// forwardToSubscribers forwards audio/video messages to all subscribers of the stream
func forwardToSubscribers(publisher *rtmp.ServerConnection, msg *rtmp.Message) error {
	// Find the stream being published by this connection
	streams := streamManager.ListStreams()
	var publisherStream *StreamInfo
	var publisherStreamName string

	for name, stream := range streams {
		if stream.Publisher != nil && stream.Publisher.ID() == publisher.ID() {
			publisherStream = stream
			publisherStreamName = name
			break
		}
	}

	if publisherStream == nil {
		// No stream found for this publisher
		return nil
	}

	// Forward message to all subscribers
	for _, subscriber := range publisherStream.Subscribers {
		if subscriber.ID() != publisher.ID() {
			if err := subscriber.WriteMessage(msg); err != nil {
				log.Printf("Failed to forward message to subscriber %s: %v", subscriber.ID(), err)
				// Remove failed subscriber
				streamManager.RemoveSubscriber(publisherStreamName, subscriber)
			}
		}
	}

	return nil
}

// handleMetadata handles metadata messages
func handleMetadata(conn *rtmp.ServerConnection, msg *rtmp.Message) error {
	log.Printf("Received metadata from %s: %d bytes", conn.ID(), msg.Length)

	// Find the stream for this publisher
	streams := streamManager.ListStreams()
	for _, stream := range streams {
		if stream.Publisher != nil && stream.Publisher.ID() == conn.ID() {
			// Store metadata (in a real implementation, you might parse the AMF data)
			stream.Metadata["lastUpdate"] = time.Now()
			stream.Metadata["dataSize"] = msg.Length

			// Forward metadata to subscribers
			return forwardToSubscribers(conn, msg)
		}
	}

	return nil
}

// Example usage:
//
// Basic server:
// go run main.go
//
// Custom address and timeouts:
// go run main.go -addr :1935 -read-timeout 60s -write-timeout 60s
//
// With authentication:
// go run main.go -require-auth -username myuser -password mypass
//
// Limited connections and specific apps:
// go run main.go -max-connections 100 -allowed-apps "live,stream,broadcast"
