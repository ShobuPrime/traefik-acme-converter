# Docker Hub Multi-Architecture Setup

This repository contains two Dockerfiles for building multi-architecture images:

- `Dockerfile` - Builds for **AMD64** (x86_64)
- `Dockerfile.arm64` - Builds for **ARM64** (aarch64/Raspberry Pi)

## Docker Hub Configuration

To enable multi-architecture support in Docker Hub automated builds, configure the following build rules:

### Build Rules

1. **AMD64 Build:**
   - **Source Type:** Branch
   - **Source:** `main`
   - **Docker Tag:** `latest-amd64`
   - **Dockerfile location:** `Dockerfile`
   - **Build Context:** `/`
   - **Autobuild:** Enabled

2. **ARM64 Build:**
   - **Source Type:** Branch
   - **Source:** `main`
   - **Docker Tag:** `latest-arm64`
   - **Dockerfile location:** `Dockerfile.arm64`
   - **Build Context:** `/`
   - **Autobuild:** Enabled

3. **Latest Manifest (AMD64 default):**
   - **Source Type:** Branch
   - **Source:** `main`
   - **Docker Tag:** `latest`
   - **Dockerfile location:** `Dockerfile`
   - **Build Context:** `/`
   - **Autobuild:** Enabled

## Usage

Users can pull architecture-specific images or let Docker automatically select the right one:

```bash
# Automatic (recommended) - Docker pulls the correct architecture
docker pull shobuprime/traefik-acme-converter:latest

# Explicit AMD64
docker pull shobuprime/traefik-acme-converter:latest-amd64

# Explicit ARM64
docker pull shobuprime/traefik-acme-converter:latest-arm64
```

## Creating a Multi-Arch Manifest (Optional)

If you want a true multi-arch manifest where `latest` works for both architectures, you can create it manually using Docker CLI:

```bash
# Enable experimental features
export DOCKER_CLI_EXPERIMENTAL=enabled

# Create manifest
docker manifest create shobuprime/traefik-acme-converter:latest \
  shobuprime/traefik-acme-converter:latest-amd64 \
  shobuprime/traefik-acme-converter:latest-arm64

# Annotate platforms
docker manifest annotate shobuprime/traefik-acme-converter:latest \
  shobuprime/traefik-acme-converter:latest-amd64 \
  --os linux --arch amd64

docker manifest annotate shobuprime/traefik-acme-converter:latest \
  shobuprime/traefik-acme-converter:latest-arm64 \
  --os linux --arch arm64

# Push manifest
docker manifest push shobuprime/traefik-acme-converter:latest
```

This needs to be done after each Docker Hub build completes.
