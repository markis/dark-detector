package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds the configuration for the application.
type Config struct {
	Interval     int
	ImageURL     string
	MQTTHost     string
	MQTTTopic    string
	MQTTClientID string
	MQTTUsername string
	MQTTPassword string
	HAName       string
}

// Load initializes the configuration by loading environment variables and setting up the MQTT client.
func Load() (*Config, error) {
	envVars := map[string]*string{
		"IMAGE_URL":      nil,
		"INTERVAL":       &[]string{"60"}[0],
		"MQTT_HOST":      nil,
		"MQTT_TOPIC":     nil,
		"MQTT_CLIENT_ID": &[]string{"dark-detector"}[0],
		"HA_NAME":        &[]string{"Light Sensor"}[0],
	}

	if err := validateEnvVars(envVars); err != nil {
		return nil, err
	}

	interval, err := strconv.Atoi(*envVars["INTERVAL"])
	if err != nil {
		return nil, fmt.Errorf("error parsing INTERVAL: %v", err)
	}

	mqttHost := buildMQTTHost(*envVars["MQTT_HOST"])

	config := &Config{
		ImageURL:     *envVars["IMAGE_URL"],
		Interval:     interval,
		MQTTHost:     mqttHost,
		MQTTTopic:    *envVars["MQTT_TOPIC"],
		MQTTClientID: *envVars["MQTT_CLIENT_ID"],
		MQTTUsername: os.Getenv("MQTT_USERNAME"),
		MQTTPassword: os.Getenv("MQTT_PASSWORD"),
	}

	// if err := setupMQTTClient(config); err != nil {
	// 	return nil, fmt.Errorf("failed to setup MQTT client: %w", err)
	// }

	return config, nil
}

// validateEnvVars checks if required environment variables are set and assigns them to the config struct.
func validateEnvVars(envVars map[string]*string) error {
	for key, defaultVal := range envVars {
		if value := os.Getenv(key); value != "" {
			envVars[key] = &value
		} else if defaultVal == nil {
			return fmt.Errorf("%s environment variable is not set", key)
		}
	}
	return nil
}

// buildMQTTHost constructs the MQTT host string with the port (default port 1883).
func buildMQTTHost(mqttHost string) string {
	if mqttPort := os.Getenv("MQTT_PORT"); mqttPort != "" {
		return fmt.Sprintf("%s:%s", mqttHost, mqttPort)
	}
	return fmt.Sprintf("%s:1883", mqttHost)
}
