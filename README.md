# pgopher - Automated PostgreSQL Backup System

Complete backup and recovery system for PostgreSQL with support for multiple cloud storage providers (S3, Dropbox, Google Drive, Mega), developed in Go.

## 🚀 Features

- **Automatic backup** of PostgreSQL databases with cron scheduling
- **Multi-cloud storage**: S3, Dropbox, Google Drive, Mega, Google Cloud Storage
- **Local backup retention policies** (days or backup count)
- **Notifications** via Email, Discord, and Telegram
- **Backup encryption** (optional)
- **Restore** from local and remote backups
- **Complete CLI** to manage backups manually
- **Integrated healthcheck** and HTTP server


## ⏰ Scheduling

**pgopher** uses a **simplified daily scheduling** system. You only need to specify the **times of day** when you want backups to run.

### How it works

- **Format**: `HH:MM` (24-hour)
- **Execution**: Automatic daily at defined times
- **Timezone**: Configurable via `timezone` (default: UTC)


### Configuration example

```yaml
timezone: "America/Sao_Paulo"  # Set timezone

local:
  schedule:
    - "02:00"  # Runs at 2 AM
    - "14:00"  # Runs at 2 PM
    - "22:00"  # Runs at 10 PM

providers:
  - name: "s3"
    schedule:
      - "03:00"  # Remote backup at 3 AM
      - "15:00"  # Remote backup at 3 PM
```


### Features

✅ **Simple**: No need to understand cron syntax
✅ **Multiple times**: Run as many backups per day as you want
✅ **Independent**: Each provider can have its own schedule
✅ **Timezone-aware**: Respects configured timezone

## 📦 Installation

### Via Docker (Recommended)

```bash
# Clone the repository
git clone https://github.com/your-username/pgopher.git
cd pgopher

# Create configuration file
cp config.example.yaml config.yaml
nano config.yaml

# Start container
docker-compose up -d
```


### Local Build

```bash
go build -o pgopher ./main.go
./pgopher --help
```


## 🔧 Available Commands

### Daemon (Server Mode)

Starts the automatic backup service with scheduling:

```bash
pgopher daemon --config config.yaml
```


### Manual Backup

Executes an immediate backup:

```bash
pgopher backup --config config.yaml
```


### Restore

Restores a specific backup:

```bash
# List available backups
pgopher restore --list --config config.yaml

# Restore backup by ID
pgopher restore --id shortId --config config.yaml
```


### Help

```bash
pgopher --help
pgopher daemon --help
```


## ⚙️ Configuration (config.yaml)

### Complete Structure

```yaml
# =============================================================================
# PGOPHER - PostgreSQL Backup Configuration
# =============================================================================

server:
  addr: ":8080"

timezone: "" #Ex: America/Sao_Paulo, UTC, default is UTC

database:
  host: "localhost"
  port: 5432
  username: ""
  password: ""
  name: ""

local:
  dir: "./backups"
  schedule:
    - "02:00"
    - "14:00"
  retention:
  # retention_days: 30
  # max_backups: 10
  enabled: true

providers:
  - name: "s3"
    type: "s3"
    enabled: true
    schedule:
      - "02:00"
      - "14:00"
    path: "backups/db" #bucket or bucket/folder
    maxVersions: 5
    timeout: 300 #seconds
    config:
      provider: "s3"
      access_key_id: ""
      secret_access_key: ""
      region: ""
      # endpoint: ""
      # acl: ""
      # force_path_style: ""
      # no_check_bucket: ""

  - name: "drive"
    type: "drive"
    enabled: false
    schedule:
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      token: "" #json format base64
      scope: "drive"

  - name: "dropbox"
    type: "dropbox"
    enabled: false
    schedule:
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      token: "" #json format base64

  - name: "mega"
    type: "mega"
    enabled: false
    schedule:
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      user: ""
      pass: "" #obscure password

  - name: "gcs"
    type: "gcs"
    enabled: false
    schedule:
      - "02:00"
      - "14:00"
    path: "backups" #folder
    maxVersions: 0
    timeout: 600 #seconds
    config:
      service_account_credentials: ""  #json format base64
      project_number: ""
      # bucket_policy_only: ""
      # location: ""
      # storage_class: ""

notification:
  success_enabled: true
  error_enabled: true
  emails:
    - "admin@example.com"
    - "ops@example.com"
  email_from: "backup@example.com"
  smtp_server: ""
  smtp_port: 587
  smtp_user: ""
  smtp_password: ""
  smtp_auth: "plain"
  smtp_tls: false
  discord_webhook_url: "" #https://discord.com/api/webhooks/...
  telegram_bot_token: ""
  telegram_chat_id: ""

encryption_key: ""  #my-super-secret-key

run_on_startup: false
run_remote_on_startup: false
```


## 🐳 Docker Compose

### Basic Example

```yaml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: mydb
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: secret
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  pgopher:
    build: .
    depends_on:
      - postgres
    environment:
      # Override YAML configurations via ENV
      DATABASE_HOST: postgres
      DATABASE_PASSWORD: secret
      BACKUP_DIR: /data/backups
      TZ: America/Sao_Paulo
    volumes:
      # Local backups
      - ./backups:/data/backups
      # Configuration file
      - ./config.yaml:/app/config.yaml:ro
    command: daemon --config /app/config.yaml
    restart: unless-stopped
    ports:
      - "8080:8080"

volumes:
  postgres_data:
```


### With Environment Variables (.env)

