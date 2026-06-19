# AutoXmate CLI

CLI tool that syncs with [autoxmate.github.io](https://autoxmate.github.io) — a knowledge base of 2,262+ security and system tools — and lets you search, query, install, and run them with parameterized commands.

## Quick Install

```bash
curl -fsSL https://autoxmate.github.io/downloads/install.sh | bash
```

Or download a pre-built binary from the [releases page](https://github.com/AutoXMate/AutoXmate/releases).

## Usage

```
autoxmate sync        Sync tool definitions from autoxmate.github.io
autoxmate list        List all available tools (2,262+)
autoxmate status      Check which tools are installed on your system
autoxmate install     Install tools (apt, brew, pip, go, git, etc.)
autoxmate search      Search tools by name, capability, or domain
autoxmate query       Structured/free-text query for exact commands
autoxmate run         Execute a tool with parameterized arguments
autoxmate exec        Open an interactive terminal session
autoxmate mcp         Start MCP server for AI model integration
autoxmate tui         Launch interactive terminal UI
```

### Examples

```bash
# Sync the tool database
autoxmate sync

# List all tools
autoxmate list

# Search for nmap
autoxmate search nmap

# Structured query
autoxmate query "domain(network):action(port-scan):target(10.10.10.0/24)"

# Install a tool
autoxmate install nmap

# Run a tool with parameters
autoxmate run nmap target=scanme.nmap.org

# Launch TUI
autoxmate tui
```

## Features

- **Offline cache** — tools are cached locally after `sync` for offline use
- **Multi-method installer** — auto-detects `apt`, `brew`, `pip`, `go`, `git`, and more
- **Structured query parser** — domain/action/target syntax for precise tool discovery
- **MCP server** — expose tools to AI models via the Model Context Protocol
- **TUI** — Bubble Tea terminal UI with keybindings and command preview
- **SQLite-backed** — fast local storage with integrity verification

## Build from Source

```bash
git clone https://github.com/AutoXMate/AutoXmate.git
cd AutoXmate
go build -o autoxmate -ldflags="-s -w" .
```

Requires Go 1.21+.

## Install via Go

```bash
go install github.com/AutoXMate/AutoXmate@latest
```

## Data Source

All tool definitions come from [autoxmate.github.io](https://autoxmate.github.io), a static knowledge repository aggregating definitions from GTFOBins, LOLBAS, LOLDrivers, HijackLibs, and 20+ other security tool collections.

## License

MIT
