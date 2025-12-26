# ng-brain
Self-hosted, text-first digital garden OS powered by SilverBullet, Nginx, and Docker.

**ng-brain** separates the Engine (this repo) from the Content (your data) so you can upgrade or replace infrastructure without touching your notes. It turns a standard SilverBullet instance into a multi-user platform with public/private access, automatic background versioning, and per-user permissions.

## Overview
- Engine: Docker, Nginx templates, and the Go-based Librarian orchestrator (ephemeral).
- Content: Your Git repo at ./content (ignored by the engine, fully yours).
- Remote-first: The content repo clones from a remote (SSH or token) and syncs automatically.
- Outcome: Multi-tenant, permissioned SilverBullet with public docs, private writer, and per-user sandboxes.

## Features
- Text-first workflow: author in your editor of choice (VS Code, Vim, Emacs, etc.) while collaborators get a lightweight web UI for quick access.
- Remote Git sync: clones on startup and keeps ./content in lockstep with your remote (SSH key or token-supported) using file-system events.
- Gatekeeper: Unified Nginx reverse proxy routing public, admin, and per-user subdomains.
- Librarian: Go orchestrator reading permissions.yaml to spin up/tear down user containers and manage symlinks.
- Ghost Watcher: Event-driven git snapshots and syncs triggered on file changes (no fixed polling interval).
- Time Travel UI: Sidebar plugin for commit history, diffs, and read-only past versions.
- Centralized Tooling: Shared plugins (Mermaid, Excalidraw, TreeView) updated once and propagated to all spaces.

## Architecture
ng-brain follows a split-brain model: Engine (infrastructure) vs. Content (data).

## Demo
- Live public SilverBullet reader: https://docs.nourgaser.com

```mermaid
graph TD
    User[User]
    Nginx[Gatekeeper]
    Writer[SB Writer]
    Reader[SB Reader]
    Alice[SB Alice]
    Librarian[The Librarian]
    Perms[permissions.yaml]
    Ghost[Ghost Watcher]
    Content[./content]

    subgraph Infrastructure
        Librarian -->|Watches| Perms
        Librarian -->|Spawns| Alice
        Ghost -->|Commits| Content
    end

    subgraph DataPersistence
        Writer -->|Mounts| Content
        Reader -->|Mounts| Content
        Alice -->|Symlinks| Content
    end

    User -->|https| Nginx
    Nginx -->|admin.yourdomain.com| Writer
    Nginx -->|docs.yourdomain.com| Reader
    Nginx -->|alice.yourdomain.com| Alice
```

## Requirements
- Docker and Docker Compose
- Domain with wildcard DNS (*.yourdomain.com) pointing to the host

## Quick Start
1) Clone the engine
```bash
git clone https://github.com/nourgaser/ng-brain.git
cd ng-brain
```

2) Create a .env file (see Configuration) with CONTENT_REMOTE set to your Git URL (SSH or token) and optional CONTENT_BRANCH (defaults to main).

3) Launch the stack
```bash
docker compose up -d
```

4) Access
- Writer: https://admin.yourdomain.com
- Reader: https://docs.yourdomain.com
- User spaces: https://<user>.yourdomain.com

## Configuration
Sample .env (adjust domains and paths):
```env
# Domains
PUBLIC_HOST=docs.nourgaser.com
ADMIN_HOST=admin.nourgaser.com
SPACE_DOMAIN_SUFFIX=yourdomain.com

# Content Git
CONTENT_REMOTE=git@github.com:you/your-notes.git
CONTENT_BRANCH=main
# Optional (base64-encoded)
# CONTENT_REMOTE_SSH_KEY=...
# CONTENT_REMOTE_SSH_KNOWN_HOSTS=...

# Credentials
SB_WRITER_USER=admin
SB_WRITER_PASSWORD=change_this_immediately

# Host paths
HOST_ROOT_DIR=/home/user/docker/ng-brain
```

## Permissions
Define users and access levels in content/permissions.yaml. The Librarian consumes this file and provisions containers dynamically.

```yaml
spaces:
  public:
    paths:
      - "index.md"
      - "assets/"
      - "Library/Core.md"

  alice:
    password: "secret_password"
    paths:
      - "projects/secret-game/"
      - "assets/"
```

## Services
- Gatekeeper (Nginx): Routes public, admin, and per-user subdomains.
- Librarian (Go): Watches permissions.yaml, manages per-user containers and symlinks.
- Ghost Watcher: Event-driven git sync against CONTENT_REMOTE (clone/pull/rebase/push with conflict branches on failure).
- SilverBullet Writer/Reader: Authenticated writer and public read-only instance.

## Advanced Usage
- Ghost Watcher
  - Logs: `docker logs -f ng-watcher`
  - On-demand snapshot: open the Command Palette and run "Git: Snapshot Now".
- Remote auth
  - SSH: set CONTENT_REMOTE to SSH URL and provide CONTENT_REMOTE_SSH_KEY (base64) plus CONTENT_REMOTE_SSH_KNOWN_HOSTS if needed.
  - Token: use an HTTPS URL that embeds a token or rely on a credential helper baked into the image.
- Conflict handling
  - If a rebase/push conflict occurs, the watcher creates `conflict-YYYYMMDD-HHMMSS` in the remote with current changes and resets the main branch locally.
- History Sidebar
  - Uses the bundled Lua script to show history/diffs inside SilverBullet.

## Roadmap
- [x] Remote Git sync via event-driven watcher (SSH or token).
- [ ] Docker-in-Docker builds for custom per-user images.
- [ ] CI/CD to auto-deploy engine changes.
- [ ] Unified search across spaces.
- [ ] Off-site backups for ./content (S3/R2).

## 📄 License
MIT License. Built on top of SilverBullet by Zef Hemel.
