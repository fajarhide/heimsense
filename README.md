

# Heimsense

<p align="center">
  <a href="https://github.com/fajarhide/heimsense/stargazers"><img src="https://img.shields.io/github/stars/fajarhide/heimsense?style=for-the-badge" alt="Stars"/></a>
  <a href="https://github.com/fajarhide/heimsense/releases"><img src="https://img.shields.io/badge/Updated-Mar_25,_2026-brightgreen?style=for-the-badge" alt="Last Update"/></a>
  <a href="./go.mod"><img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version"/></a>
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

> *"Like Heimdall's supernatural senses that perceive everything across the nine realms, Heimsense bridges your Claude CLI to any LLM provider."*

A lightweight, production-ready API adapter that enables **Claude Code CLI** (and any Anthropic-compatible client) to work with OpenAI-compatible backends.

```
Claude Code CLI ──► Heimsense (:8080) ──► Any LLM Provider
  Anthropic format      translates          OpenAI format
```

---

## Why Heimsense?

### The Mythology

In Norse mythology, **Heimdall** is the guardian of Bifröst — the rainbow bridge connecting the nine realms. He possesses extraordinary senses: he can see and hear everything happening across all worlds, day and night, without sleeping. His keen perception makes him the perfect sentinel, watching over the cosmos.

### The Philosophy

Just as Heimdall stands at the gateway between realms, **Heimsense** sits between your Anthropic-compatible applications and the vast landscape of LLM providers:

| Heimdall | Heimsense |
|----------|-----------|
| Guards Bifröst (the bridge) | Guards your API gateway |
| Sees across all nine realms | Connects to multiple LLM providers |
| Never sleeps | Always-on, production-ready |
| Heightened senses | Intelligent request routing |
| Warns of threats | Handles errors gracefully with retries |

### The Name

**Heim** (from Heimdall) + **Sense** (perception/awareness) = **Heimsense**

The ability to sense, route, and adapt API requests across the LLM multiverse.

---

## What Problems Does It Solve?

| Problem | Solution |
|---------|----------|
| **Vendor Lock-in** | Switch providers without code changes |
| **High API Costs** | Use cheaper alternatives to Anthropic |
| **Model Limitations** | Access GPT, Gemini, DeepSeek, Llama, etc. via Claude Code |
| **API Downtime** | Automatic retry with exponential backoff |
| **Format Incompatibility** | Seamless Anthropic ↔ OpenAI translation |
| **Complex Setup** | Single binary, zero config complexity |

---

## Benefits

### Cost Savings
- Use budget-friendly providers like DeepSeek, Groq, or Together AI
- Pay fraction of Anthropic's pricing for similar capabilities

### Flexibility
- Switch between providers by changing a single environment variable
- Test and compare different models with the same interface

### Reliability
- Automatic retry on transient failures (5xx errors)
- Exponential backoff prevents overwhelming upstream
- Graceful shutdown preserves in-flight requests

### Simplicity
- Single binary deployment
- Environment-based configuration
- Works with existing Claude Code setup (one command)

### Transparency
- Structured JSON logging for observability
- Health check endpoint for monitoring
- Request/response metrics in logs

---

## How Heimsense Works

### Architecture

```mermaid
graph LR
    subgraph Client
        CC["🖥️ Claude Code<br/><small>Anthropic format</small>"]
    end

    subgraph Heimsense
        direction LR
        H["Handler<br/><small>parse & validate</small>"] --> A["Adapter<br/><small>transform</small>"] --> C["Client<br/><small>HTTP + retry</small>"]
    end

    subgraph Upstream
        LP["🤖 LLM Provider<br/><small>OpenAI format</small>"]
    end

    CC -- "POST /v1/messages<br/><small>Anthropic request</small>" --> H
    C -- "/v1/chat/completions<br/><small>OpenAI request</small>" --> LP
    LP -- "OpenAI response" --> C
    C -- "Anthropic response" --> CC
```

### Request Flow

1. **Receive** — Claude Code sends Anthropic-format request to `/v1/messages`
2. **Parse** — Handler validates and extracts request components
3. **Transform** — Adapter converts Anthropic schema → OpenAI schema
4. **Forward** — Client sends to upstream with retry logic
5. **Adapt** — Response transformed back to Anthropic format
6. **Return** — Claude Code receives expected response

### Translation Layer

