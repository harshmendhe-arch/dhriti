# Archived: Project has Moved

This repository is no longer maintained and has been archived for provenance.

The project has continued under the name [Crush][crush], developed by the original author and the Charm team.

Please follow [Crush][crush] for ongoing development.

[crush]: https://github.com/charmbracelet/crush


# ⌬ Dhriti

> **⚠️ Early Development Notice:** This project is in early development and is not yet ready for production use. Features may change, break, or be incomplete. Use at your own risk.

A powerful terminal-based AI assistant for developers, providing intelligent coding assistance directly in your terminal.

## Overview

Dhriti is a Go-based CLI application that brings AI assistance to your terminal. It provides a TUI (Terminal User Interface) for interacting with various AI models to help with coding tasks, debugging, and more.

## Features

- **Interactive TUI**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a smooth terminal experience
- **AI Providers**: Support for OpenAI and Google Gemini
- **Session Management**: Save and manage multiple conversation sessions
- **Tool Integration**: AI can execute commands, search files, and modify code
- **Vim-like Editor**: Integrated editor with text input capabilities
- **Persistent Storage**: SQLite database for storing conversations and sessions
- **LSP Integration**: Language Server Protocol support for code intelligence
- **File Change Tracking**: Track and visualize file changes during sessions
- **External Editor Support**: Open your preferred editor for composing messages
- **Named Arguments for Custom Commands**: Create powerful custom commands with multiple named placeholders

## Installation

### curl (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/harshmendhe-arch/dhriti/main/install | bash
```

### npm (Node 18+)

```bash
npm install -g dhriti
# or from git until published:
npm install -g git+https://github.com/harshmendhe-arch/dhriti.git#main:npm
```

### winget (Windows)

```powershell
# After the package is published to winget-pkgs:
winget install harshmendhe-arch.dhriti
```

Manifests for submission live under [`winget/`](./winget).

### Build from source

```bash
# From the repository root
go build -o dhriti

# Run
./dhriti
```

### First-run login (ChatGPT + Gemini + gateway)

```bash
dhriti login
```

Opens your browser for sign-in, stores keys in `~/.dhriti.json`, and optionally configures the WebSocket gateway. Use `dhriti login --gateway-only` to set only the gateway URL.

See [Development](#development) for more details.

## Configuration

Dhriti looks for configuration in the following locations:

- `$HOME/.dhriti.json`
- `$XDG_CONFIG_HOME/dhriti/.dhriti.json`
- `./.dhriti.json` (local directory)

### Auto Compact Feature

OpenCode includes an auto compact feature that automatically summarizes your conversation when it approaches the model's context window limit. When enabled (default setting), this feature:

- Monitors token usage during your conversation
- Automatically triggers summarization when usage reaches 95% of the model's context window
- Creates a new session with the summary, allowing you to continue your work without losing context
- Helps prevent "out of context" errors that can occur with long conversations

You can enable or disable this feature in your configuration file:

```json
{
  "autoCompact": true // default is true
}
```

### Environment Variables

You can configure Dhriti using environment variables:

| Environment Variable | Purpose                        |
| -------------------- | ------------------------------ |
| `OPENAI_API_KEY`     | For OpenAI models              |
| `GEMINI_API_KEY`     | For Google Gemini models       |
| `SHELL`              | Default shell to use (if not specified in config) |
| `DHRITI_GOOGLE_CLIENT_ID` | OAuth client ID for `dhriti login` Gemini flow |
| `DHRITI_GOOGLE_CLIENT_SECRET` | Matching OAuth client secret (installed app) |

Prefer `dhriti login` over exporting keys — it opens the browser and writes `~/.dhriti.json`.

### WebSocket Gateway (no local API keys)

Instead of `OPENAI_API_KEY` / `GEMINI_API_KEY`, you can route all LLM traffic through a remote WebSocket gateway that speaks the [OpenAI Realtime](https://platform.openai.com/docs/guides/realtime) protocol. When `gateway.url` is set, streaming and non-streaming calls both go through the gateway; when unset, OpenAI/Gemini behave as before.

```json
{
  "gateway": {
    "url": "wss://your-gateway.example.com/realtime",
    "apiKey": "optional-gateway-token",
    "model": "gpt-realtime"
  }
}
```

| Field    | Required | Purpose |
| -------- | -------- | ------- |
| `url`    | yes      | `ws://` or `wss://` endpoint |
| `apiKey` | no       | Sent as `Authorization: Bearer …` if present |
| `model`  | no       | Query `?model=`; falls back to the agent stream model, then `gpt-realtime` |

