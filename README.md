# ng-brain v2

A self-hosted SilverBullet stack with a writeable admin brain, a public read-only brain, and per-user spaces provisioned on the fly. Gatekeeper (NGINX) fronts everything, Librarian watches your permissions file and spawns SilverBullet containers for each user, and Git Watcher snapshots content automatically.

## What this ships
- Gatekeeper: NGINX terminator/router with templated hosts and catch-all handling for unknown spaces.
- Writer: SilverBullet instance with login (`SB_USER`) for editing the source of truth under `content/`.
- Reader: SilverBullet instance that exposes the public, read-only brain.
- Librarian: Go daemon that watches `content/permissions.yaml`, manages per-user containers, and renders per-user NGINX configs.
- Git Watcher: commits changes in `content/` every 5 minutes when there is a diff.

## Prerequisites
- Docker + Docker Compose
- An external Docker network (default `nginx-proxy`) if you are fronting this with a reverse proxy like `nginx-proxy`/`traefik`. (See [nginx-proxy.compose.yml](/nginx-proxy.compose.yml)).
- DNS records for your chosen hosts (public, admin, and wildcard for user spaces).

## Configure
1. Copy the env template and fill it:
   ```sh
   cp .env.example .env
   ```
2. Edit `.env` with your domains and credentials:
   - `PUBLIC_HOST` – read-only site (e.g., `brain.example.com`).
   - `ADMIN_HOST` – writer site (e.g., `admin.brain.example.com`).
   - `SPACE_DOMAIN_SUFFIX` – wildcard base for user spaces (e.g., `spaces.brain.example.com`, yielding `<user>.spaces.brain.example.com`).
   - `VIRTUAL_HOST` / `LETSENCRYPT_HOST` – comma-separated list for your proxy (commonly the three hosts above).
   - `SB_WRITER_USER` / `SB_WRITER_PASSWORD` – admin credentials for the writer instance.
   - `HOST_ROOT_DIR` – absolute host path to this repo (so Librarian can bind-mount correctly).
3. Prepare directories (Compose mounts them):
   ```sh
   mkdir -p content spaces nginx/conf.d
   ```
4. Create `content/permissions.yaml` to describe spaces:
   ```yaml
   spaces:
     public:
       password: ""      # ignored for public
       paths: ["/"]
     writer:
       password: ""      # managed by SB_WRITER_USER/PASSWORD
       paths: ["/"]
     alice:
       password: super-secret
       paths: ["Notes", "Projects"]
   ```
   - `paths` are folders under `content/` that get symlinked into each space.
   - Any space other than `public`/`writer` gets its own SilverBullet container and NGINX vhost at `<space>.${SPACE_DOMAIN_SUFFIX}`.

## Run
```sh
docker compose up -d
```
- Gatekeeper renders `nginx/nginx.conf.template` with your env vars before starting NGINX.
- Librarian watches `content/permissions.yaml`; changes trigger container and vhost updates automatically.
- Git Watcher commits `content/` on change; adjust or disable in `compose.yml` if undesired.

## How routing works
- Public: `${PUBLIC_HOST}` → `sb-reader`
- Admin: `${ADMIN_HOST}` → `sb-writer`
- Spaces: `<user>.${SPACE_DOMAIN_SUFFIX}` → `ng-space-<user>` (spawned by Librarian)
- Unknown spaces fall through to the custom 404 page at `nginx/not_found.html`.

## Security notes
- Keep `.env` private; `.gitignore` already excludes it.
- Rotate `SB_WRITER_PASSWORD` after cloning; it is your admin credential.
- Passwords in `permissions.yaml` are mounted into containers as `SB_USER=<user>:<password>`; treat that file as sensitive.

## Useful paths
- Docker Compose: `compose.yml`
- NGINX template: `nginx/nginx.conf.template`
- Librarian daemon: `librarian.go`
- Login page: `nginx/login.html`
- Custom 404: `nginx/not_found.html`
- nginx-proxy Docker Compose: `nginx-proxy.compose.yml`

## Troubleshooting
- Ensure `HOST_ROOT_DIR` matches the absolute path to this repo on the host; mismatches break volume mounts for per-user containers.
- If using an external reverse proxy, confirm the shared Docker network (`proxy-net` by default) exists: `docker network create nginx-proxy`.
- Validate env substitution: `docker compose config` to render the final service definitions.
