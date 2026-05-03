# Local Development Guide

Welcome to the Heimsense development guide! This document outlines how to set up your local environment, run the project, and contribute to the codebase.

## Prerequisites

To develop Heimsense locally, you need the following tools installed:

1. **Go (Golang)**: Version `1.25` or higher.
2. **Make**: Required for running the build and test scripts defined in the `Makefile`.
3. **Git**: For version control.

---

## Initial Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/fajarhide/heimsense.git
   cd heimsense
   ```

2. **Download dependencies:**
   ```bash
   go mod download
   ```

3. **Initialize Configuration:**
   Heimsense requires a `config.toml` file to run locally. You can use the setup wizard to generate it automatically:
   ```bash
   make setup
   ```
   *Note: This wizard will place the configuration file at `~/.heimsense/config.toml`.*

---

## Development Workflow

We use a `Makefile` to streamline development tasks. Here are the primary commands you will use:

### Running the Server Locally
To start the Heimsense proxy server locally (which will listen on `:8080` by default):
```bash
make run
```
*Tip: If you want to run it with live reload during development (e.g., using `air` or `nodemon`), you can run `make dev`.*

### Testing the Code
Before committing any changes, ensure all unit tests pass:
```bash
make test
```
To run the full Continuous Integration (CI) suite locally (Formatter, Linter, Tests, and Build check):
```bash
make ci
```

### Building the Binary
To compile a standalone binary for your current operating system into the `./bin/` directory:
```bash
make build
```

---

## Code Architecture Overview

If you are looking to contribute or understand the codebase, here is a quick overview of the `internal/` packages:

- **`internal/adapter/`**: Contains the core Go structs for Anthropic (`messages`) and OpenAI (`chat/completions`) request/response formats.
- **`internal/translator/`**: The core translation engine.
  - `formats.go`: Detects whether an incoming request is Anthropic or OpenAI.
  - `inbound.go`: Standardizes the request into a common OpenAI format.
  - `outbound.go`: Prepares the standardized request for the target upstream provider.
- **`internal/handler/`**: Contains the `UniversalRouterHandler` which glues the HTTP requests, distillation, and translation engines together.
- **`internal/config/`**: TOML configuration parsing and loading.
- **`internal/omni/`**: Logic for token distillation via the Omni MCP server.

---

## Contributing

1. Create a new branch for your feature or bugfix (`git checkout -b feature/your-feature`).
2. Write unit tests for your changes.
3. Ensure the code passes all checks by running `make ci`.
4. Submit a Pull Request!
