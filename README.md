# Heimsense

<p align="center">
  <a href="https://github.com/fajarhide/heimsense/stargazers"><img src="https://img.shields.io/github/stars/fajarhide/heimsense?style=for-the-badge" alt="Stars"/></a>
  <a href="https://github.com/fajarhide/heimsense/releases"><img src="https://img.shields.io/badge/Updated-May_03,_2026-brightgreen?style=for-the-badge" alt="Last Update"/></a>
  <a href="./go.mod"><img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"/></a>
  <a href="#supported-providers"><img src="https://img.shields.io/badge/Providers-20+-orange?style=for-the-badge" alt="Supported Providers"/></a>
  <a href="./Containerfile"><img src="https://img.shields.io/badge/Container-ready-blueviolet?style=for-the-badge&logo=podman&logoColor=white" alt="Container Ready"/></a>
  <a href="https://github.com/fajarhide/heimsense/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/fajarhide/heimsense/ci.yml?style=for-the-badge&label=CI" alt="CI"/></a>
</p>

## Table of Contents
- [The Problem](#the-problem)
- [The Solution](#the-solution)
- [Our Philosophy](#our-philosophy)
- [Features](#features)
- [Architecture Overview](#architecture-overview)
- [Quick Start](#quick-start)
  - [1. Installation](#1-installation)
  - [2. Configuration & Execution](#2-configuration--execution)
  - [3. Usage with AI Agents](#3-usage-with-ai-agents)
- [Configuration (`config.toml`)](#configuration-configtoml)
- [Supported Providers](#supported-providers)
- [Container Deployment](#container-deployment)
- [Development & API Reference](#development--api-reference)

---

## The Problem

Modern AI agents and coding assistants (like Claude Code, OpenClaw, Hermes, or Cursor) are incredibly powerful, but they often come with a strict limitation: **Vendor Lock-in**.

1. **Format Incompatibility:** Claude Code only speaks the *Anthropic format* (`/v1/messages`), while many open-source agents only speak the *OpenAI format* (`/v1/chat/completions`).
2. **Cost & Rate Limits:** Using official, premium models exclusively can drain your budget quickly. When you hit a rate limit (429) or a server outage (500) during a complex coding task, your workflow is abruptly halted.
3. **Bloated Workarounds:** Existing proxy tools that try to solve this are often bloated Node.js or Python applications that consume hundreds of megabytes of RAM and require complex package managers just to run locally.

## The Solution

**Heimsense** is a lightweight, production-ready Universal AI Router and API adapter. 

It acts as an intelligent bridge (Bifröst) between your favorite AI coding agents and **any** LLM provider in the world. Heimsense automatically detects the incoming request format (OpenAI or Anthropic) and translates it on the fly, allowing you to use DeepSeek, Groq, local Ollama models, or even Free-tier API providers inside Claude Code or any other agent.

Furthermore, Heimsense features an enterprise-grade **Provider Fallback Chain**. If your primary model goes down or hits a rate limit, Heimsense instantly routes your request to a backup provider without interrupting your workflow.

## Our Philosophy

* **Performance First (Go-Native):** Distributed as a single compiled Go binary. Zero dependencies, no `npm`, no `pip`. It runs with a footprint of less than 20MB RAM.
* **Universal Compatibility:** An AI router should be invisible. Whether the client speaks OpenAI or Anthropic, Heimsense just makes it work.
* **Token Efficiency:** AI should be cheap. Through the integration of the Omni Distillation Engine, Heimsense proactively strips away bloated token usage from large `tool_result` outputs, saving you money automatically.
* **Robustness:** AI APIs are notoriously flaky. Heimsense guarantees reliability through exponential backoff and transparent fallback chains.

---

## Features

* **Universal Translator Engine:** Bi-directional translation between Anthropic's `/v1/messages` and OpenAI's `/v1/chat/completions`.
* **Multi-Provider Fallback:** Configure primary, secondary, and tertiary providers. Automatically failover on 429 (Rate Limit) or 5xx errors.
* **Token Distillation (Omni):** Intercepts and compresses heavy tool outputs before they reach the LLM, reducing token costs drastically.
* **Zero Dependencies:** A single `heimsense` executable is all you need.
* **Automated Setup:** Features an interactive CLI wizard (`heimsense setup`) to configure your tools automatically.

---

## Architecture Overview

```text
  Claude Code CLI    ────►                 ────►  Anthropic API
 (Anthropic format)           [ Heimsense ]       (Anthropic format)
                              [ :8080     ] 
  OpenClaw / Cursor  ────►                 ────►  DeepSeek / Groq / Ollama
 (OpenAI format)                                  (OpenAI format)
```

1. The AI Agent sends a query in its native format.
2. Heimsense's Inbound Engine standardizes the request.
3. The request is routed through the **Provider Chain**, handling retries and fallbacks.
4. The Outbound Engine formats the payload specifically for the target upstream provider.
5. SSE Streams and tool calls are translated backwards in real-time.

---

## Quick Start

### 1. Installation

Download the appropriate binary for your OS to `~/.local/bin/`:

```bash
curl -fsSL https://raw.githubusercontent.com/fajarhide/heimsense/main/scripts/install.sh | bash
```

*Alternatively, download pre-compiled binaries from the [Releases](https://github.com/fajarhide/heimsense/releases) page or build from source using `make build`.*

### 2. Configuration & Execution

Run the interactive setup wizard. This process will create your `~/.heimsense/config.toml` and prompt you to set up your primary and fallback providers.

```bash
heimsense setup
```

Start the Heimsense server:

```bash
heimsense run
```

### 3. Usage with AI Agents

**For Claude Code:**
```bash
claude
# Use the /model command and select "Heimsense Custom Model"
```

**For Open-Source Agents (OpenAI Compatible):**
Simply point your agent's API Base URL to: `http://localhost:8080/v1`

---

## Configuration (`config.toml`)

Heimsense uses a robust TOML configuration file located at `~/.heimsense/config.toml`. 

```toml
[server]
listen_addr = ":8080"
request_timeout_ms = 120000

[omni]
enabled = true
mcp_url = "http://localhost:7070"
min_content_bytes = 1024

[[providers]]
name = "Primary DeepSeek"
base_url = "https://api.deepseek.com/v1"
api_key = "sk-deepseek..."
default_model = "deepseek-coder"
priority = 1
max_retries = 3

[[providers]]
name = "Fallback Groq"
base_url = "https://api.groq.com/openai/v1"
api_key = "gsk-..."
default_model = "llama-3.3-70b-versatile"
priority = 2
max_retries = 2
```

---

## Supported Providers

Heimsense translates formats flawlessly, unlocking support for:

* **Cloud Services:** OpenAI, DeepSeek, Groq, Together AI, Mistral, xAI (Grok), OpenRouter, Fireworks AI, Anthropic (Native).
* **Free Tiers & OAuth:** OpenCode Free, Kiro AI, GitHub Copilot (Coming soon).
* **Local Implementations:** Ollama, LM Studio, vLLM, LocalAI.

---

## Container Deployment

Heimsense is distributed as a compact container image (~15MB) for environments utilizing Docker or Podman.

```bash
# Start the container
docker run -d \
  --name heimsense \
  -p 8080:8080 \
  -v ~/.heimsense/config.toml:/.heimsense/config.toml \
  ghcr.io/fajarhide/heimsense:latest
```

---

## Development & API Reference

For detailed instructions on setting up your local environment, running tests, and understanding the codebase architecture, please refer to our **[Local Development Guide](DEVELOPMENT.md)**.

Standard development commands:

```bash
make run        # Build binary and start server
make test       # Execute test suite
make build      # Compile executable to ./bin/
make ci         # Run formatters, linters, and test suite
```

<details>
<summary><strong>API Endpoints</strong></summary>

### `POST /v1/messages` 
Handles requests formatted according to the Anthropic API specification.

### `POST /v1/chat/completions`
Handles requests formatted according to the standard OpenAI API specification.

### `GET /health`
Health check endpoint.
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
