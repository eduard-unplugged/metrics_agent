#!/bin/bash

set -e

# Configuration
AGENT_NAME="metrics_agent"
AGENT_REPO="https://github.com/eduard-unplugged/metrics_agent.git" # Specify the correct repository
INSTALL_DIR="/opt/$AGENT_NAME"
SERVICE_FILE="/etc/systemd/system/$AGENT_NAME.service"
CONFIG_FILE="/etc/$AGENT_NAME/config.env"

# Check for root permissions
if [ "$EUID" -ne 0 ]; then
  echo "Please run this script as root."
  exit 1
fi

# Install required packages
echo "[INFO] Installing required packages..."
apt update && apt install -y git curl

# Install Go (if not already installed)
if ! command -v go &> /dev/null; then
  echo "[INFO] Installing Go..."
  curl -fsSL https://dl.google.com/go/go1.20.7.linux-amd64.tar.gz -o go.tar.gz
  tar -C /usr/local -xzf go.tar.gz
  echo "export PATH=\$PATH:/usr/local/go/bin" >> /etc/profile
  source /etc/profile
  rm go.tar.gz
fi

# Clone the repository and build the agent
echo "[INFO] Cloning the agent repository..."
if [ -d "$INSTALL_DIR" ]; then
  rm -rf "$INSTALL_DIR"
fi
git clone "$AGENT_REPO" "$INSTALL_DIR"

echo "[INFO] Building the agent..."
cd "$INSTALL_DIR"
go build -o $AGENT_NAME .

# Create the configuration file
echo "[INFO] Creating the configuration file..."
mkdir -p "/etc/$AGENT_NAME"
cat <<EOF > "$CONFIG_FILE"
SERVER_URL=http://your-server-url
PRUNE_THRESHOLD=80.0
CHECK_INTERVAL=5m
EOF

# Set up the systemd service
echo "[INFO] Setting up the systemd service..."
cat <<EOF > "$SERVICE_FILE"
[Unit]
Description=$AGENT_NAME Service
After=network.target

[Service]
EnvironmentFile=$CONFIG_FILE
ExecStart=$INSTALL_DIR/$AGENT_NAME
Restart=always
User=root
WorkingDirectory=$INSTALL_DIR

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and start the service
echo "[INFO] Reloading systemd and starting the service..."
systemctl daemon-reload
systemctl enable "$AGENT_NAME"
systemctl start "$AGENT_NAME"

echo "[INFO] Installation complete. Service status:"
systemctl status "$AGENT_NAME"