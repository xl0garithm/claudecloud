#cloud-config

package_update: true
package_upgrade: true

packages:
  - curl
  - git
  - sudo
  - locales
  - mosh
  - ufw

write_files:
  - path: /opt/cloudcode/setup.sh
    permissions: "0755"
    content: |
      #!/bin/bash
      set -euo pipefail
      export DEBIAN_FRONTEND=noninteractive

      echo "=== CloudCode instance setup ==="

      # Locale
      locale-gen en_US.UTF-8 2>/dev/null || true
      export LANG=en_US.UTF-8

      # Node.js 20
      if ! command -v node &>/dev/null; then
          curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
          apt-get install -y nodejs
      fi

      # Claude Code
      if ! command -v claude &>/dev/null; then
          npm install -g @anthropic-ai/claude-code
      fi

      # Zellij
      if ! command -v zellij &>/dev/null; then
          curl -fsSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz \
              | tar -xz -C /usr/local/bin
      fi

      # Create claude user
      if ! id claude &>/dev/null; then
          useradd -m -s /bin/bash claude
          echo "claude ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
      fi

      # Claude Code config
      mkdir -p /home/claude/.config/claude
      echo '{"autoApprove":true,"dangerouslyApproveAll":true}' > /home/claude/.config/claude/settings.json
      chown -R claude:claude /home/claude/.config

      # Mount volume
      mkdir -p /claude-data
      chown claude:claude /claude-data

      echo "=== CloudCode instance setup complete ==="

  - path: /opt/cloudcode/start-session.sh
    permissions: "0755"
    content: |
      #!/bin/bash
      set -euo pipefail
      SESSION_NAME="${1:-claude}"
      LAYOUT="claude"
      if zellij list-sessions 2>/dev/null | grep -q "^$${SESSION_NAME}"; then
          exec zellij attach "$SESSION_NAME"
      fi
      exec zellij --session "$SESSION_NAME" --layout "$LAYOUT"

  - path: /home/claude/.config/zellij/layouts/claude.kdl
    permissions: "0644"
    content: |
      layout {
          pane split_direction="vertical" {
              pane size="70%" {
                  name "claude"
              }
              pane size="30%" {
                  name "shell"
              }
          }
      }

runcmd:
  # Run the setup script
  - /opt/cloudcode/setup.sh

  # Copy session helper
  - cp /opt/cloudcode/start-session.sh /home/claude/start-session.sh
  - chown claude:claude /home/claude/start-session.sh
  - chown -R claude:claude /home/claude/.config/zellij

  # Netbird setup (if setup key is provided)
%{ if netbird_setup_key != "" ~}
  - curl -fsSL https://pkgs.netbird.io/install.sh | bash
  - netbird up --setup-key ${netbird_setup_key}

  # UFW: allow only mosh UDP and Netbird WireGuard
  - ufw default deny incoming
  - ufw default allow outgoing
  - ufw allow 60000:60010/udp comment "mosh"
  - ufw allow 51820/udp comment "netbird-wireguard"
  - ufw --force enable
%{ endif ~}

  # Start Zellij session as claude user
  - su - claude -c "zellij --session claude --layout claude &"

  # Signal ready
  - echo "cloud-init complete for user ${user_id}" > /var/log/claude-init.log
