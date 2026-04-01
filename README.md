# Heimsense

<p align="center">
  <a href="https://github.com/fajarhide/heimsense/stargazers"><img src="https://img.shields.io/github/stars/fajarhide/heimsense?style=for-the-badge" alt="Stars"/></a>
  <a href="https://github.com/fajarhide/heimsense/releases"><img src="https://img.shields.io/badge/Updated-Mar_31,_2026-brightgreen?style=for-the-badge" alt="Last Update"/></a>
  <a href="./go.mod"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"/></a>
  <a href="#supported-providers"><img src="https://img.shields.io/badge/Providers-20+-orange?style=for-the-badge" alt="Supported Providers"/></a>
  <a href="./Containerfile"><img src="https://img.shields.io/badge/Container-ready-blueviolet?style=for-the-badge&logo=podman&logoColor=white" alt="Container Ready"/></a>
  <a href="https://github.com/fajarhide/heimsense/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/fajarhide/heimsense/ci.yml?style=for-the-badge&label=CI" alt="CI"/></a>
  <a href="https://github.com/fajarhide/heimsense/releases/latest"><img src="https://img.shields.io/github/release/fajarhide/heimsense?style=for-the-badge" alt="Release Version"/></a>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-MIT-blue.svg" alt="License: MIT"/></a>
  <a href="https://github.com/fajarhide/heimsense/issues"><img src="https://img.shields.io/github/issues/fajarhide/heimsense.svg" alt="Issues"/></a>
  <a href="https://github.com/fajarhide/heimsense/pulls"><img src="https://img.shields.io/github/last-commit/fajarhide/heimsense.svg" alt="Last Commit"/></a>
</p>

Heimsense is a lightweight, production-ready API adapter that enables the use of the Claude Code CLI with any LLM provider, such as OpenAI, DeepSeek, Groq, or local models. It functions by translating Anthropic's API protocol to the OpenAI format and vice-versa. 

Delivered as a single compiled Go binary, Heimsense eliminates the need for Python or Node.js runtime environments.

```text
  Claude Code CLI ────► [ Heimsense ] ────► Any LLM Provider
 (Anthropic format)     [ :8080     ]       (OpenAI format)
```

## Features

* **Provider Flexibility:** Compatible with various models including DeepSeek, ChatGPT, Groq, or local options like Ollama.
* **Cost Efficiency:** Allows utilization of more cost-effective models as alternatives to Anthropic's pricing.
* **Zero Dependencies:** Distributed as a single Go binary. No external runtime environments required.
* **Production Ready:** Includes automatic retries on 5xx errors, graceful shutdown, and health check endpoints.
* **Automated Setup:** Features an interactive CLI to automatically configure Claude Code.

---

## Quick Start

### 1. Installation

Execute the installation script to download the appropriate binary for your operating system to `~/.local/bin/`:

```bash
curl -fsSL https://raw.githubusercontent.com/fajarhide/heimsense/main/scripts/install.sh | bash
```

Alternatively, pre-compiled binaries are available on the [Releases](https://github.com/fajarhide/heimsense/releases) page, or it can be built from source using `make build`.

### 2. Configuration & Execution

Run the interactive setup. This process prompts for your target API key and configures the Claude Code CLI:

```bash
heimsense setup
```

Start the Heimsense server:

```bash
heimsense run
```

### 3. Usage with Claude Code

Open a new terminal session and launch Claude Code:

```bash
claude
# Use the /model command and select "Heimsense Custom Model"
```

---

## Configuration

Configuration is managed via the `~/.heimsense/.env` file. Modify these variables to adjust your provider or model settings.

| Variable | Example | Description |
|----------|---------|-------------|
| `ANTHROPIC_BASE_URL` | `https://api.openai.com/v1` | Target LLM provider API URL |
| `ANTHROPIC_API_KEY` | `sk-...` | Authentication token for the upstream API |
| `ANTHROPIC_CUSTOM_MODEL_OPTION` | `gpt-4o` | Default model when none is specified or mapped |
| `MODEL_MAP_HAIKU` | `gemini-2.5-flash` | (Optional) Redirection for Claude Haiku requests |
| `MODEL_MAP_SONNET` | `gemini-2.5-pro` | (Optional) Redirection for Claude Sonnet requests |
| `MODEL_MAP_OPUS` | `gemini-2.5-pro` | (Optional) Redirection for Claude Opus requests |
| `LISTEN_ADDR` | `:8080` | Local server listening address and port |

*Note: After making manual changes to the `.env` file, execute `heimsense sync` to propagate the updates to Claude Code's configuration.*

---

## Container Deployment

Heimsense is distributed as a compact container image (~15MB) for environments utilizing Docker or Podman.

```bash
# 1. Prepare configuration
cp env.example .env
# Edit .env to set your target API key and Base URL

# 2. Start the container
docker run -d \
  --name heimsense \
  -p 8080:8080 \
  -v $(pwd)/.env:/.env \
  ghcr.io/fajarhide/heimsense:latest

# 3. Configure local Claude Code instance
heimsense setup
```

---

## Supported Providers

Heimsense is compatible with endpoints adhering to the OpenAI API specification, including:

* **Cloud Services:** OpenAI, DeepSeek, Groq, Together AI, Mistral, xAI (Grok), OpenRouter, Fireworks AI.
* **Local Implementations:** Ollama, LM Studio, vLLM, LocalAI.

---

## Architecture Overview

Heimsense operates as a translation proxy layer between the Claude Code client and the target LLM API.

1. The Claude Code client sends queries in the **Anthropic format** (`/v1/messages`).
2. Heimsense transforms the request payload into the **OpenAI format** (`/v1/chat/completions`).
3. The query is forwarded to the designated LLM provider.
4. The provider's response, including SSE streams and tool/function calls, is translated back to the Anthropic format for consumption by the client.

---

## Comparison: Why Heimsense

In contrast to similar tools built with Python or Node.js, Heimsense prioritizes simplicity and minimal footprint through its Go implementation:

* **No package managers:** Bypasses `pip`, `npm`, or virtual environments in favor of a standalone binary.
* **Minimal resource usage:** Typical RAM consumption is under 20MB.
* **Integrated CLI:** Dedicated commands (`setup`, `sync`, `run`) streamline the configuration process for Claude Code.
* **Reliability features:** Incorporates exponential backoff retries, graceful shutdown, structured logging, and health monitoring.

---

## Development & API Reference

Standard development commands:

```bash
make run        # Build binary and start server
make test       # Execute test suite
make build      # Compile executable to ./bin/
make lint       # Run code formatters and linters
```

<details>
<summary><strong>API Endpoints</strong></summary>

### `POST /v1/messages` 

This endpoint handles requests formatted according to the Anthropic API specification:

**Streaming Example:**
```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-key" \
  -d '{
    "model": "gpt-4o",
    "max_tokens": 1024,
    "stream": true,
    "messages": [{"role": "user", "content": "Explain the concept of an API."}]
  }'
```

*(Tool and function calling features are supported)*

### `GET /health`
```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

</details>


## Star History

<p align="center">
  <a href="https://star-history.com/#fajarhide/heimsense&Date">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=fajarhide/heimsense&type=Date&theme=dark" />
      <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=fajarhide/heimsense&type=Date" />
      <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=fajarhide/heimsense&type=Date" width="600" />
    </picture>
  </a>
</p>

---
*Heimsense: Inspired by Heimdall, the guardian of the Bifröst bridge.*  
**License:** [MIT](./LICENSE)
