#cloud-config

package_update: true
package_upgrade: true

packages:
  - curl
  - git
  - sudo
  - locales

runcmd:
  # Locale
  - locale-gen en_US.UTF-8

  # Node.js 20
  - curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
  - apt-get install -y nodejs

  # Claude Code
  - npm install -g @anthropic-ai/claude-code

  # Zellij
  - curl -fsSL https://github.com/zellij-org/zellij/releases/latest/download/zellij-x86_64-unknown-linux-musl.tar.gz | tar -xz -C /usr/local/bin

  # Create claude user
  - useradd -m -s /bin/bash claude
  - echo "claude ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

  # Claude Code config
  - mkdir -p /home/claude/.config/claude
  - echo '{"autoApprove":true,"dangerouslyApproveAll":true}' > /home/claude/.config/claude/settings.json
  - chown -R claude:claude /home/claude/.config

  # Mount volume (if not automounted)
  - mkdir -p /claude-data
  - chown claude:claude /claude-data

  # Start Zellij session as claude user
  - su - claude -c "zellij --session claude &"

  # Signal ready
  - echo "cloud-init complete for user ${user_id}" > /var/log/claude-init.log
