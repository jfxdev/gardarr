# Gardarr

<p align="center">
  <img src="assets/banner-lg.png" alt="Gardarr Logo" width="50%" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License: AGPL-3.0" />
  <img src="https://img.shields.io/github/v/release/jfxdev/gardarr" alt="Release" />
  <img src="https://img.shields.io/github/actions/workflow/status/jfxdev/gardarr/.github%2Fworkflows%2Fbuild.yml" alt="GitHub Actions" />
  <img src="https://img.shields.io/coderabbit/prs/github/jfxdev/gardarr?utm_source=oss&utm_medium=github&utm_campaign=gardarr%2Fgardarr&labelColor=171717&color=FF570A&link=https%3A%2F%2Fcoderabbit.ai&label=CodeRabbit+Reviews" alt="CodeRabbit Reviews" />
</p>

Gardarr is a **modern, lightweight multi-instance management platform for qBittorrent**. Connect, monitor, and control multiple qBittorrent servers from one centralized, mobile-first interface.

**Open-source** project licensed under the GNU Affero General Public License v3.0 (AGPLv3), designed to be self-hosted by the community with clean and maintainable code.

## Code quality

Code quality and coverage are monitored with Codacy. The CI workflow publishes Go and frontend coverage when the repository secret `CODACY_PROJECT_TOKEN` is configured. After the repository is connected in Codacy, enable its GitHub status checks and add the project-provided quality and coverage badges here.

## 📸 Screenshots

### 📱 Mobile Experience

<table>
  <tr>
    <td align="center" width="50%">
      <img src="assets/screenshots/dashboard-mobile.png" alt="Dashboard Mobile" width="280" /><br />
      <em>Dashboard with real-time analytics: speeds, ratio, storage, and active tasks</em>
    </td>
    <td align="center" width="50%">
      <img src="assets/screenshots/torrents-page-compact-mobile.png" alt="Torrents Compact Mobile" width="280" /><br />
      <em>Compact view optimized for quick browsing on smaller screens</em>
    </td>
  </tr>
  <tr>
    <td align="center" width="50%">
      <img src="assets/screenshots/torrents-page-mobile.png" alt="Torrents Mobile" width="280" /><br />
      <em>Torrents list with card view featuring cover art and ratio grades</em>
    </td>
    <td align="center" width="50%">
      <img src="assets/screenshots/integrations-page-mobile.png" alt="Integrations Mobile" width="280" /><br />
      <em>Integrations page for webhooks, notifications, and external services</em>
    </td>
  </tr>
</table>

### 🖥️ Desktop Experience

<table>
  <tr>
    <td align="center">
      <img src="assets/screenshots/dashboard-desktop.png" alt="Dashboard Desktop" width="100%" /><br />
      <em>Full dashboard with analytics widgets, recent torrents, categories, and top uploads</em>
    </td>
  </tr>
  <tr>
    <td align="center">
      <img src="assets/screenshots/torrents-page-table-desktop.png" alt="Torrents Table Desktop" width="100%" /><br />
      <em>Table view with sortable columns, color-coded ratio grades (E to S++), and batch actions</em>
    </td>
  </tr>
  <tr>
    <td align="center">
      <img src="assets/screenshots/torrent-details-desktop.png" alt="Torrent Details Desktop" width="100%" /><br />
      <em>Detailed torrent view with progress timeline, ratio grading system, and custom cover art</em>
    </td>
  </tr>
  <tr>
    <td align="center">
      <img src="assets/screenshots/categories-page-desktop.png" alt="Categories Desktop" width="100%" /><br />
      <em>Category management with custom icons, colors, default directories, and auto-tags</em>
    </td>
  </tr>
  <tr>
    <td align="center">
      <img src="assets/screenshots/add-torrent-desktop.png" alt="Add Torrent Desktop" width="100%" /><br />
      <em>Add torrent modal with magnet URI parsing, auto-filled categories and tags</em>
    </td>
  </tr>
</table>

## 📑 Table of Contents