| Anthropic | Direction | OpenAI |
|-----------|-----------|--------|
| `/v1/messages` | → | `/v1/chat/completions` |
| `content[]` array | → | `message.content` string |
| `system` field | → | `messages[0]` with role:system |
| `max_tokens` | ↔ | `max_tokens` |
| `tools` | ↔ | `tools` / `functions` |
| `input_tokens` | ← | `prompt_tokens` |
| `output_tokens` | ← | `completion_tokens` |

---

## Features

- Full `/v1/messages` → `/v1/chat/completions` translation
- Streaming (SSE) with Anthropic event protocol
- String and array content formats
- System prompt handling
- Function calling (tools) support
- Authorization header passthrough
- Retry with exponential backoff (5xx)
- Configurable timeout
- Structured JSON logging (`slog`)
- Graceful shutdown (SIGINT/SIGTERM)
- Health check endpoint (`/health`)
- Auto-setup script for Claude Code
- Container support (Podman/Docker)

---

## Requirements

- Go 1.22+ (for building from source)
- Podman or Docker (for containerized deployment)
- API key from any OpenAI-compatible provider

---

## Quick Start

### Option 1: One-Line Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/fajarhide/heimsense/main/scripts/install.sh | bash
```

This will:
1. Auto-detect your OS & architecture
2. Download the latest binary from GitHub Releases
3. Install to `~/.local/bin/`
4. Configure Claude Code settings (prompts for API key)

Then start Heimsense and run Claude:

```bash
heimsense

# In another terminal:
claude
# Inside Claude → /model → select Heimsense Custom Model
```

Config is saved to `~/.heimsense/.env` — edit it anytime to change provider, key, or model.

### Option 2: Native Go

```bash
# 1. Setup environment
cp env.example .env
# Edit .env → set your ANTHROPIC_API_KEY

# 2. Start Heimsense
make run

# 3. Configure Claude Code
make setup

# 4. Run Claude Code
claude

# 5. Select Model (inside Claude)
/model
# Select your custom model (e.g., Heimsense Model)
```

### Option 2: Podman

```bash
# 1. Setup environment
cp env.example .env
# Edit .env → set your ANTHROPIC_API_KEY

# 2. Build and run
make podman-build
make podman-run

# 3. Configure Claude Code
make setup

# 4. Run Claude Code
claude

# 5. Select Model (inside Claude)
/model
# Select your custom model (e.g., Heimsense Model)
```

### Option 3: Docker

```bash
# 1. Setup environment
cp env.example .env

# 2. Build and run
make docker-build
make docker-run

# 3. Configure Claude Code
make setup

# 4. Run Claude Code
claude

# 5. Select Model (inside Claude)
/model
# Select your custom model (e.g., Heimsense Model)
```

---

## Configuration

All configuration via environment variables (or `.env` file):

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTHROPIC_BASE_URL` | `https://api.openai.com/v1` | Upstream OpenAI-compatible API |
| `ANTHROPIC_API_KEY` | — | Fallback API key for upstream |
| `ANTHROPIC_CUSTOM_MODEL_OPTION  ` | — | Default model if not specified |
| `ANTHROPIC_CUSTOM_MODEL_OPTION_NAME` | — | Default model if not specified |
| `ANTHROPIC_CUSTOM_MODEL_OPTION_DESCRIPTION` | — | Default model if not specified |
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `REQUEST_TIMEOUT_MS` | `120000` | Upstream timeout (ms) |
| `MAX_RETRIES` | `3` | Retry attempts on 5xx errors |

### Example `.env`

```bash
ANTHROPIC_BASE_URL=https://api.openai.com/v1
ANTHROPIC_API_KEY=sk-your-api-key-here
ANTHROPIC_CUSTOM_MODEL_OPTION=gpt-5.1
ANTHROPIC_CUSTOM_FORCE_MODEL=
LISTEN_ADDR=:8080
REQUEST_TIMEOUT_MS=120000
MAX_RETRIES=3
```

---

## Supported Providers

Heimsense works with any OpenAI-compatible API:

### General LLM Providers

