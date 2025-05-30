package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"dark-detector/internal/config"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	connectionTimeout = 10 * time.Second
	publishTimeout    = 10 * time.Second
)

// Publisher handles MQTT communication for light sensor data
// including Home Assistant auto-discovery
type Publisher struct {
	client                 mqtt.Client
	topic                  string
	entityName             string
	uniqueID               string
	needToPublishDiscovery bool
	autoDiscoveryTopic     string
	autoDiscoveryEnabled   bool
	availabilityTopic      string
}

// NewPublisher creates a configured MQTT client with automatic
// reconnection and QoS 1 support
func NewPublisher(cfg *config.Config) *Publisher {
	entityName := cfg.HASSName
	uniqueId := strings.ToLower(strings.ReplaceAll(entityName, " ", "_"))
	topic := fmt.Sprintf("%s/%s/state", cfg.MQTTTopic, uniqueId)
	availabilityTopic := fmt.Sprintf("%s/%s/availability", cfg.MQTTTopic, uniqueId)
	clientID := fmt.Sprintf("%s-%s", cfg.MQTTClientID, uniqueId)

	p := &Publisher{
		topic:                  topic,
		entityName:             entityName,
		uniqueID:               uniqueId,
		needToPublishDiscovery: true,
		autoDiscoveryTopic:     cfg.HASSAutoDiscoveryTopic,
		autoDiscoveryEnabled:   cfg.HASSAutoDiscoveryEnabled,
		availabilityTopic:      availabilityTopic,
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MQTTHost).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetMaxReconnectInterval(2*time.Minute).
		SetKeepAlive(30*time.Second).
		SetConnectRetry(true).
		SetCleanSession(true).
		SetOrderMatters(false).
		SetWill(availabilityTopic, "offline", 2, true).
		SetOnConnectHandler(func(client mqtt.Client) {
			log.Println("Connected to MQTT broker")
			// Publish online status
			if token := client.Publish(availabilityTopic, 2, true, "online"); token.Wait() && token.Error() != nil {
				log.Printf("Failed to publish online status: %v", token.Error())
			}
			if err := p.SubscribeHomeAssistantStatus(context.Background(), func() {
				p.needToPublishDiscovery = true
			}); err != nil {
				log.Printf("Failed to subscribe to HA status: %v", err)
			}
		}).
		SetConnectionLostHandler(func(client mqtt.Client, err error) {
			log.Printf("Connection to MQTT broker lost: %v", err)
		})

	if cfg.MQTTUsername != "" && cfg.MQTTPassword != "" {
		opts.SetUsername(cfg.MQTTUsername)
		opts.SetPassword(cfg.MQTTPassword)
	}

	p.client = mqtt.NewClient(opts)
	return p
}

func (p *Publisher) Connect(ctx context.Context) error {
	token := p.client.Connect()

	timer := time.NewTimer(connectionTimeout)
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
	// Publish offline status manually
	token := p.client.Publish(p.availabilityTopic, 2, true, "offline")
	token.Wait()
	p.client.Disconnect(250)
}

type DiscoveryPayload struct {
	Name              string                 `json:"name"`
	DeviceClass       string                 `json:"device_class"`
	StateTopic        string                 `json:"state_topic"`
	UnitOfMeasurement string                 `json:"unit_of_measurement"`
	UniqueID          string                 `json:"unique_id"`
	AvailabilityTopic string                 `json:"availability_topic"`
	Device            DiscoveryPayloadDevice `json:"device"`
	HasEntityName     bool                   `json:"has_entity_name"`
}

type DiscoveryPayloadDevice struct {
	Name         string `json:"name"`
	Identifiers  string `json:"identifiers"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
}

func (p *Publisher) PublishLux(ctx context.Context, lux int) error {
	// Publish state
	statePayload := strconv.Itoa(lux)
	token := p.client.Publish(p.topic, 1, false, statePayload)
	if err := waitForPublish(ctx, token); err != nil {
		return fmt.Errorf("failed to publish state: %w", err)
	}

	return p.PublishDiscovery(ctx)
}

func (p *Publisher) PublishDiscovery(ctx context.Context) error {
	if !p.autoDiscoveryEnabled || !p.needToPublishDiscovery {
		return nil
	}

	// Home Assistant discovery config
	discoveryTopic := fmt.Sprintf("%s/sensor/%s/config", p.autoDiscoveryTopic, p.uniqueID)
	payload := DiscoveryPayload{
		Name:              p.entityName,
		DeviceClass:       "illuminance",
		StateTopic:        p.topic,
		UnitOfMeasurement: "lx",
		UniqueID:          p.uniqueID,
		AvailabilityTopic: p.availabilityTopic,
		HasEntityName:     true,
		Device: DiscoveryPayloadDevice{
			Name:         "Dark Detector",
			Identifiers:  p.uniqueID,
			Manufacturer: "Markis Taylor",
			Model:        "darkdetector",
		},
	}
	discoveryPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal discovery payload: %w", err)
	}

	// Publish discovery config
	token := p.client.Publish(discoveryTopic, 1, true, discoveryPayload)
	if err := waitForPublish(ctx, token); err != nil {
		return fmt.Errorf("failed to publish discovery config: %w", err)
	}

	p.needToPublishDiscovery = false
	return nil
}

func (p *Publisher) SubscribeHomeAssistantStatus(ctx context.Context, onOnline func()) error {
	if !p.autoDiscoveryEnabled {
		return nil
	}

	topic := fmt.Sprintf("%s/status", p.autoDiscoveryTopic)
	qos := byte(1)

	token := p.client.Subscribe(topic, qos, func(client mqtt.Client, msg mqtt.Message) {
		payload := string(msg.Payload())
		if payload == "online" {
			log.Println("Home Assistant is online. Re-publishing discovery config.")
			onOnline()
		}
	})

	if err := waitForPublish(ctx, token); err != nil {
		return fmt.Errorf("failed to subscribe to Home Assistant status: %w", err)
	}
	return nil
}

// Helper function to wait for MQTT publish
func waitForPublish(ctx context.Context, token mqtt.Token) error {
	timer := time.NewTimer(publishTimeout)
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
