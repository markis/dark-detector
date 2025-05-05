# Dark Detector

A Go-based service that monitors ambient light levels and publishes the readings to Home Assistant via MQTT. This project is designed to help automate lighting control based on environmental conditions by integrating with Home Assistant's automation system.

## Features

- Ambient light level detection and measurement in lux
- Configurable measurement intervals
- MQTT integration for publishing light readings
- Containerized deployment support

## Configuration

The application requires configuration for:

- MQTT broker connection details
- Measurement interval
- Image processing parameters

Configuration can be provided through environment variables or a configuration file.

### Environment Variables

The following environment variables can be used to configure the application:

| Variable         | Required | Default       | Description                                                                    |
| ---------------- | -------- | ------------- | ------------------------------------------------------------------------------ |
| `IMAGE_URL`      | Yes      | -             | URL of the image to process for light detection                                |
| `INTERVAL`       | No       | 60            | Measurement interval in seconds                                                |
| `IMAGE_CROP`     | No       | -             | Comma-separated list of integers for image cropping (e.g., "x,y,width,height") |
| `MQTT_HOST`      | Yes      | -             | Hostname or IP address of the MQTT broker                                      |
| `MQTT_PORT`      | No       | 1883          | Port number of the MQTT broker                                                 |
| `MQTT_TOPIC`     | Yes      | -             | MQTT topic to publish light readings                                           |
| `MQTT_CLIENT_ID` | No       | dark-detector | Client ID for MQTT connection                                                  |
| `MQTT_USERNAME`  | No       | -             | Username for MQTT authentication                                               |
| `MQTT_PASSWORD`  | No       | -             | Password for MQTT authentication                                               |
| `HA_NAME`        | No       | Light Sensor  | Name of the sensor in Home Assistant                                           |

## Building and Running

### Local Development

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Run the application:
   ```bash
   go run main.go
   ```

### Docker Deployment

Build and run using Docker:

```bash
docker build -t dark-detector .
docker run dark-detector
```

Or using Docker Compose:

```bash
docker-compose up
```

## MQTT Integration

The service publishes light readings to Home Assistant via MQTT using the Home Assistant MQTT discovery protocol. The readings are in lux units and are automatically configured as a light sensor entity in Home Assistant. This integration allows you to:

- Monitor ambient light levels in real-time through Home Assistant
- Create automations based on light levels (e.g., automatically turn on lights when it gets dark)
- View historical light level data in Home Assistant's dashboard
- Integrate with other Home Assistant entities and automations

The sensor will appear in Home Assistant with the name specified in the `HA_NAME` environment variable (defaults to "Light Sensor").

## Contributing

1. Fork the repository
2. Create your feature branch
3. Commit your changes
4. Push to the branch
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
