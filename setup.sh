#!/usr/bin/env bash
#
# setup.sh — one-command setup for the Agentic AI (Golang) project.
#
# Target: macOS (darwin) with Homebrew. Idempotent — safe to re-run.
# It installs/verifies Go, MongoDB, and Ollama; pulls the embedding + chat
# models; runs `go mod download` and builds both modules; and creates the
# local config.json files from the examples if they are missing.
#
# API keys (Tavily / Pinecone / Anthropic) CANNOT be auto-provisioned. If the
# matching environment variables are set when you run this script, they are
# written into agents/config.json automatically; otherwise placeholders are
# left in place and a clear notice prints exactly which keys to fill and where.
#
# config.json is gitignored in both modules (it holds secrets) — only the
# config.example.json files are committed.
#
# Usage:
#   ./setup.sh
#   TAVILY_API_KEY=... PINECONE_API_KEY=... PINECONE_HOST=... ./setup.sh
#
set -uo pipefail

# ─── Locate repo root (this script lives at the repo root) ──────────────────────
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTS_DIR="$REPO_ROOT/agents"
MCP_DIR="$REPO_ROOT/mcp"

# ─── Pretty output ──────────────────────────────────────────────────────────────
BOLD="$(tput bold 2>/dev/null || true)"
DIM="$(tput dim 2>/dev/null || true)"
RED="$(tput setaf 1 2>/dev/null || true)"
GREEN="$(tput setaf 2 2>/dev/null || true)"
YELLOW="$(tput setaf 3 2>/dev/null || true)"
CYAN="$(tput setaf 6 2>/dev/null || true)"
RESET="$(tput sgr0 2>/dev/null || true)"

step()  { printf "\n%s==>%s %s%s%s\n" "$CYAN" "$RESET" "$BOLD" "$*" "$RESET"; }
ok()    { printf "  %s✓%s %s\n" "$GREEN" "$RESET" "$*"; }
warn()  { printf "  %s!%s %s\n" "$YELLOW" "$RESET" "$*"; }
err()   { printf "  %s✗%s %s\n" "$RED" "$RESET" "$*"; }
info()  { printf "    %s%s%s\n" "$DIM" "$*" "$RESET"; }

have()  { command -v "$1" >/dev/null 2>&1; }

# Track keys still needing manual entry so we can print one clear notice at the end.
MISSING_KEYS=()

# Chat model to pull for local Ollama (small + capable). Override with OLLAMA_CHAT_MODEL.
OLLAMA_CHAT_MODEL="${OLLAMA_CHAT_MODEL:-gemma2:2b}"
EMBED_MODEL="nomic-embed-text"

printf "%s\n" "${BOLD}Agentic AI (Golang) — setup${RESET}"
printf "%s\n" "${DIM}Local-first ReAct assistant · Go · MongoDB · Ollama · Pinecone${RESET}"

# ─── 0. Platform check ──────────────────────────────────────────────────────────
step "Checking platform"
if [[ "$(uname -s)" != "Darwin" ]]; then
  warn "This script targets macOS (darwin) with Homebrew."
  warn "On Linux/Windows, install Go, MongoDB, and Ollama manually, then re-run the build steps below."
else
  ok "macOS detected"
fi

# ─── 1. Homebrew ────────────────────────────────────────────────────────────────
step "Homebrew"
if have brew; then
  ok "brew present ($(brew --version 2>/dev/null | head -1))"
else
  warn "Homebrew not found."
  info "Install it from https://brew.sh then re-run ./setup.sh, or install Go/MongoDB/Ollama manually."
fi

brew_install() {
  # brew_install <formula> [--cask] — install if missing, no-op if present.
  local formula="$1"; shift || true
  if brew list "$formula" >/dev/null 2>&1; then
    ok "$formula already installed"
  else
    info "brew install $formula $*"
    if brew install "$@" "$formula" >/dev/null 2>&1; then
      ok "$formula installed"
    else
      err "failed to install $formula via brew — install it manually"
    fi
  fi
}

# ─── 2. Go ──────────────────────────────────────────────────────────────────────
step "Go toolchain"
if have go; then
  ok "go present ($(go version 2>/dev/null))"
elif have brew; then
  brew_install go
else
  err "Go not found and Homebrew unavailable — install Go 1.21+ from https://go.dev/dl/"
fi

# ─── 3. MongoDB ─────────────────────────────────────────────────────────────────
step "MongoDB"
if have mongod || have mongosh; then
  ok "MongoDB present"
