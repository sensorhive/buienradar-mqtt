# buienradar-mqtt

Small Golang program to stuff data from the [buienradar.nl](https://buienradar.nl) API into MQTT on a schedule.

## installation

### container

This will start a long-running container with `buienradar-mqtt` in it which runs all
available subcommands. The container is available for `amd64` and `aarch64`.

```
podman run \
    -e MQTT_HOST="tcp://hostname:1883" \
    -e MQTT_PREFIX="your-prefix" \
    -e MQTT_TOPIC="buienradar" \
    -e BUIENRADAR_REGION="rotterdam" \
    ghcr.io/sensorhive/buienradar-mqtt:latest
```

*The `podman` command can be switched out `docker` if you wish.*

### quadlet

Place the following file in `/etc/containers/systemd`:

```
[Unit]
Description=The buienradar-mqtt container service.
After=local-fs.target

[Container]
Image=ghcr.io/sensorhive/buienradar-mqtt:latest
Environment=MQTT_HOST="tcp://hostname:1883"
Environment=MQTT_PREFIX="your-prefix"
Environment=MQTT_TOPIC="buienradar"
Environment=BUIENRADAR_REGION="rotterdam"
Label=io.containers.autoupdate=registry

[Install]
WantedBy=multi-user.target default.target
```

Followed by a:

```
systemctl daemon-reload
systemctl enable --now buienradar-mqtt
```

## usage 

`buienradar-mqtt` is configured through environment variables.

### `MQTT_HOST`

The MQTT host to publish to.

### `MQTT_PREFIX`

A prefix you would want before your topic. The default is `home.arpa`.

### `MQTT_TOPIC`

The topic you want for the data to be published under. Each value from the API will add its
own `/value` to it.

### `BUIENRADAR_REGION`

The region from Buienradar you want to get data from. See the list in the [API XML](https://data.buienradar.nl/1.0/feed/xml).
This refers to the `region` attribute on the `stationnaam` elements.
