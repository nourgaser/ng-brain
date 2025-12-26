# 🧠 ng-brain
> A self-hosted, text-first digital garden OS.
> Powered by SilverBullet, Nginx, and Docker.
> 
ng-brain is an opinionated, privacy-focused architecture for hosting a digital second brain. It separates the Engine (this repository) from the Content (your data), allowing you to upgrade, destroy, and replicate the infrastructure without ever touching your actual notes.
It transforms a standard SilverBullet instance into a multi-user platform with public/private access, automatic background versioning, and per-user permission management.

## ✨ Features

 * 🏰 The Gatekeeper: A unified Nginx reverse proxy that handles traffic routing.
   * docs.yourdomain.com → Public Reader (Read-only, high performance).
   * admin.yourdomain.com → Writer (Authenticated, full access).
   * alice.yourdomain.com → User Space (Sandboxed subdomains).
 * 🤖 The Librarian: A custom Go orchestrator that watches a permissions.yaml file. It dynamically spins up and kills Docker containers for users (Alice, Bob) and manages their file system symlinks in real-time.
 * 👻 Ghost Watcher: An invisible background service (Alpine/Git) that monitors your content directory. It runs a silent "Snapshot" every 5 minutes, ensuring you never lose a thought even if you forget to save or commit.
 * ⏳ Time Travel UI: A custom Space Lua plugin that provides a sidebar with commit history, instantaneous diffs, and read-only views of past file versions—all running locally without a GitHub UI.
 * 🔌 Centralized Tooling: Plugins and Libraries (Mermaid, Excalidraw, TreeView) are managed centrally. You update them once in the Core, and they propagate to all users instantly.

## 🏗 Architecture

ng-brain follows a "Split-Brain" philosophy:
 * The Engine (Outer Repo): This repository. It contains the logic, Docker configurations, Nginx templates, and the Librarian binary. It is ephemeral.
 * The Content (Inner Repo): A standard Git repository living at ./content. This is your data. It is .gitignored by the Engine.

```mermaid
graph TD
    User[User] -->|https| Nginx[Gatekeeper (Nginx)]
    Nginx -->|admin.com| Writer[SB Writer (Admin)]
    Nginx -->|docs.com| Reader[SB Reader (Public)]
    Nginx -->|alice.com| Alice[SB Alice (Restricted)]
    
    subgraph "Infrastructure"
        Librarian[The Librarian (Go)] -->|Watches| Perms[permissions.yaml]
        Librarian -->|Spawns| Alice
        Ghost[Ghost Watcher] -->|Commits| Content
    end

    subgraph "Data Persistence"
        Writer -->|Mounts| Content[./content (Git Repo)]
        Reader -->|Mounts| Content
        Alice -->|Symlinks| Content
    end
```

## 🚀 Getting Started

### 0. Prerequisites
 * Docker & Docker Compose
 * A domain name (with wildcard DNS *.yourdomain.com pointing to your server)

### 1. Installation
#### Clone the Engine
git clone https://github.com/nourgaser/ng-brain.git
cd ng-brain

### Create the Content Directory (Your inner brain)
mkdir content
cd content && git init && cd ..

### 2. Configuration
Create a .env file based on the template:
#### Domains
PUBLIC_HOST=docs.nourgaser.com
ADMIN_HOST=admin.nourgaser.com
SPACE_DOMAIN_SUFFIX=docs.nourgaser.com

#### Credentials
SB_WRITER_USER=admin
SB_WRITER_PASSWORD=change_this_immediately

#### Infrastructure
HOST_ROOT_DIR=/home/user/docker/ng-brain

### 3. Permissions
Define your users and their access levels in content/permissions.yaml. The Librarian will read this and auto-configure the system.

```yaml
spaces:
  # Public View (Root Domain)
  public:
    paths:
      - "index.md"
      - "assets/"
      - "Library/Core.md"

  # A restricted user
  alice:
    password: "secret_password"
    paths:
      - "projects/secret-game/"
      - "assets/"
```

### 4. Launch
docker compose up -d

### Access your Writer

At https://admin.yourdomain.com and start writing!

## 🛠 Advanced Usage

### The "Ghost" Committer
The ng-watcher service runs locally. It checks for changes every 5 minutes.
 * Logs: docker logs -f ng-watcher
 * Force Save: Open the Command Palette ###

### (Cmd+K) and run Git: Snapshot Now.

### The History Sidebar

We include a custom Lua script (Library/GitManual.md) that renders a Git UI directly in the editor.
 * Open any file.
 * Run Git: History Sidebar.
 * Click View to see past versions or Diff to see changes.
## 🗺 Roadmap & To-Do

The system is currently V1 (Local Only). Future plans include:
 * [ ] GitHub 2-Way Sync: Replace the local "Ghost Watcher" with a sync agent that pushes/pulls from a private GitHub repository.
 * [ ] Docker-in-Docker (DinD): Allow the Librarian to build custom per-user images on the fly.
 * [ ] CI/CD Pipeline: Auto-deploy the Engine changes via GitHub Actions.
 * [ ] Search: Implement a unified search index across all accessible spaces.
 * [ ] Backup Strategy: Off-site automated backups of the content directory (S3/R2).

##📄 License

MIT License. Built on top of the incredible work by Zef Hemel (SilverBullet).