```bash
# .env

#GLOBAL
BACKUP_DIR=
TZ=
RUN_ON_STARTUP=
RUN_REMOTE_ON_STARTUP=
BACKUP_ENCRYPTION_KEY=
OBSCURE_KEY=

#DATABASE
DATABASE_HOST=
DATABASE_PORT=
DATABASE_USERNAME=
DATABASE_PASSWORD=
DATABASE_NAME=

#LOCAL
BACKUP_SCHEDULE=
#RETENTION_DAYS=
BACKUP_LIMIT=

#PROVIDER - S3
REMOTE_S3_ENABLED=
REMOTE_S3_PATH=
REMOTE_S3_SCHEDULE=
REMOTE_S3_MAX_VERSIONS=
REMOTE_S3_TIMEOUT=
REMOTE_S3_PROVIDER=
REMOTE_S3_ENDPOINT=
REMOTE_S3_ACCESS_KEY_ID=
REMOTE_S3_SECRET_ACCESS_KEY=
REMOTE_S3_FORCE_PATH_STYLE=
#REMOTE_S3_REGION=
#REMOTE_S3_ACL=

#PROVIDER - GOOGLE DRIVE
REMOTE_GDRIVE_ENABLED=
REMOTE_GDRIVE_PATH=
REMOTE_GDRIVE_SCHEDULE=
REMOTE_GDRIVE_MAX_VERSIONS=
REMOTE_GDRIVE_TIMEOUT=
REMOTE_GDRIVE_PROVIDER=
REMOTE_GDRIVE_TOKEN=
REMOTE_GDRIVE_SCOPE=

#PROVIDER - DROPBOX
REMOTE_DROPBOX_ENABLED=
REMOTE_DROPBOX_PATH=
REMOTE_DROPBOX_SCHEDULE=
REMOTE_DROPBOX_MAX_VERSIONS=
REMOTE_DROPBOX_TIMEOUT=
REMOTE_DROPBOX_PROVIDER=
REMOTE_DROPBOX_CLIENT_ID=
REMOTE_DROPBOX_CLIENT_SECRET=
REMOTE_DROPBOX_TOKEN=

#PROVIDER - MEGA
REMOTE_MEGA_ENABLED=
REMOTE_MEGA_PATH=
REMOTE_MEGA_SCHEDULE=
REMOTE_MEGA_MAX_VERSIONS=
REMOTE_MEGA_TIMEOUT=
REMOTE_MEGA_PROVIDER=
REMOTE_MEGA_USER=
REMOTE_MEGA_PASS=

#PROVIDER - GOOGLE-CLOUD
#REMOTE_GCS_ENABLED=
#REMOTE_GCS_PATH=
#REMOTE_GCS_SCHEDULE=
#REMOTE_GCS_MAX_VERSIONS=
#REMOTE_GCS_PROVIDER=
#REMOTE_GCS_SERVICE_ACCOUNT_CREDENTIALS=
#REMOTE_GCS_PROJECT_NUMBER=
#REMOTE_GCS_BUCKET_POLICY_ONLY=
#REMOTE_GCS_LOCATION=
#REMOTE_GCS_STORAGE_CLASS=

#NOTIFICATION
NOTIFICATION_SUCCESS_ENABLED=
NOTIFICATION_ERROR_ENABLED=

#NOTIFICATION - MAIL
NOTIFICATION_EMAIL=
NOTIFICATION_EMAIL_FROM=
SMTP_SERVER=
SMTP_PORT=
SMTP_USER=
SMTP_PASSWORD=
#plain,login
SMTP_AUTH_METHOD=
SMTP_TLS=false

#NOTIFICATION - DISCORD
DISCORD_WEBHOOK_URL=

#NOTIFICATION - TELEGRAM
#TELEGRAM_BOT_TOKEN=
#TELEGRAM_CHAT_ID=

#SERVER
SERVER_ADDR=
```


## 🔄 Usage Examples

### Immediate backup

```bash
docker-compose exec pgopher pgopher backup --config /app/config.yaml
```


### List available backups

```bash
docker-compose exec pgopher pgopher restore --list --config /app/config.yaml
```


### Restore specific backup

```bash
docker-compose exec pgopher pgopher restore --id abc123 --config /app/config.yaml
```


### View logs in real-time

```bash
docker-compose logs -f pgopher
```


## 🏗️ Docker Image Structure

The Docker image is optimized with multi-stage build:

1. **Builder** (golang:1.25-alpine)
   - Compiles static binary with CGO disabled
   - Compresses with UPX to reduce size
2. **Runtime** (alpine:latest)
   - Installs PostgreSQL client (version configurable via `PG_VERSION`)
   - Creates non-root user `pgopher` (UID 1000)
   - Creates `/data/backups` directory with proper permissions
   - Final image size ~20-30 MB

## 🛡️ Security

- Container runs with non-root user (`pgopher`)
- Backup encryption support via `encryption_key`
- Passwords can be obscured using `OBSCURE_KEY`
- Configuration files mounted as read-only (`:ro`)
- Credentials via environment variables or configuration file
- Logs don't expose passwords


## 🤝 Contributing

```bash
# Clone and create a branch
git checkout -b feat/my-feature

# Commit using Conventional Commits
git commit -m "feat: add FTP support"

# Push and open PR
git push origin feat/my-feature
```

***

**Done!** Now you have a complete README with commands, environment variables, and practical examples.
<span style="display:none">[^1][^2][^3]</span>

<div align="center">⁂</div>

[^1]: work.projects.docker_development

[^2]: preferences.docker_minimalist

[^3]: preferences.postgres_client_version_via_args_env