- [📸 Screenshots](#-screenshots)
- [Code quality](#code-quality)
- [✨ Key Features](#-key-features)
- [📋 Requirements](#-requirements)
- [🚀 Getting Started](#-getting-started)
  - [Docker Run](#-docker-run)
  - [Docker Compose](#-docker-compose)
  - [Connect qBittorrent Workers](#-connect-qbittorrent-workers)
- [🏗️ Architecture](#architecture)
- [⏱️ Bandwidth Schedules](#bandwidth-schedules)
- [🏆 Ratio Grading System](#-ratio-grading-system)
- [🔌 Integrations & Events](#-integrations--events)
- [📊 Metrics & Monitoring](#-metrics--monitoring)
- [🎨 Customization](#-customization)
  - [Themes](#themes)
  - [Cover Art & Metadata](#cover-art--metadata)
  - [Categories](#categories)
  - [Tags & Composite Tags](#tags--composite-tags)
- [🛠️ Technology Stack](#-technology-stack)
- [⚙️ Configuration](#-configuration)
- [🛠️ Development](#-development)
- [📄 License](#-license)

## ✨ Key Features

- **🔌 Multi-Worker Management**: Centralized dashboard to manage multiple qBittorrent instances across different servers. Each worker represents one direct qBittorrent connection registered in Gardarr.
- **⚡ Torrent Control**: Full control to add, pause, resume, delete, and prioritize torrents.
- **📊 Advanced Analytics**:
  - Real-time download/upload speed monitoring.
  - Historical data analysis.
  - **Ratio Grading & Sharing**: Grades from E to S++ and shareable torrent status images.
- **⏱️ Bandwidth Schedules**: Automatically apply download and upload limits to each worker at the times you choose, with a clear weekly calendar and automatic restoration of the normal limits afterward.
- **📝 Event System**: Comprehensive event tracking for state changes, completions, and errors, persisted in the database.
- **🔐 Secure Authentication**: 
  - Argon2id password hashing.
  - HTTP-only secure cookies for session management.
  - Role-based access control (RBAC).
- **🏷️ Organization**: Category management plus a colorable tag system with GitLab-style scoped tags (`quality::4k`) and grouped tags (`genre:action`), rename/merge, and a backward-compatibility check for existing data.
- **📱 Mobile-First**: Responsive UI designed for seamless usage on smartphones and tablets.
- **🐳 Docker Native**: Built for containerized environments, with support for Docker secrets through `_FILE` variables.

### 🌟 For Seedboxes

Essential infrastructure for seedbox operators managing single or multiple servers:
- **Unified Control**: Manage multiple seedboxes across different providers from one dashboard
- **Ratio Optimization**: Track and maintain healthy ratios for private trackers with visual grading
- **Cost Efficiency**: Monitor storage, bandwidth, and performance to maximize ROI
- **Bulk Operations**: Handle hundreds of torrents efficiently with batch actions and smart organization
- **Mobile Management**: Control your seedbox fleet on-the-go with responsive mobile interface

## ⏱️ Bandwidth Schedules

Create up to five weekly rules per worker with separate download and upload limits (`0` means unlimited). Rules follow Gardarr's configured timezone; when they overlap, the higher rule wins. Outside scheduled windows, Gardarr restores the worker's saved default limits. Avoid using qBittorrent's own alternative speed-limit scheduler for the same worker.

## 📋 Requirements

- **Docker** (Docker Compose is optional)
- **qBittorrent** v4.1+ (Web UI enabled)

## 🚀 Getting Started

### 🐋 Docker Run

Create persistent named volumes, generate an encryption key, and start the container directly:

```bash
docker volume create gardarr_data
docker volume create gardarr_media

export GARDARR_ENCRYPTION_KEY="$(openssl rand -base64 32)"
echo "Save this key securely before continuing: $GARDARR_ENCRYPTION_KEY"

docker run -d \
  --name gardarr \
  --restart unless-stopped \
  --env APP_URL=http://localhost:3200 \
  --env ENCRYPTION_KEY="$GARDARR_ENCRYPTION_KEY" \
  -p 3200:3200 \
  -v gardarr_data:/data \
  -v gardarr_media:/media \
  ghcr.io/jfxdev/gardarr:latest
```

The named volumes preserve the database and uploaded media when the container is recreated. Keep the generated key in a secure password manager or secret store: it is required to recreate the container and read existing worker credentials.

### 🐳 Docker Compose

Gardarr is a single service. The same deployment can manage one or multiple qBittorrent instances, which are added after signing in.

```bash
# Copy the Compose example
cp examples/default/docker-compose.yml docker-compose.yml

# Generate a key, then save its output as ENCRYPTION_KEY in .env
openssl rand -base64 32
```

Create a `.env` file alongside `docker-compose.yml` with at least:

```dotenv
APP_URL=http://localhost:3200
ENCRYPTION_KEY=paste-the-generated-value-here
```

`ENCRYPTION_KEY` is mandatory and must be a Base64-encoded 32-byte key. Keep it unchanged after the first start; changing it makes previously stored credentials unreadable.

```bash
docker compose up -d
```

Access the dashboard at `http://localhost:3200` and create the initial user.

### 🔌 Connect qBittorrent Workers

After signing in, go to **Settings → Workers** and add each qBittorrent server using its Web UI URL, username, and password. Gardarr encrypts these stored credentials with `ENCRYPTION_KEY`.

If qBittorrent runs on the Docker host, use an address reachable from inside the Gardarr container; `localhost` refers to the container itself.

## 🏗️ Architecture

Gardarr now runs as a single central service that connects directly to one or more qBittorrent Web UI endpoints.

### Single Process Architecture
- **Simplicity**: One deployment, one API process.
- **Direct connectivity**: Gardarr talks to each registered qBittorrent worker directly, without an intermediate worker process.
- **Multi-instance support**: Add multiple qBittorrent servers directly from the UI.

## 🏆 Ratio Grading System

Gardarr converts each torrent's share ratio into a grade from **E** to **S++**, making it easy to see its seeding performance at a glance.

From the torrent details, you can generate downloadable and shareable images of its status, including the ratio and progress. Choose a layout and theme, then save or share the image directly when your browser supports it.

## 🔌 Integrations & Events

The platform features a robust event-driven architecture to keep you connected.

### Event System

Automatically tracks and persists torrent lifecycle events (state changes, additions, removals, completions) with configurable retention via `EVENT_RETENTION_DAYS`.

Transfer Reports capture cumulative torrent upload/download counters on a configurable local-time schedule (four times per day by default), publish daily and weekly rankings, and retain raw snapshots for 15 days. Configure schedules at **Reports**; the latest daily and weekly report are also available as events and can be sent to Discord incoming webhooks from **Integrations**.

**Features:**
- Real-time event tracking and filtering by event type
- Search by torrent name or hash with pagination
- Visual indicators with color-coded badges and timestamps

### Event History Page

A comprehensive interface to monitor all torrent activity with event types: `state_change`, `added`, `removed`, and `completed`.

### Webhooks

Connect webhook-compatible services such as Discord, Slack, n8n, and Home Assistant with multiple endpoints, event filtering, retries, and delivery logging.

## 📊 Metrics & Monitoring

The application exposes Prometheus-compatible metrics at the `/metrics` endpoint for integration with observability stacks.

**Metrics Categories:**
- **Task Metrics**: Download/upload speeds, progress, ratio, seeders/leechers, size, state (labeled by worker_id, task_id, task_name)
- **Worker Metrics**: Status, free disk space, global ratio, qBittorrent version, all-time downloaded/uploaded (labeled by worker_id, worker_name)
- **Server Metrics**: Total workers count by status (ACTIVE, ERRORED, INACTIVE)

**Configuration:**
- Set `METRICS_ENABLED=true`, `METRICS_USERNAME`, and `METRICS_PASSWORD` to enable the endpoint with Basic Auth.
- `METRICS_DISABLE_AUTH=true` exposes the endpoint without credentials; use it only on a protected internal network.

**Example Prometheus scrape configuration:**
```yaml
scrape_configs:
  - job_name: 'gardarr'
    basic_auth:
      username: 'prometheus'
      password: 'your-secure-password'
    static_configs:
      - targets: ['gardarr:3200']
    metrics_path: '/metrics'
```

## 🎨 Customization

Personalize your library with rich metadata and visual options.

### Themes

- **Dark & Light modes** with persistent preferences
- **14 pre-built color palettes** (Default, Aura, Sunset, Ocean, Forest, Cyberpunk, and more)
- **3 custom palette slots** with 4-color system (Primary, Secondary, Accent, Muted)

### Cover Art & Metadata

Upload custom images (JPEG, PNG, GIF, WEBP up to 10MB) with positioning and opacity controls. Add descriptions and notes to torrents.

### Categories

Organize torrents with pre-configured categories featuring custom icons (16 options), colors (10 options), default directories, and default tags. Configure once, apply everywhere.

### Tags & Composite Tags

A dedicated **Tags** page (under **Management**, alongside Categories) persists color per tag - the same enrichment relationship Categories already has with qBittorrent: a tag observed on a worker but never colored locally still shows up and is fully manageable.

Beyond plain tags (`movies`, `music`), the `::` and `:` separators carry real, enforced meaning rather than just a rendering convention:

| Form | Meaning | Multiple values per torrent? |
|---|---|---|
| `key::value` | **Scoped** (GitLab-style) - e.g. `quality::4k` | No - applying `quality::1080p` to a torrent already holding `quality::4k` replaces it |
| `key:value` | **Grouped**, hierarchical for display/filtering - e.g. `genre:action` | Yes |
| `value` | Plain tag | n/a |

Every `key::*` tag shares one color by default (set once on the scope), with an optional override per exact value - so a torrent's quality, resolution, or year reads at a glance instead of blending into a wall of same-colored pills. The torrents page's tag filter groups accordingly, with each scope under its own heading.

Renaming and merging tags share one mechanic (a rename is a merge with a single source): every affected torrent, on every worker, gets the new tag and loses the old one, reported per worker rather than claimed as a blanket success if a worker is unreachable. The Tags page also surfaces case-variant clusters (`4k` vs `4K`) as one-click merge candidates, and flags any pre-existing torrent whose tags are no longer valid under scoped semantics (e.g. two values for the same scope) without silently rewriting them.

**Deriving tags** creates new catalog entries that reuse one side of an existing tag's separator while varying the other, without retagging any torrent. A prefix/suffix switch picks which side stays fixed: in the default prefix mode, deriving `resolution` and `codec` from `quality::4k` keeps the `4k` suffix and produces `resolution::4k` and `codec::4k`; flipping to suffix mode instead keeps the `quality` prefix and derives `quality::1080p`, `quality::720p`, etc. A composed source (`key::value` scoped or `key:value` grouped) reuses its own separator automatically. A plain source has no built-in separator, so you supply the delimiter yourself - deriving `quality` from `movies-1080p` with delimiter `-` produces `quality-1080p`.

## 🛠️ Technology Stack

### Backend
- **Language**: Go 1.25+
- **Framework**: Gin Web Framework
- **Database**: SQLite (default) or PostgreSQL (via GORM)
- **Authentication**: Argon2id + Secure Sessions
- **Config**: Viper (Environment variables & file-based secrets)

### Frontend
- **Framework**: React 19 + Vite
- **Styling**: TailwindCSS 4
- **Charts**: Recharts
- **State/Data**: React Query / Context API
- **Icons**: Lucide React

## ⚙️ Configuration

Gardarr is configured via environment variables. Create a `.env` file beside your Compose file. Most sensitive values can be supplied with Docker secrets using the `_FILE` suffix (for example, `ENCRYPTION_KEY_FILE=/run/secrets/encryption_key`); see the detailed reference for variable-specific behavior.

### Application

| Variable | Description | Default |
|----------|-------------|---------|
| `APP_PORT` | Port for the web interface | `3200` |
| `APP_URL` | Public URL (CORS, cookies). Origin-only, no trailing slash | `http://localhost:3200` |
| `APP_DOMAINS` | Additional allowed CORS origins, separated by commas | - |
| `BANDWIDTH_SCHEDULE_INTERVAL` | Frequency for evaluating bandwidth schedules | `1m` |
| `WS_STATS_INTERVAL` | Frequency for WebSocket worker-stat updates | `2s` |
| `GIN_MODE` | `debug` or `release` (enables HSTS) | `release` |
| `LOG_LEVEL` | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR` | `INFO` |

### Database

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_DRIVER` | Database driver: `sqlite` or `postgres` | `sqlite` |
| `DATABASE_FILE_PATH` | SQLite file path | `/data/gardarr_database.db` |
| `DATABASE_HOST` | PostgreSQL host | `localhost` |
| `DATABASE_PORT` | PostgreSQL port | `5432` |
| `DATABASE_USERNAME` | PostgreSQL username | `gardarr` |
| `DATABASE_PASSWORD` | PostgreSQL password | - |
| `DATABASE_NAME` | PostgreSQL database name | `gardarr_database` |
| `DATABASE_SSL_MODE` | PostgreSQL SSL mode | `disable` |
| `DATABASE_MAX_IDLE_CONNS` | Max idle connections | `10` |
| `DATABASE_MAX_OPEN_CONNS` | Max open connections | `100` |
| `DATABASE_CONN_MAX_LIFETIME` | Connection max lifetime | `1h` |

### Security

| Variable | Description | Default |
|----------|-------------|---------|
| `ENCRYPTION_KEY` | Required Base64-encoded 32-byte key for encrypting stored credentials | - |
| `CUSTOM_CSP` | Custom Content Security Policy override | Built-in CSP |
| `TORRENT_IMAGE_UPLOAD_DIR` | Directory for uploaded torrent images | `/media/uploads/images` |

### Worker Connectivity

| Variable | Description | Default |
|----------|-------------|---------|
| `WORKER_TIMEOUT_SECONDS` | Timeout used when validating and communicating with direct qBittorrent worker connections | `10` |

### Prometheus Metrics

| Variable | Description | Default |
|----------|-------------|---------|
| `METRICS_ENABLED` | Enables the `/metrics` endpoint | `false` |
| `METRICS_DISABLE_AUTH` | Exposes `/metrics` without Basic Auth | `false` |
| `METRICS_USERNAME` | Username for /metrics Basic Auth | (not set) |
| `METRICS_PASSWORD` | Password for /metrics Basic Auth | (not set) |

### Events

| Variable | Description | Default |
|----------|-------------|---------|
| `EVENT_POLL_INTERVAL` | Frequency for detecting torrent state changes | `30s` |
| `EVENT_RETENTION_DAYS` | Days to keep event history | `7` |
| `EVENT_SUBSCRIBER_BUFFER` | In-memory event queue size per subscriber | `256` |
| `EVENT_CLEANUP_INTERVAL` | Frequency for cleaning expired events | `24h` |
| `WEBHOOK_QUEUE_SIZE` | Pending events allowed per webhook | `100` |
| `WEBHOOK_MAX_ATTEMPTS` | Delivery attempts per webhook event | `3` |
| `WEBHOOK_RETRY_BASE_DELAY` | Base delay for webhook retry backoff | `2s` |

### Advanced Integrations

| Variable | Description | Default |
|----------|-------------|---------|
| `TGDB_KEY` | API key to bootstrap TheGamesDB metadata | - |
| `TMDB_KEY` | API key to bootstrap TMDB metadata | - |

For detailed documentation, see [backend/docs/ENVIRONMENT_VARIABLES.md](backend/docs/ENVIRONMENT_VARIABLES.md).

## 🛠️ Development

We welcome contributions! Please see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed setup instructions.

```bash
# Quick dev start
git clone https://github.com/jfxdev/gardarr.git
cd gardarr
make dev
```

## 📄 License

Gardarr is free software licensed under the **GNU Affero General Public License v3.0 (AGPLv3)**. You may use, modify, and distribute it under the license terms.

If you modify Gardarr and make it available for users to interact with over a network, the AGPLv3 requires you to offer those users the corresponding source code for the version you run.

See [LICENSE](LICENSE) for the complete license text.