Config is read from `$HOME/.dhriti.json`, `$XDG_CONFIG_HOME/dhriti/.dhriti.json`, or `./.dhriti.json`.

### Shell Configuration

Dhriti allows you to configure the shell used by the bash tool. By default, it uses the shell specified in the `SHELL` environment variable, or falls back to `/bin/bash` if not set.

You can override this in your configuration file:

```json
{
  "shell": {
    "path": "/bin/zsh",
    "args": ["-l"]
  }
}
```

This is useful if you want to use a different shell than your default system shell, or if you need to pass specific arguments to the shell.

### Configuration File Structure

```json
{
  "data": {
    "directory": ".dhriti"
  },
  "providers": {
    "openai": {
      "apiKey": "your-api-key",
      "disabled": false
    },
    "gemini": {
      "apiKey": "your-api-key",
      "disabled": false
    }
  },
  "agents": {
    "coder": {
      "model": "gpt-4.1",
      "maxTokens": 5000
    },
    "task": {
      "model": "gpt-4.1",
      "maxTokens": 5000
    },
    "title": {
      "model": "gpt-4.1-mini",
      "maxTokens": 80
    }
  },
  "shell": {
    "path": "/bin/bash",
    "args": ["-l"]
  },
  "mcpServers": {
    "example": {
      "type": "stdio",
      "command": "path/to/mcp-server",
      "env": [],
      "args": []
    }
  },
  "lsp": {
    "go": {
      "disabled": false,
      "command": "gopls"
    }
  },
  "debug": false,
  "debugLSP": false,
  "autoCompact": true
}
```

## Supported AI Models

Dhriti supports a variety of AI models from different providers:

### OpenAI

- GPT-4.1 family (gpt-4.1, gpt-4.1-mini, gpt-4.1-nano)
- GPT-4.5 Preview
- GPT-4o family (gpt-4o, gpt-4o-mini)
- O1 family (o1, o1-pro, o1-mini)
- O3 family (o3, o3-mini)
- O4 Mini

### Google

- Gemini 2.5
- Gemini 2.5 Flash
- Gemini 2.0 Flash
- Gemini 2.0 Flash Lite

## Usage

```bash
# Start Dhriti
dhriti

# Start with debug logging
dhriti -d

# Start with a specific working directory
dhriti -c /path/to/project
```

## Non-interactive Prompt Mode

You can run Dhriti in non-interactive mode by passing a prompt directly as a command-line argument. This is useful for scripting, automation, or when you want a quick answer without launching the full TUI.

```bash
# Run a single prompt and print the AI's response to the terminal
dhriti -p "Explain the use of context in Go"

# Get response in JSON format
dhriti -p "Explain the use of context in Go" -f json

# Run without showing the spinner (useful for scripts)
dhriti -p "Explain the use of context in Go" -q
```

In this mode, Dhriti will process your prompt, print the result to standard output, and then exit. All permissions are auto-approved for the session.

By default, a spinner animation is displayed while the model is processing your query. You can disable this spinner with the `-q` or `--quiet` flag, which is particularly useful when running Dhriti from scripts or automated workflows.

### Output Formats

Dhriti supports the following output formats in non-interactive mode:

| Format | Description                     |
| ------ | ------------------------------- |
| `text` | Plain text output (default)     |
| `json` | Output wrapped in a JSON object |

The output format is implemented as a strongly-typed `OutputFormat` in the codebase, ensuring type safety and validation when processing outputs.

## Command-line Flags

| Flag              | Short | Description                                         |
| ----------------- | ----- | --------------------------------------------------- |
| `--help`          | `-h`  | Display help information                            |
| `--debug`         | `-d`  | Enable debug mode                                   |
| `--cwd`           | `-c`  | Set current working directory                       |
| `--prompt`        | `-p`  | Run a single prompt in non-interactive mode         |
| `--output-format` | `-f`  | Output format for non-interactive mode (text, json) |
| `--quiet`         | `-q`  | Hide spinner in non-interactive mode                |

## Keyboard Shortcuts

### Global Shortcuts

| Shortcut | Action                                                  |
| -------- | ------------------------------------------------------- |
| `Ctrl+C` | Quit application                                        |
| `Ctrl+?` | Toggle help dialog                                      |
| `?`      | Toggle help dialog (when not in editing mode)           |
| `Ctrl+L` | View logs                                               |
| `Ctrl+A` | Switch session                                          |
| `Ctrl+K` | Command dialog                                          |
| `Ctrl+O` | Toggle model selection dialog                           |
| `Esc`    | Close current overlay/dialog or return to previous mode |

