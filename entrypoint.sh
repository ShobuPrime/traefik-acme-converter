#!/bin/sh
set -e

# Entrypoint script for traefik-acme-converter
# Simply runs the application - user switching is handled by Docker's --user flag

echo "Starting traefik-acme-converter (USER_UID=${USER_UID}, USER_GID=${USER_GID})"
exec /app/traefik-acme-converter