elif have brew; then
  # The community server lives in a tap.
  if ! brew tap | grep -q "mongodb/brew"; then
    info "brew tap mongodb/brew"
    brew tap mongodb/brew >/dev/null 2>&1 || warn "could not tap mongodb/brew"
  fi
  brew_install mongodb-community
else
  err "MongoDB not found and Homebrew unavailable — install MongoDB Community Edition manually"
fi

# Start MongoDB as a brew service (idempotent).
if have brew; then
  if brew services list 2>/dev/null | grep -q "mongodb-community.*started"; then
    ok "mongodb-community service already running"
  else
    info "brew services start mongodb-community"
    if brew services start mongodb-community >/dev/null 2>&1; then
      ok "mongodb-community service started"
    else
      warn "could not start mongodb-community via brew services — start MongoDB manually"
    fi
  fi
fi

# ─── 4. Ollama ──────────────────────────────────────────────────────────────────
step "Ollama (local LLM + embeddings)"
if have ollama; then
  ok "ollama present"
elif have brew; then
  brew_install ollama
else
  err "Ollama not found and Homebrew unavailable — install from https://ollama.com/download"
fi

# Make sure the Ollama server is running (needed to pull models / embed).
if have ollama; then
  if curl -s -o /dev/null -w "%{http_code}" http://localhost:11434/api/tags 2>/dev/null | grep -q "200"; then
    ok "ollama server reachable on :11434"
  else
    # Prefer the brew service; fall back to a detached `ollama serve`.
    if have brew && brew services list 2>/dev/null | grep -q "^ollama"; then
      info "brew services start ollama"
      brew services start ollama >/dev/null 2>&1 || true
    fi
    if ! curl -s -o /dev/null -w "%{http_code}" http://localhost:11434/api/tags 2>/dev/null | grep -q "200"; then
      info "starting 'ollama serve' in the background"
      nohup ollama serve >/dev/null 2>&1 &
    fi
    # Wait briefly for the server to come up.
    for _ in 1 2 3 4 5 6 7 8 9 10; do
      if curl -s -o /dev/null -w "%{http_code}" http://localhost:11434/api/tags 2>/dev/null | grep -q "200"; then
        ok "ollama server is up"
        break
      fi
      sleep 1
    done
    if ! curl -s -o /dev/null -w "%{http_code}" http://localhost:11434/api/tags 2>/dev/null | grep -q "200"; then
      warn "ollama server did not respond — run 'ollama serve' in another terminal, then re-run ./setup.sh"
    fi
  fi

  # Pull the embedding + chat models (idempotent — Ollama skips if present).
  pull_model() {
    local m="$1"
    if ollama list 2>/dev/null | awk '{print $1}' | grep -q "^${m}$"; then
      ok "model $m already pulled"
    else
      info "ollama pull $m  (this can take a while)"
      if ollama pull "$m" >/dev/null 2>&1; then
        ok "pulled $m"
      else
        warn "could not pull $m — pull it later with: ollama pull $m"
      fi
    fi
  }
  pull_model "$EMBED_MODEL"
  pull_model "$OLLAMA_CHAT_MODEL"
fi

# ─── 5. Config files ────────────────────────────────────────────────────────────
step "Config files (config.json from examples)"

ensure_config() {
  # ensure_config <dir> — copy config.example.json -> config.json if missing.
  local dir="$1"
  local example="$dir/config.example.json"
  local target="$dir/config.json"
  local label="${dir##*/}"
  if [[ ! -f "$example" ]]; then
    warn "$label: config.example.json not found — skipping"
    return
  fi
  if [[ -f "$target" ]]; then
    ok "$label/config.json already exists (left untouched)"
  else
    cp "$example" "$target"
    ok "$label/config.json created from example"
  fi
}

ensure_config "$AGENTS_DIR"
ensure_config "$MCP_DIR"

# ─── 5a. Inject API keys from the environment, where provided ───────────────────
# We only touch agents/config.json (mcp/config.json holds no secrets).
# Uses a tiny portable Python helper so we never hand-edit JSON with sed.
AGENTS_CONFIG="$AGENTS_DIR/config.json"

set_config_key() {
  # set_config_key <JSON_KEY> <ENV_VALUE> <PLACEHOLDER_SUBSTRING> <human-name>
  local key="$1" value="$2" placeholder="$3" human="$4"
  if [[ ! -f "$AGENTS_CONFIG" ]]; then return; fi
  if [[ -n "$value" ]]; then
    if have python3; then
      python3 - "$AGENTS_CONFIG" "$key" "$value" <<'PY'
import json, sys
path, key, value = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path) as f:
    cfg = json.load(f)
