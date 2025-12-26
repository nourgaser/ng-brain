#!/bin/sh
set -euo pipefail

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*"
}

CONTENT_REMOTE=${CONTENT_REMOTE:?set CONTENT_REMOTE}
CONTENT_BRANCH=${CONTENT_BRANCH:-main}
HOME=${HOME:-/home/git}
REPO_DIR=/repo

install_tools() {
  if command -v inotifywait >/dev/null 2>&1; then return; fi
  apk add --no-cache openssh-client inotify-tools >/dev/null
}

setup_ssh() {
  mkdir -p "$HOME/.ssh"
  chmod 700 "$HOME/.ssh"

  if [ -n "${CONTENT_REMOTE_SSH_KEY:-}" ]; then
    echo "$CONTENT_REMOTE_SSH_KEY" | base64 -d > "$HOME/.ssh/id_ed25519"
    chmod 600 "$HOME/.ssh/id_ed25519"
    export GIT_SSH_COMMAND="ssh -i $HOME/.ssh/id_ed25519 -o StrictHostKeyChecking=no"
  fi

  if [ -n "${CONTENT_REMOTE_SSH_KNOWN_HOSTS:-}" ]; then
    echo "$CONTENT_REMOTE_SSH_KNOWN_HOSTS" | base64 -d > "$HOME/.ssh/known_hosts"
    chmod 644 "$HOME/.ssh/known_hosts"
  fi
}

bootstrap_repo() {
  cd "$REPO_DIR"

  if [ -d .git ]; then
    git remote set-url origin "$CONTENT_REMOTE"
    git fetch origin "$CONTENT_BRANCH" || true
    git checkout "$CONTENT_BRANCH" 2>/dev/null || git checkout -b "$CONTENT_BRANCH"
    git reset --hard "origin/$CONTENT_BRANCH" 2>/dev/null || true
    log "Repo already initialized; synced origin URL"
    return
  fi

  if [ -z "$(ls -A "$REPO_DIR" 2>/dev/null || true)" ]; then
    if git clone --branch "$CONTENT_BRANCH" "$CONTENT_REMOTE" "$REPO_DIR"; then
      cd "$REPO_DIR"
      log "Cloned remote branch $CONTENT_BRANCH"
      return
    fi
    log "Remote clone failed; initializing empty repo"
    git init
    git checkout -b "$CONTENT_BRANCH"
    git remote add origin "$CONTENT_REMOTE"
    git commit --allow-empty -m "Initial empty commit"
    git push -u origin "$CONTENT_BRANCH" || log "Initial push failed; check remote access"
    return
  fi

  log "Existing files found; importing into new repo"
  git init
  git checkout -b "$CONTENT_BRANCH"
  git remote add origin "$CONTENT_REMOTE"
  git add -A
  git commit -m "Initial import from existing content"
  git push -u origin "$CONTENT_BRANCH" || log "Initial push failed; check remote access"
}

push_conflict_branch() {
  reason=$1
  branch="conflict-$(date '+%Y%m%d-%H%M%S')"
  git status --short > CONFLICT_STATUS.txt || true
  git add -A || true
  git commit -m "Conflict snapshot ($reason)" || true
  git push origin "$branch" || log "Failed to push conflict branch $branch"
  log "Conflict detected; pushed $branch with current changes"
  git checkout "$CONTENT_BRANCH"
  git reset --hard "origin/$CONTENT_BRANCH" || true
}

sync_repo() {
  reason=$1
  cd "$REPO_DIR"
  log "Sync start ($reason)"

  git fetch origin "$CONTENT_BRANCH" || { log "Fetch failed"; return; }

  if [ -n "$(git status --porcelain)" ]; then
    git add -A
    git commit -m "Auto-Snapshot: $(date '+%Y-%m-%d %H:%M:%S')"
  fi

  if ! git rebase "origin/$CONTENT_BRANCH"; then
    git rebase --abort || true
    push_conflict_branch "rebase"
    return
  fi

  if git push origin "HEAD:$CONTENT_BRANCH"; then
    log "Sync complete"
    return
  fi

  log "Push failed; retrying after rebase"
  git fetch origin "$CONTENT_BRANCH" || { log "Fetch retry failed"; return; }

  if ! git rebase "origin/$CONTENT_BRANCH"; then
    git rebase --abort || true
    push_conflict_branch "push-rebase"
    return
  fi

  if ! git push origin "HEAD:$CONTENT_BRANCH"; then
    log "Second push failed; creating conflict branch"
    push_conflict_branch "push"
  else
    log "Sync complete after retry"
  fi
}

run_watch_loop() {
  inotifywait -m -r -e close_write,create,delete,move --exclude '/\.git/' "$REPO_DIR" |
  while read -r _; do
    sync_repo "fs-event"
  done
}

install_tools
setup_ssh
mkdir -p "$REPO_DIR"
bootstrap_repo
sync_repo "startup" &
run_watch_loop
wait
