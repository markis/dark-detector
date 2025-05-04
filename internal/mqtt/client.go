package mqtt

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"dark-detector/internal/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Publisher struct {
	client mqtt.Client
	topic  string
}

func NewPublisher(cfg *config.Config) *Publisher {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MQTTHost).
		SetClientID(cfg.MQTTClientID).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(2 * time.Minute).
		SetKeepAlive(time.Minute).
		SetConnectRetry(true).
		SetOnConnectHandler(func(client mqtt.Client) {
			log.Println("Connected to MQTT broker")
		}).
		SetConnectionLostHandler(func(client mqtt.Client, err error) {
			fmt.Printf("Connection to MQTT broker lost: %v", err)
		})

	if cfg.MQTTUsername != "" && cfg.MQTTPassword != "" {
		opts.SetUsername(cfg.MQTTUsername)
		opts.SetPassword(cfg.MQTTPassword)
	}

	log.Printf("Connecting to MQTT broker at %s", cfg.MQTTHost)

	return &Publisher{
		client: mqtt.NewClient(opts),
		topic:  cfg.MQTTTopic,
	}
}

func (p *Publisher) Connect(ctx context.Context) error {
	token := p.client.Connect()

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("MQTT connection cancelled: %w", ctx.Err())
	case <-timer.C:
		return fmt.Errorf("MQTT connection timeout")
	case <-waitForToken(token):
		if err := token.Error(); err != nil {
			return fmt.Errorf("MQTT connection error: %w", err)
		}
		return nil
	}
}

func (p *Publisher) Disconnect() {
	p.client.Disconnect(250)
}

func (p *Publisher) PublishLux(ctx context.Context, lux int) error {
	if !p.client.IsConnected() {
		return fmt.Errorf("mqtt client is not connected")
	}

	// Home Assistant discovery config
	discoveryTopic := "homeassistant/sensor/lux_sensor/config"
	discoveryPayload := `{
		"name": "Light Sensor",
		"device_class": "illuminance",
		"state_topic": "` + p.topic + `",
		"unit_of_measurement": "lx",
		"unique_id": "lux_sensor"
	}`

	// Publish discovery config
	token := p.client.Publish(discoveryTopic, 1, true, discoveryPayload)
	if err := waitForPublish(ctx, token); err != nil {
		return fmt.Errorf("failed to publish discovery config: %w", err)
	}

	// Publish state
	statePayload := strconv.Itoa(lux)
	token = p.client.Publish(p.topic, 1, false, statePayload)
	if err := waitForPublish(ctx, token); err != nil {
		return fmt.Errorf("failed to publish state: %w", err)
	}

	return nil
}

// Helper function to wait for MQTT publish
func waitForPublish(ctx context.Context, token mqtt.Token) error {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("publish cancelled: %w", ctx.Err())
	case <-timer.C:
		return fmt.Errorf("mqtt publish timeout after 10s")
	case <-waitForToken(token):
		if err := token.Error(); err != nil {
			return fmt.Errorf("mqtt publish error: %w", err)
		}
		return nil
	}
}

// Helper function to convert token.Wait() to channel
func waitForToken(token mqtt.Token) chan struct{} {
	done := make(chan struct{})
	go func() {
		token.Wait()
		close(done)
	}()
	return done
}