cfg[key] = value
with open(path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PY
      ok "$human written into agents/config.json from \$$key-style env var"
    else
      warn "python3 unavailable — set $key manually in agents/config.json"
      MISSING_KEYS+=("$human  →  agents/config.json key \"$key\"")
    fi
  else
    # No env value: check whether the example placeholder is still in place.
    if grep -q "$placeholder" "$AGENTS_CONFIG" 2>/dev/null; then
      MISSING_KEYS+=("$human  →  agents/config.json key \"$key\"")
    fi
  fi
}

set_config_key "TAVILY_API_KEY"    "${TAVILY_API_KEY:-}"    "tvly-your-key-here"  "Tavily API key (web search)"
set_config_key "PINECONE_API_KEY"  "${PINECONE_API_KEY:-}"  "pcsk_your-key-here"  "Pinecone API key (PDF vector search)"
set_config_key "PINECONE_HOST"     "${PINECONE_HOST:-}"     "your-index.svc"      "Pinecone host URL (PDF vector search)"

# Anthropic is optional — only inject if the user supplied a key.
if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
  set_config_key "ANTHROPIC_API_KEY" "${ANTHROPIC_API_KEY:-}" "sk-ant-" "Anthropic API key (optional Claude backend)"
  if [[ -n "${ANTHROPIC_MODEL:-}" ]]; then
    set_config_key "ANTHROPIC_MODEL" "${ANTHROPIC_MODEL:-}" "__never__" "Anthropic model id"
  fi
  warn "To USE Claude, also set \"ANTHROPIC_CREDIT_BALANCE\": true in agents/config.json"
fi

# ─── 6. Go modules: download + build ────────────────────────────────────────────
build_module() {
  # build_module <dir> <output-binary-name>
  local dir="$1" out="$2" label="${1##*/}"
  if [[ ! -f "$dir/go.mod" ]]; then
    warn "$label: no go.mod — skipping"
    return
  fi
  if ! have go; then
    warn "$label: Go unavailable — skipping build"
    return
  fi
  info "($label) go mod download"
  ( cd "$dir" && go mod download ) && ok "$label: dependencies downloaded" || err "$label: go mod download failed"
  info "($label) go build"
  ( cd "$dir" && go build -o "$out" . ) && ok "$label: built ./$out" || err "$label: build failed"
}

step "Building modules"
build_module "$MCP_DIR" "mcp"
build_module "$AGENTS_DIR" "agentic-ai"

# ─── 7. Final notice ────────────────────────────────────────────────────────────
step "Setup complete"

if [[ ${#MISSING_KEYS[@]} -gt 0 ]]; then
  printf "\n%s%sACTION NEEDED — fill these API keys (they can't be auto-provisioned):%s\n" "$BOLD" "$YELLOW" "$RESET"
  for k in "${MISSING_KEYS[@]}"; do
    printf "  %s•%s %s\n" "$YELLOW" "$RESET" "$k"
  done
  info "Tip: re-run with the keys in your env to inject them automatically, e.g."
  info "  TAVILY_API_KEY=... PINECONE_API_KEY=... PINECONE_HOST=... ./setup.sh"
  printf "  %sconfig.json is gitignored — your keys are never committed.%s\n" "$DIM" "$RESET"
else
  ok "All required API keys are present in agents/config.json"
fi

printf "\n%sNext steps — run the two services in separate terminals:%s\n" "$BOLD" "$RESET"
printf "  %s1.%s  cd mcp    && go run .        %s# MongoDB MCP server on :8083%s\n" "$GREEN" "$RESET" "$DIM" "$RESET"
printf "  %s2.%s  cd agents && go run .        %s# Agent + chat UI on :8082%s\n" "$GREEN" "$RESET" "$DIM" "$RESET"
printf "\n%sThen open:%s\n" "$BOLD" "$RESET"
printf "  • Chat:   http://localhost:8082\n"
printf "  • Ingest: http://localhost:8082/ingest.html\n"
printf "\n%sNotes:%s\n" "$BOLD" "$RESET"
info "Both toggles off = answers from the model's own knowledge (no keys needed)."
info "📚 My Library needs Pinecone keys + a host; 🌐 Web needs a Tavily key."
info "Conversation memory needs MongoDB running (started above)."
printf "\n"