### Chat Page Shortcuts

| Shortcut | Action                                  |
| -------- | --------------------------------------- |
| `Ctrl+N` | Create new session                      |
| `Ctrl+X` | Cancel current operation/generation     |
| `i`      | Focus editor (when not in writing mode) |
| `Esc`    | Exit writing mode and focus messages    |

### Editor Shortcuts

| Shortcut            | Action                                    |
| ------------------- | ----------------------------------------- |
| `Ctrl+S`            | Send message (when editor is focused)     |
| `Enter` or `Ctrl+S` | Send message (when editor is not focused) |
| `Ctrl+E`            | Open external editor                      |
| `Esc`               | Blur editor and focus messages            |

### Session Dialog Shortcuts

| Shortcut   | Action           |
| ---------- | ---------------- |
| `↑` or `k` | Previous session |
| `↓` or `j` | Next session     |
| `Enter`    | Select session   |
| `Esc`      | Close dialog     |

### Model Dialog Shortcuts

| Shortcut   | Action            |
| ---------- | ----------------- |
| `↑` or `k` | Move up           |
| `↓` or `j` | Move down         |
| `←` or `h` | Previous provider |
| `→` or `l` | Next provider     |
| `Esc`      | Close dialog      |

### Permission Dialog Shortcuts

| Shortcut                | Action                       |
| ----------------------- | ---------------------------- |
| `←` or `left`           | Switch options left          |
| `→` or `right` or `tab` | Switch options right         |
| `Enter` or `space`      | Confirm selection            |
| `a`                     | Allow permission             |
| `A`                     | Allow permission for session |
| `d`                     | Deny permission              |

### Logs Page Shortcuts

| Shortcut           | Action              |
| ------------------ | ------------------- |
| `Backspace` or `q` | Return to chat page |

## AI Assistant Tools

Dhriti's AI assistant has access to various tools to help with coding tasks:

### File and Code Tools

| Tool          | Description                 | Parameters                                                                               |
| ------------- | --------------------------- | ---------------------------------------------------------------------------------------- |
| `glob`        | Find files by pattern       | `pattern` (required), `path` (optional)                                                  |
| `grep`        | Search file contents        | `pattern` (required), `path` (optional), `include` (optional), `literal_text` (optional) |
| `ls`          | List directory contents     | `path` (optional), `ignore` (optional array of patterns)                                 |
| `view`        | View file contents          | `file_path` (required), `offset` (optional), `limit` (optional)                          |
| `write`       | Write to files              | `file_path` (required), `content` (required)                                             |
| `edit`        | Edit files                  | Various parameters for file editing                                                      |
| `patch`       | Apply patches to files      | `file_path` (required), `diff` (required)                                                |
| `diagnostics` | Get diagnostics information | `file_path` (optional)                                                                   |

### Other Tools

| Tool          | Description                            | Parameters                                                                                |
| ------------- | -------------------------------------- | ----------------------------------------------------------------------------------------- |
| `bash`        | Execute shell commands                 | `command` (required), `timeout` (optional)                                                |
| `fetch`       | Fetch data from URLs                   | `url` (required), `format` (required), `timeout` (optional)                               |
| `sourcegraph` | Search code across public repositories | `query` (required), `count` (optional), `context_window` (optional), `timeout` (optional) |
| `agent`       | Run sub-tasks with the AI agent        | `prompt` (required)                                                                       |

## Architecture

Dhriti is built with a modular architecture:

- **cmd**: Command-line interface using Cobra
- **internal/app**: Core application services
- **internal/config**: Configuration management
- **internal/db**: Database operations and migrations
- **internal/llm**: LLM providers and tools integration
- **internal/tui**: Terminal UI components and layouts
- **internal/logging**: Logging infrastructure
- **internal/message**: Message handling
- **internal/session**: Session management
- **internal/lsp**: Language Server Protocol integration

## Custom Commands

Dhriti supports custom commands that can be created by users to quickly send predefined prompts to the AI assistant.

### Creating Custom Commands

Custom commands are predefined prompts stored as Markdown files in one of three locations:

1. **User Commands** (prefixed with `user:`):

   ```
   $XDG_CONFIG_HOME/dhriti/commands/
   ```

   (typically `~/.config/dhriti/commands/` on Linux/macOS)

   or

   ```
   $HOME/.dhriti/commands/
   ```

2. **Project Commands** (prefixed with `project:`):

   ```
   <PROJECT DIR>/.dhriti/commands/
   ```

Each `.md` file in these directories becomes a custom command. The file name (without extension) becomes the command ID.

