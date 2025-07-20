#!/bin/bash

# --- Configuration ---
ROKU_IP="192.168.254.3"
ROKU_USER="rokudev"
ROKU_PASS="097130"
ZIP_NAME="roku_channel_upload.zip"

# --- Script Directory ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# --- Zip everything in the directory except the script and the output zip ---
echo "Zipping channel source in $SCRIPT_DIR..."
cd "$SCRIPT_DIR"
zip -r "$ZIP_NAME" . -x "$ZIP_NAME" -x "$(basename "$0")"

# --- Upload to Roku ---
echo "Uploading $ZIP_NAME to Roku at $ROKU_IP..."
curl -v -u "$ROKU_USER:$ROKU_PASS" --digest \
  -F "mysubmit=Install" \
  -F "archive=@$ZIP_NAME" \
  "http://$ROKU_IP/plugin_install"

echo
echo "Upload complete. Check your Roku device!"

# --- Optionally, delete the zip after upload ---
rm "$ZIP_NAME"