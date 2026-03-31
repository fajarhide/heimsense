# Heimsense 🔱

<p align="center">
  <a href="https://github.com/fajarhide/heimsense/stargazers"><img src="https://img.shields.io/github/stars/fajarhide/heimsense?style=for-the-badge" alt="Stars"/></a>
  <a href="https://github.com/fajarhide/heimsense/releases"><img src="https://img.shields.io/badge/Updated-Mar_31,_2026-brightgreen?style=for-the-badge" alt="Last Update"/></a>
  <a href="./go.mod"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"/></a>
  <a href="#-supported-providers"><img src="https://img.shields.io/badge/Providers-20+-orange?style=for-the-badge" alt="Supported Providers"/></a>
  <a href="./Containerfile"><img src="https://img.shields.io/badge/Container-ready-blueviolet?style=for-the-badge&logo=podman&logoColor=white" alt="Container Ready"/></a>
  <a href="https://github.com/fajarhide/heimsense/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/fajarhide/heimsense/ci.yml?style=for-the-badge&label=CI" alt="CI"/></a>
  <a href="https://github.com/fajarhide/heimsense/releases/latest"><img src="https://img.shields.io/github/release/fajarhide/heimsense?style=for-the-badge" alt="Release Version"/></a>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"/></a>
  <a href="https://github.com/fajarhide/heimsense/issues"><img src="https://img.shields.io/github/issues/fajarhide/heimsense.svg" alt="Issues"/></a>
  <a href="https://github.com/fajarhide/heimsense/pulls"><img src="https://img.shields.io/github/last-commit/fajarhide/heimsense.svg" alt="Last Commit"/></a>
</p>

> *Claude Code is the supercar. Heimsense unlocks it for any LLM.*

A lightweight, production-ready API adapter that unlocks **Claude Code CLI** for **any LLM provider** (OpenAI, DeepSeek, Groq, local models) by translating Anthropic's protocol to OpenAI's, and back. **Zero Python/Node dependencies. Single binary.**

```text
  Claude Code CLI ────► [ Heimsense ] ────► Any LLM Provider
 (Anthropic format)     [ :8080     ]       (OpenAI format)
```

## ✨ Features & Benefits

* **Use Any Model:** DeepSeek for cheap coding, ChatGPT, Groq for speed, or local Ollama.
* **Cost Savings:** Pay a fraction of Anthropic's pricing.
* **Zero Dependencies:** Single Go binary. No Python, no Node.js.
* **Production Ready:** Auto-retries on 5xx errors, graceful shutdown, health checks.
* **Auto Setup:** Interactive CLI wizard configures Claude Code for you automatically.

---

## 🚀 Quick Start

### 1. Install Heimsense
Use the one-line installer (auto-detects OS & installs to `~/.local/bin/`):
```bash
curl -fsSL https://raw.githubusercontent.com/fajarhide/heimsense/main/scripts/install.sh | bash
```
*(Alternatively, download the binary from [Releases](https://github.com/fajarhide/heimsense/releases) or build it yourself using `make build`)*

### 2. Configure & Run
Run the interactive setup. It will ask for your API key and automatically configure Claude Code:
```bash
heimsense setup
```

Then, start the Heimsense server:
```bash
heimsense run
```

### 3. Use in Claude Code
In a new terminal, open Claude Code:
```bash
claude
# Once inside, type /model and select "Heimsense Custom Model"
```

---

## ⚙️ Configuration

Heimsense uses `~/.heimsense/.env` for configuration. You can edit this file to change your provider or model at any time.

| Variable | Example | Description |
|----------|---------|-------------|
| `ANTHROPIC_BASE_URL` | `https://api.openai.com/v1` | Upstream API URL |
| `ANTHROPIC_API_KEY` | `sk-...` | Your API key |
| `ANTHROPIC_CUSTOM_MODEL_OPTION` | `gpt-4o` | Model used if none is specified |
| `LISTEN_ADDR` | `:8080` | Local server port |

*Tip: If you manually change the port in `.env`, run `heimsense sync` to update Claude Code's settings.*

---

## 🐳 Docker / Podman

If you prefer containers, Heimsense is available as a lightweight ~15MB image.

```bash
# 1. Prepare configuration
cp env.example .env
nano .env # Add your API key and Base URL

# 2. Run with Docker
docker run -d \
  --name heimsense \
  -p 8080:8080 \
  -v $(pwd)/.env:/.env \
  ghcr.io/fajarhide/heimsense:latest

# 3. Setup Claude Code locally
heimsense setup
```
*(For Podman, just replace `docker` with `podman`)*

---

## 🧩 Supported Providers

Heimsense works flawlessly with **any** OpenAI-compatible API:

* **Cloud Providers:** OpenAI, DeepSeek, Groq, Together AI, Mistral, xAI (Grok), OpenRouter, Fireworks AI.
* **Local / Self-Hosted:** Ollama, LM Studio, vLLM, LocalAI.

---

## 🧠 How It Works

Heimsense acts as a transparent reverse proxy between Claude Code and your LLM of choice.

1. Claude Code sends a request in **Anthropic format** (`/v1/messages`).
2. Heimsense translates the payload to **OpenAI format** (`/v1/chat/completions`).
3. Your chosen LLM provider responds.
4. Heimsense translates the response (including SSE stream and tool/function calls) back to the Anthropic format expected by Claude Code.

---

## 🆚 Why Heimsense? (vs Alternatives)

While there are Python and Node.js proxies available, Heimsense is built in Go for maximum simplicity:

* **No `pip install`, no `npm install`.** Just a single compiled binary.
* **Extremely lightweight.** Uses <20MB of RAM.
* **Built specifically for Claude Code.** Includes CLI commands (`setup`, `sync`, `run`) to automate configuration directly.
* **Production-ready defaults.** Built-in retry with exponential backoff, graceful shutdown, structured logging, and health checks.

---

## 🛠️ Development & API Reference

If you want to contribute, build from source, or use the API manually:

```bash
make run        # Build + start server
make test       # Run tests
make build      # Compile to ./bin/
make lint       # Code formatting and linting
```

<details>
<summary><strong>Click to view API details</strong></summary>

### `POST /v1/messages` 

You can interact with Heimsense just like the official Anthropic API:

**Streaming:**
```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-key" \
  -d '{
    "model": "gpt-4o",
    "max_tokens": 1024,
    "stream": true,
    "messages": [{"role": "user", "content": "Tell me a story about Heimdall."}]
  }'
```

*(Tool calling is fully supported)*

### `GET /health`
```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

</details>

---
*Heimsense: Inspired by Heimdall, the guardian of the Bifröst bridge. Unlocking cross-realm AI capabilities.*  
**License:** [MIT](./LICENSE)
