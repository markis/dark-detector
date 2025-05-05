package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dark-detector/internal/config"
	"dark-detector/internal/image"
	"dark-detector/internal/mqtt"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and initialize channels
	errChan := make(chan error, 1)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to get config: %v", err)
	}

	processor := image.NewProcessor(cfg)
	publisher := mqtt.NewPublisher(cfg)
	if err := publisher.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", err)
	}
	defer publisher.Disconnect()
	ticker := time.NewTicker(time.Duration(cfg.Interval) * time.Second)
	defer ticker.Stop()

	// Start processing in background
	go runProcessingLoop(ctx, ticker, processor, publisher, errChan)

	// Handle shutdown gracefully
	select {
	case err := <-errChan:
		log.Printf("Error occurred, shutting down: %v", err)
		cancel()
		os.Exit(1)
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully", sig)
		cancel()
		log.Println("Shutdown complete")
	}
}

func runProcessingLoop(
	ctx context.Context,
	ticker *time.Ticker,
	processor *image.Processor,
	publisher *mqtt.Publisher,
	errChan chan<- error,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lux, err := processor.Process(ctx)
			if err != nil {
				errChan <- err
				return
			}
			if err := publisher.PublishLux(ctx, lux); err != nil {
				errChan <- err
				return
			}
		}
	}
}
