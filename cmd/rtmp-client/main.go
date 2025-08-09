// Package main provides a simple RTMP client example.
// This demonstrates how to use the dma-gortmp library to connect to an RTMP server,
// authenticate, create a stream, and publish content.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DMA-Software/dma-gortmp/pkg/rtmp"
)

// Config holds the client configuration
type Config struct {
	ServerURL    string
	AppName      string
	StreamName   string
	Username     string
	Password     string
	ConnTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func main() {
	// Parse command line arguments
	config := parseFlags()

	// Create client configuration
	clientConfig := &rtmp.ClientConfig{
		ConnectTimeout: config.ConnTimeout,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		ChunkSize:      128,
		WindowAckSize:  2500000,
		AppName:        config.AppName,
		StreamName:     config.StreamName,
		Username:       config.Username,
		Password:       config.Password,
	}

	// Create RTMP client
	client := rtmp.NewClient(clientConfig)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, closing client...")
		cancel()
	}()

	// Run the client
	if err := runClient(ctx, client, config); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}

// parseFlags parses command line arguments
func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.ServerURL, "url", "rtmp://localhost:1935", "RTMP server URL")
	flag.StringVar(&config.AppName, "app", "live", "Application name")
	flag.StringVar(&config.StreamName, "stream", "test", "Stream name to publish")
	flag.StringVar(&config.Username, "user", "", "Username for authentication (optional)")
	flag.StringVar(&config.Password, "pass", "", "Password for authentication (optional)")
	flag.DurationVar(&config.ConnTimeout, "conn-timeout", 30*time.Second, "Connection timeout")
	flag.DurationVar(&config.ReadTimeout, "read-timeout", 30*time.Second, "Read timeout")
	flag.DurationVar(&config.WriteTimeout, "write-timeout", 30*time.Second, "Write timeout")

	flag.Parse()

	return config
}

// runClient runs the main client logic
func runClient(ctx context.Context, client *rtmp.Client, config *Config) error {
	log.Printf("Connecting to RTMP server: %s", config.ServerURL)

	// Connect to the server
	if err := client.Connect(ctx, config.ServerURL); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer func(client *rtmp.Client) {
		err := client.Close()
		if err != nil {
			log.Printf("Failed to close client: %v", err)
		}
	}(client)

	log.Printf("Connected successfully. Current state: %s", client.State().String())

	// Start publishing
	log.Printf("Starting to publish stream: %s", config.StreamName)
	if err := client.Publish(ctx, config.StreamName, rtmp.StreamTypeLive); err != nil {
		return fmt.Errorf("failed to start publishing: %w", err)
	}

	log.Printf("Publishing started. Current state: %s", client.State().String())

	// Simulate streaming by sending periodic messages
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	messageCount := 0
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, stopping client")
			return nil

		case <-ticker.C:
			messageCount++

			// Create a sample message (in a real application, this would be video/audio data)
			msg := rtmp.NewMessage(
				2, // Chunk stream ID
				1, // Message stream ID
				rtmp.MessageTypeDataMessageAMF0,
				[]byte(fmt.Sprintf("Sample data message #%d at %s", messageCount, time.Now().Format(time.RFC3339))),
			)

			// Send the message
			if err := client.WriteMessage(msg); err != nil {
				log.Printf("Failed to send message: %v", err)
				continue
			}

			log.Printf("Sent message #%d", messageCount)

			// Try to read any incoming messages
			go func() {
				for {
					msg, err := client.ReadMessage()
					if err != nil {
						// This is expected in a publish-only scenario
						return
					}
					log.Printf("Received message: Type=%s, Length=%d", msg.Type.String(), msg.Length)
				}
			}()

		default:
			// Check client state
			state := client.State()
			if state == rtmp.ClientStateError || state == rtmp.ClientStateDisconnected {
				return fmt.Errorf("client entered error state: %s", state.String())
			}

			// Small delay to prevent busy waiting
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Example usage:
//
// Basic usage:
// go run main.go -url rtmp://localhost:1935/live -stream mystream
//
// With authentication:
// go run main.go -url rtmp://localhost:1935/live -stream mystream -user username -pass password
//
// Custom timeouts:
// go run main.go -url rtmp://localhost:1935/live -stream mystream -conn-timeout 60s -read-timeout 45s -write-timeout 45s
