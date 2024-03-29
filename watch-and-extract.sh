#!/bin/sh

# Variables with default values
WATCH_DIR="${WATCH_DIR:-/traefik/certs}"
INTERVAL="${INTERVAL:-1800}"
PROVIDER="${PROVIDER:-ACME}"
ACME_FILE_NAME=${PROVIDER}.json
OUTPUT_DIR="${OUTPUT_DIR:-/app/output}"
USER_UID="${USER_UID}:-1000"
USER_GID="${USER_GID}:-1000"

# Loop indefinitely
while true; do

  # Check if the ACME.json file exists in the watch directory
  echo "Checking if \"${WATCH_DIR}/${ACME_FILE_NAME}\" exists..."
  if [ -f "${WATCH_DIR}/${ACME_FILE_NAME}" ]; then

    echo "\"${WATCH_DIR}/${ACME_FILE_NAME}\" found!"

    # Getting the length of the \"Certificates\" array for ${PROVIDER}..."
    CERTIFICATE_COUNT=$(jq -r ".${PROVIDER}.Certificates | length" "${WATCH_DIR}/${ACME_FILE_NAME}")

    echo "Looping through all the domains..."
    for i in $(seq 0 $(($CERTIFICATE_COUNT -1))); do
      DOMAIN=$(jq -r ".${PROVIDER}.Certificates[$i].domain.main" "${WATCH_DIR}/${ACME_FILE_NAME}")
      
      # Check if the domain starts with a wildcard and remove it
      if [ "${DOMAIN:0:2}" = "*." ]; then
        DOMAIN=${DOMAIN:2}
      fi
      
      FULLCHAIN=$(jq -r ".${PROVIDER}.Certificates[$i].certificate" "${WATCH_DIR}/${ACME_FILE_NAME}")
      PRIVKEY=$(jq -r ".${PROVIDER}.Certificates[$i].key" "${WATCH_DIR}/${ACME_FILE_NAME}")

      # Checking if jq returned anything"
      if [ -z "$FULLCHAIN" ] || [ -z "$PRIVKEY" ]; then
        echo "Certificate for domain ${DOMAIN} not found. Please try another domain name..."
        echo "Skipping..."
      else
        # Creating output directory if it doesn't exist...
        mkdir -p "${OUTPUT_DIR}/${DOMAIN}"

        # Saving the fullchain and privkey to files...
        echo "$FULLCHAIN" | base64 -d > "${OUTPUT_DIR}/${DOMAIN}/fullchain.pem"
        echo "$PRIVKEY" | base64 -d > "${OUTPUT_DIR}/${DOMAIN}/privkey.pem"

        # Setting the appropriate file permissions
        chown ${USER_UID}:${USER_GID} ${OUTPUT_DIR}/${DOMAIN}/*
        chmod 600 ${OUTPUT_DIR}/${DOMAIN}/*

        echo "Certificates for ${DOMAIN} have been extracted."
      fi
    done
  else
    echo "File ${WATCH_DIR}/${ACME_FILE_NAME} not found. Skipping..."
  fi

  # Sleep for the defined interval before the next iteration
  sleep ${INTERVAL}
done
