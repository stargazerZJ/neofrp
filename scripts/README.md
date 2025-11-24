# FRP Helper Scripts

## frps-reload.sh

A helper script that runs `frps` in a continuous loop, enabling hot-reload of configuration and binary updates without manual intervention.

### Features

- Automatically restarts `frps` when it exits
- Enables config/binary reload via HTTP endpoint
- Logs restart events with timestamps
- Configurable via environment variables

### Usage

Basic usage:
```bash
./scripts/frps-reload.sh
```

With custom configuration:
```bash
FRPS_CONFIG=./custom-config.toml ./scripts/frps-reload.sh
```

With custom binary path:
```bash
FRPS_BIN=/usr/local/bin/frps ./scripts/frps-reload.sh
```

### Environment Variables

- `FRPS_BIN` - Path to frps binary (default: `./bin/frps`)
- `FRPS_CONFIG` - Path to config file (default: `./frps.toml`)
- `FRPS_PORT` - Server port for reload endpoint (default: `7000`)

### Reloading Configuration or Binary

To reload the configuration or update the binary:

1. Update the config file or replace the binary
2. Trigger a reload:
   ```bash
   curl http://localhost:7000/shutdown
   ```
3. The script will automatically restart frps with the new config/binary

### Health Check

Check if the server is running:
```bash
curl http://localhost:7000/health
```

Expected response: `ok`

### Stopping the Server

To stop the server completely, press `Ctrl+C` in the terminal running the script.