| Provider | Base URL | Notes |
|----------|----------|-------|
| OpenAI | `https://api.openai.com/v1` | Official GPT models |
| [DeepSeek](https://deepseek.com) | `https://api.deepseek.com/v1` | Excellent for coding, competitive pricing |
| [GLM (Zhipu AI)](https://open.bigmodel.cn) | `https://open.bigmodel.cn/api/paas/v4` | Chinese LLM, GLM-4 series |
| [MiniMax](https://minimax.chat) | `https://api.minimax.chat/v1` | Chinese LLM provider |
| [Groq](https://groq.com) | `https://api.groq.com/openai/v1` | Ultra-fast inference (LPU) |
| [Together AI](https://together.ai) | `https://api.together.xyz/v1` | Open-source models |
| [OpenRouter](https://openrouter.ai) | `https://openrouter.ai/api/v1` | Multi-provider gateway |
| [Fireworks AI](https://fireworks.ai) | `https://api.fireworks.ai/inference/v1` | Fast serverless inference |
| [Replicate](https://replicate.com) | `https://api.replicate.com/v1` | Model hosting platform |
| [Perplexity](https://perplexity.ai) | `https://api.perplexity.ai` | Search-augmented LLM |
| [Mistral](https://mistral.ai) | `https://api.mistral.ai/v1` | European LLM provider |
| [Cohere](https://cohere.com) | `https://api.cohere.ai/v1` | Enterprise LLM |
| [xAI (Grok)](https://x.ai) | `https://api.x.ai/v1` | Elon Musk's AI company |

### Coding-Focused LLMs

| Provider | Models | Best For |
|----------|--------|----------|
| [DeepSeek](https://deepseek.com) | `deepseek-coder` | Code generation, debugging |
| [Cursor](https://cursor.sh) | Various | AI-powered IDE |
| [Codeium](https://codeium.com) | `codeium` | Free code completion |
| [Tabnine](https://tabnine.com) | Various | Enterprise code assistant |
| [Amazon CodeWhisperer](https://aws.amazon.com/codewhisperer) | Various | AWS-integrated coding |
| [Sourcegraph Cody](https://sourcegraph.com/cody) | Various | Code understanding |
| [Replit AI](https://replit.com) | Various | Browser-based coding |
| [CodeLlama](https://huggingface.co/codellama) | `codellama-*` | Meta's code model (via Ollama/Together) |
| [StarCoder](https://huggingface.co/bigcode) | `starcoder*` | BigCode's models (via Ollama/Together) |

### Local / Self-Hosted

| Provider | Base URL | Notes |
|----------|----------|-------|
| [Ollama](https://ollama.ai) | `http://localhost:11434/v1` | Run models locally |
| [LM Studio](https://lmstudio.ai) | `http://localhost:1234/v1` | GUI for local models |
| [vLLM](https://github.com/vllm-project/vllm) | `http://localhost:8000/v1` | High-performance serving |
| [LocalAI](https://localai.io) | `http://localhost:8080/v1` | Drop-in OpenAI replacement |
| [Text Generation WebUI](https://github.com/oobabooga/text-generation-webui) | Varies | Flexible local inference |

### Popular Models by Use Case

| Use Case | Recommended Models |
|----------|-------------------|
| **General Chat** | `gpt-4o`, `claude-3-opus`, `glm-4`, `deepseek-chat` |
| **Coding** | `deepseek-coder`, `gpt-4o`, `claude-3.5-sonnet`, `codellama-70b` |
| **Fast/Cheap** | `gpt-4o-mini`, `deepseek-chat`, `glm-4-flash`, `groq-llama3` |
| **Large Context** | `claude-3-opus` (200K), `glm-4` (128K), `deepseek` (64K) |
| **Local** | `llama3`, `codellama`, `mistral`, `qwen2.5-coder` |

---

## Using with Claude Code

### Auto Setup (Recommended)

```bash
make setup
```

This updates `~/.claude/settings.json`:

```diff
 {
   "env": {
-    "ANTHROPIC_BASE_URL": "https://api.anthropic.com",
+    "ANTHROPIC_BASE_URL": "http://localhost:8080",
     "ANTHROPIC_AUTH_TOKEN": "sk-xxx",
   }
 }
```

**To revert:**

```bash
make revert
```

### Manual Setup

```bash
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_API_KEY=your-api-key
claude
```

---

## Container Deployment

### Podman

```bash
# Build
podman build -t heimsense:latest .

# Run
podman run -d \
  --name heimsense \
  -p 8080:8080 \
  --env-file .env \
  heimsense:latest

# Or with compose
podman-compose up -d
```

### Docker

```bash
# Build
docker build -t heimsense:latest .

# Run
docker run -d \
  --name heimsense \
  -p 8080:8080 \
  --env-file .env \
  heimsense:latest

# Or with compose
docker compose up -d
```

### Container Features

- Multi-stage build (~15MB image)
- Non-root user for security
- Read-only filesystem with tmpfs
- Health check support
- Graceful shutdown

---

## Make Targets

```bash
# Development
make run        # Build + start server
make dev        # Run via `go run`
make build      # Compile to ./bin/
make test       # Run tests
make fmt        # Format code
make lint       # Run go vet
make clean      # Remove build artifacts

# Claude Code Setup
make setup      # Configure Claude Code
make revert     # Revert settings

# Docker
make docker-build   # Build image
make docker-run     # Run with compose
make docker-stop    # Stop compose
make docker-logs    # View logs

# Podman
make podman-build   # Build image
make podman-run     # Run with compose
make podman-stop    # Stop compose
make podman-logs    # View logs

# Help
make help
```

---

## API Endpoints

### `POST /v1/messages`

**Non-streaming:**

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-key" \
  -d '{
    "model": "gpt-5.1",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Streaming:**

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-key" \
  -d '{
    "model": "gpt-5.1",
    "max_tokens": 1024,
    "stream": true,
    "messages": [{"role": "user", "content": "Tell me a story"}]
  }'
```

**With tools:**

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-key" \
  -d '{
    "model": "gpt-5.1",
    "max_tokens": 1024,
    "tools": [{
      "name": "get_weather",
      "description": "Get weather",
      "input_schema": {
        "type": "object",
        "properties": {"location": {"type": "string"}},
        "required": ["location"]
      }
    }],
    "messages": [{"role": "user", "content": "Weather in Tokyo?"}]
  }'
```

### `GET /health`

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

---

## API Translation Reference

### Request: Anthropic → OpenAI

| Anthropic | OpenAI | Notes |
|-----------|--------|-------|
| `model` | `model` | Pass-through with fallback |
| `messages` | `messages` | Arrays flattened |
| `system` | `messages[0]` | Prepended as system |
| `max_tokens` | `max_tokens` | Direct |
| `temperature` | `temperature` | Direct |
| `top_p` | `top_p` | Direct |
| `stream` | `stream` | Enables SSE |
| `stop_sequences` | `stop` | Direct |
| `tools` | `tools` | Function calling |

### Response: OpenAI → Anthropic

| OpenAI | Anthropic |
|--------|-----------|
| `choices[0].message.content` | `content[0].text` |
| `choices[0].message.tool_calls` | `content[].tool_use` |
| `usage.prompt_tokens` | `usage.input_tokens` |
| `usage.completion_tokens` | `usage.output_tokens` |
| `finish_reason: "stop"` | `stop_reason: "end_turn"` |
| `finish_reason: "length"` | `stop_reason: "max_tokens"` |
| `finish_reason: "tool_calls"` | `stop_reason: "tool_use"` |

### Streaming Events

```
message_start → content_block_start
  → content_block_delta (repeated)
    → content_block_stop → message_delta → message_stop
```

---

## Project Structure

```
heimsense/
├── .github/workflows/              # CI, Release, Docker
├── cmd/server/main.go              # Entry point
├── internal/
│   ├── adapter/transform.go        # Format transformation
│   ├── client/openai.go            # HTTP client + retry
│   ├── config/config.go            # Config loader
│   └── handler/
│       ├── messages.go             # Request handler
│       └── messages_test.go        # Tests
├── scripts/setup-claude.sh         # Claude Code setup
├── Containerfile                   # Container build
├── docker-compose.yaml             # Compose config
├── Makefile                        # Build targets
├── env.example                     # Config template
├── LICENSE                         # MIT License
└── go.mod                          # Module definition
```

---

## Development

### Run Tests

```bash
make test
```

### Build

```bash
make build  # ./bin/heimsense
```

### Code Quality

```bash
make fmt
make lint
```

---

## Troubleshooting

### Port in use

```bash
lsof -i :8080
LISTEN_ADDR=:8081 make run
```

### View logs

```bash
make podman-logs
# or
make docker-logs
```

### Auth errors

- Check `ANTHROPIC_API_KEY` in `.env`
- Verify `ANTHROPIC_BASE_URL` is correct
- Ensure key format matches provider requirements

---

## License

MIT

---

> *"Heimdall, guardian of the gods, will rise and sound the Gjallarhorn..."* — Now he guards your API gateway.
