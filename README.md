# traefik-acme-converter
Converts ACME certificates from Traefik into separate `fullchain` and `privkey` files for each domain.

This repository contains a robust Go application packaged as a Docker container that watches for ACME (Let's Encrypt) JSON files from Traefik. It automatically extracts and decodes the full chain certificates and private keys for all domains specified in the JSON file.

## Features

- **Real-time File Watching**: Uses fsnotify for instant certificate updates when ACME files change
- **Polling Mode Fallback**: Can operate in polling mode for environments where file watching isn't reliable
- **Structured Logging**: Clear, timestamped logs with severity levels (INFO, WARN, ERROR, DEBUG)
- **Robust Error Handling**: Graceful handling of malformed JSON, missing files, and extraction errors
- **Multi-certificate Support**: Processes all certificates from a single ACME file
- **Wildcard Domain Support**: Automatically strips wildcard prefixes from domain names
- **Configurable Permissions**: Set custom UID/GID for extracted certificate files
- **Zero Dependencies**: Compiled Go binary with no runtime dependencies
- **Small Footprint**: Multi-stage Docker build resulting in minimal image size
- **Multi-Architecture**: Supports AMD64 (x86_64) and ARM64 (aarch64/Raspberry Pi)

## Prerequisites

- Docker installed on your machine
- Traefik-generated ACME JSON file

## Build

### Pre-built Images (Recommended)

Pull the pre-built image from Docker Hub:

```bash
# For AMD64 (x86_64)
docker pull shobuprime/traefik-acme-converter:latest

# For ARM64 (Raspberry Pi, etc.)
docker pull shobuprime/traefik-acme-converter:latest-arm64
```

### Build Locally

Clone this repository and build the Docker image:

```bash
git clone https://github.com/shobuprime/traefik-acme-converter.git
cd traefik-acme-converter

# For AMD64
docker build -t shobuprime/traefik-acme-converter .

# For ARM64
docker build -f Dockerfile.arm64 -t shobuprime/traefik-acme-converter .
```

## Usage

Run the Docker container:

```bash
docker run -v /path/to/watch:/traefik/certs -v /path/to/output:/app/output shobuprime/traefik-acme-converter
```

### Environment Variables

You can specify several environment variables to customize the behavior:

| Variable | Description | Default |
|----------|-------------|---------|
| `WATCH_DIR` | Directory to watch for the ACME JSON file | `/traefik/certs` |
| `PROVIDER` | Provider name in the ACME JSON structure. The filename is automatically set to `${PROVIDER}.json` | `ACME` (looks for `ACME.json`) |
| `OUTPUT_DIR` | Directory where certificate files will be saved | `/app/output` |
| `USER_UID` | UID to set for extracted certificate files | `1000` |
| `USER_GID` | GID to set for extracted certificate files | `1000` |
| `USE_WATCHER` | Enable real-time file watching (`true`) or polling mode (`false`) | `true` |
| `INTERVAL` | *(Optional)* Seconds between periodic checks. In watcher mode: `0` = disabled (pure fsnotify), `>0` = fallback check for resiliency. In polling mode: required | `0` (watcher), `1800` (polling) |
| `ACME_FILE_NAME` | *(Optional)* Override the ACME JSON filename if it doesn't follow the `${PROVIDER}.json` pattern | `${PROVIDER}.json` |
| `DEBUG` | *(Optional)* Enable debug logging | `false` |

### Example with Environment Variables

```bash
docker run \
  -e WATCH_DIR=/custom/watch/dir \
  -e PROVIDER=cloudflare \
  -e OUTPUT_DIR=/custom/output \
  -e USER_UID=1001 \
  -e USER_GID=1001 \
  -v /path/to/watch:/custom/watch/dir \
  -v /path/to/output:/custom/output \
  shobuprime/traefik-acme-converter
```

**Notes:**
- With `PROVIDER=cloudflare`, the app automatically looks for `cloudflare.json`
- In watcher mode (default), certificates are extracted instantly via fsnotify
- No `INTERVAL` needed - the app relies purely on file system events!

### Docker Compose Example

```yaml
services:
  acme-converter:
    image: shobuprime/traefik-acme-converter
    environment:
      - WATCH_DIR=/certs
      - PROVIDER=cloudflare  # Automatically looks for cloudflare.json
      - OUTPUT_DIR=/app/output
      # Optional: Add INTERVAL=3600 for periodic fallback checks (resiliency)
    volumes:
      - /path/to/traefik/certs:/certs:ro
      - /path/to/output/certs:/app/output
    restart: unless-stopped
```

## Output Structure

Certificates are extracted into subdirectories named after each domain:

```
/app/output/
├── example.com/
│   ├── fullchain.pem
│   └── privkey.pem
├── another-domain.com/
│   ├── fullchain.pem
│   └── privkey.pem
```

Note: Wildcard domains (e.g., `*.example.com`) have their asterisk prefix stripped and are saved as `example.com/`.

## Modes of Operation

### File Watcher Mode (Default)

When `USE_WATCHER=true`, the application uses **fsnotify** to watch for file changes in real-time. Certificates are extracted **instantly** when Traefik renews them—no delay!

**How it works:**
- Watches the directory for file system events
- Processes certificates immediately when ACME file is modified
- Optional: Set `INTERVAL` for periodic fallback checks (catches rare missed events)
- Default: Pure fsnotify mode (no interval) for maximum efficiency

**Advantages:**
- **Instant** certificate extraction (no polling delay)
- Minimal resource usage (event-driven)
- Best for production environments

### Polling Mode

When `USE_WATCHER=false`, the application checks for changes at the specified `INTERVAL`. This is useful in environments where file system events may not work reliably (e.g., some network file systems).

**Advantages:**
- More predictable behavior
- Works on all file systems
- Simpler troubleshooting

## Development

### Building Locally

```bash
# Build the Go binary
go build -o traefik-acme-converter

# Run locally
./traefik-acme-converter

# Run with custom environment
WATCH_DIR=/tmp/certs OUTPUT_DIR=/tmp/output ./traefik-acme-converter
```

### Running Tests

```bash
go test -v ./...
```

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE.md](LICENSE.md) file for details.

---