For example, creating a file at `~/.config/dhriti/commands/prime-context.md` with content:

```markdown
RUN git ls-files
READ README.md
```

This creates a command called `user:prime-context`.

### Command Arguments

Dhriti supports named arguments in custom commands using placeholders in the format `$NAME` (where NAME consists of uppercase letters, numbers, and underscores, and must start with a letter).

For example:

```markdown
# Fetch Context for Issue $ISSUE_NUMBER

RUN gh issue view $ISSUE_NUMBER --json title,body,comments
RUN git grep --author="$AUTHOR_NAME" -n .
RUN grep -R "$SEARCH_PATTERN" $DIRECTORY
```

When you run a command with arguments, Dhriti will prompt you to enter values for each unique placeholder. Named arguments provide several benefits:

- Clear identification of what each argument represents
- Ability to use the same argument multiple times
- Better organization for commands with multiple inputs

### Organizing Commands

You can organize commands in subdirectories:

```
~/.config/dhriti/commands/git/commit.md
```

This creates a command with ID `user:git:commit`.

### Using Custom Commands

1. Press `Ctrl+K` to open the command dialog
2. Select your custom command (prefixed with either `user:` or `project:`)
3. Press Enter to execute the command

The content of the command file will be sent as a message to the AI assistant.

### Built-in Commands

Dhriti includes several built-in commands:

| Command            | Description                                                                                         |
| ------------------ | --------------------------------------------------------------------------------------------------- |
| Initialize Project | Creates or updates the Dhriti.md memory file with project-specific information |
| Compact Session    | Manually triggers the summarization of the current session, creating a new session with the summary |

## MCP (Model Context Protocol)

Dhriti implements the Model Context Protocol (MCP) to extend its capabilities through external tools. MCP provides a standardized way for the AI assistant to interact with external services and tools.

### MCP Features

- **External Tool Integration**: Connect to external tools and services via a standardized protocol
- **Tool Discovery**: Automatically discover available tools from MCP servers
- **Multiple Connection Types**:
  - **Stdio**: Communicate with tools via standard input/output
  - **SSE**: Communicate with tools via Server-Sent Events
- **Security**: Permission system for controlling access to MCP tools

### Configuring MCP Servers

MCP servers are defined in the configuration file under the `mcpServers` section:

```json
{
  "mcpServers": {
    "example": {
      "type": "stdio",
      "command": "path/to/mcp-server",
      "env": [],
      "args": []
    },
    "web-example": {
      "type": "sse",
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer token"
      }
    }
  }
}
```

### MCP Tool Usage

Once configured, MCP tools are automatically available to the AI assistant alongside built-in tools. They follow the same permission model as other tools, requiring user approval before execution.

## LSP (Language Server Protocol)

Dhriti integrates with Language Server Protocol to provide code intelligence features across multiple programming languages.

### LSP Features

- **Multi-language Support**: Connect to language servers for different programming languages
- **Diagnostics**: Receive error checking and linting information
- **File Watching**: Automatically notify language servers of file changes

### Configuring LSP

Language servers are configured in the configuration file under the `lsp` section:

```json
{
  "lsp": {
    "go": {
      "disabled": false,
      "command": "gopls"
    },
    "typescript": {
      "disabled": false,
      "command": "typescript-language-server",
      "args": ["--stdio"]
    }
  }
}
```

### LSP Integration with AI

The AI assistant can access LSP features through the `diagnostics` tool, allowing it to:

- Check for errors in your code
- Suggest fixes based on diagnostics

While the LSP client implementation supports the full LSP protocol (including completions, hover, definition, etc.), currently only diagnostics are exposed to the AI assistant.

## Development

### Prerequisites

- Go 1.24.0 or higher

### Building from Source

```bash
# From the repository root
go build -o dhriti

# Run
./dhriti
```

## Acknowledgments

Dhriti gratefully acknowledges the contributions and support from these key individuals:

- [@isaacphi](https://github.com/isaacphi) - For the [mcp-language-server](https://github.com/isaacphi/mcp-language-server) project which provided the foundation for our LSP client implementation
- [@adamdottv](https://github.com/adamdottv) - For the design direction and UI/UX architecture

Special thanks to the broader open source community whose tools and libraries have made this project possible.

## License

Dhriti is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

Dhriti is derived from [OpenCode](https://github.com/opencode-ai/opencode), originally created by the OpenCode authors. Upstream contributions are gratefully acknowledged.

## Contributing

Contributions are welcome! Here's how you can contribute:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please make sure to update tests as appropriate and follow the existing code style.
