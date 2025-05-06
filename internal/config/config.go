package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the configuration for the application.
type Config struct {
	Interval                 int
	ImageURL                 string
	ImageCrop                *[]int
	MQTTHost                 string
	MQTTTopic                string
	MQTTClientID             string
	MQTTUsername             string
	MQTTPassword             string
	HASSAutoDiscoveryEnabled bool
	HASSAutoDiscoveryTopic   string
	HASSName                 string
}

// Load initializes the configuration by loading environment variables and setting up the MQTT client.
func Load() (*Config, error) {
	envVars := map[string]*string{
		"IMAGE_URL":                   nil,
		"INTERVAL":                    &[]string{"60"}[0],
		"MQTT_HOST":                   nil,
		"MQTT_TOPIC":                  &[]string{"darkdetector"}[0],
		"MQTT_CLIENT_ID":              &[]string{"darkdetector"}[0],
		"HASS_AUTO_DISCOVERY_ENABLED": &[]string{"true"}[0],
		"HASS_AUTO_DISCOVERY_TOPIC":   &[]string{"homeassistant"}[0],
		"HASS_NAME":                   &[]string{"Light Sensor"}[0],
	}

	if err := validateEnvVars(envVars); err != nil {
		return nil, err
	}

	interval, err := strconv.Atoi(*envVars["INTERVAL"])
	if err != nil {
		return nil, fmt.Errorf("error parsing INTERVAL: %v", err)
	}

	mqttHost := buildMQTTHost(*envVars["MQTT_HOST"])

	imageCrop, err := getImageCrop()
	if err != nil {
		return nil, fmt.Errorf("error parsing IMAGE_CROP: %v", err)
	}

	config := &Config{
		ImageURL:                 *envVars["IMAGE_URL"],
		ImageCrop:                imageCrop,
		Interval:                 interval,
		MQTTHost:                 mqttHost,
		MQTTTopic:                *envVars["MQTT_TOPIC"],
		MQTTClientID:             *envVars["MQTT_CLIENT_ID"],
		MQTTUsername:             os.Getenv("MQTT_USERNAME"),
		MQTTPassword:             os.Getenv("MQTT_PASSWORD"),
		HASSAutoDiscoveryEnabled: strings.EqualFold(*envVars["HASS_AUTO_DISCOVERY_ENABLED"], "true"),
		HASSAutoDiscoveryTopic:   *envVars["HASS_AUTO_DISCOVERY_TOPIC"],
		HASSName:                 *envVars["HASS_NAME"],
	}

	return config, nil
}

func getImageCrop() (*[]int, error) {
	value := os.Getenv("IMAGE_CROP")
	if value == "" {
		return nil, nil
	}

	values := strings.Split(value, ",")
	crop := make([]int, 0)
	for _, v := range values {
		intVal, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("error parsing IMAGE_CROP value: %v", err)
		}
		crop = append(crop, intVal)
	}

	return &crop, nil
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
