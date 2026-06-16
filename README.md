# **lazyinfra**

**lazyinfra** is a fast, keyboard-driven AWS Serverless TUI for backend developers who want to inspect and interact with their infrastructure without leaving the terminal. Built in Go with Bubble Tea, it uses asynchronous commands so AWS calls stay off the UI thread and the interface remains responsive during local development.

> [!NOTE]
> This project is in its early foundation stage. The current AWS layer includes structured stubs and SDK client wiring so real AWS and LocalStack integrations can be added incrementally.

## Demo

> [!TIP]
> Demo GIF coming soon. We plan to record the first walkthrough with Charm's [`vhs`](https://github.com/charmbracelet/vhs) once the MVP flows are wired to live AWS and LocalStack data.

```text
+ lazyinfra ---------+  API Gateway
| API Gateway        |
| AWS Lambda         |  backend-dev-http  HTTP
| CloudWatch         |    GET    /users        -> users-list
| CloudFront         |    POST   /users        -> users-create
+--------------------+    POST   /orders       -> orders-process
```

## Key Features

- **AWS Lambda view and invocation:** list functions with runtime, memory size, and last modified metadata, with a placeholder flow for test-payload invocation.
- **API Gateway routes tree:** inspect REST and HTTP APIs, routes, HTTP methods, and Lambda integrations from a scannable terminal view.
- **CloudWatch logs tailing:** browse log groups and stream log output into a viewport, with `ERROR` and `Exception` lines highlighted for fast debugging.
- **CloudFront cache invalidation:** list distributions with status and domain name, plus a placeholder invalidation form for paths such as `/*`.

## Tech Stack

- **Go**: compiled, portable CLI foundation.
- **Bubble Tea**: Elm-style TUI architecture with `Model`, `Update`, and `View`.
- **Lipgloss**: terminal layout and styling.
- **Bubbles**: reusable TUI components such as lists, viewports, text inputs, and spinners.
- **AWS SDK for Go v2**: AWS client foundation for Lambda, API Gateway, CloudWatch Logs, and CloudFront.

## Getting Started

### Install Script (macOS/Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/luchopcerra/lazyinfra/main/install.sh | bash
```

### Install Script (Windows PowerShell)

```powershell
irm https://raw.githubusercontent.com/luchopcerra/lazyinfra/main/install.ps1 | iex
```

### Homebrew (macOS/Linux)

```bash
brew install luchopcerra/tap/lazyinfra
```

### WinGet (Windows)

```powershell
winget install --id luchopcerra.lazyinfra
```

WinGet availability starts after the generated manifest pull request is accepted
into [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs).

### Prerequisites

- Go installed locally (if building from source).
- An AWS profile configured through the standard AWS config files, or a LocalStack environment for local development.

> [!IMPORTANT]
> `lazyinfra` is designed to support both real AWS profiles and LocalStack. For LocalStack, the current scaffold reads `LOCALSTACK_ENDPOINT`, or uses `http://localhost:4566` when `LAZYINFRA_LOCALSTACK=1`.

### Clone

```bash
git clone https://github.com/your-org/lazyinfra.git
cd lazyinfra
```

### Run Tests

```bash
go test ./...
```

### Run The TUI

```bash
go run main.go
```

### Run With An AWS Profile

```bash
AWS_PROFILE=dev AWS_REGION=us-east-1 go run main.go
```

### Run Against LocalStack

```bash
LAZYINFRA_LOCALSTACK=1 AWS_REGION=us-east-1 go run main.go
```

Or provide an explicit endpoint:

```bash
LOCALSTACK_ENDPOINT=http://localhost:4566 AWS_REGION=us-east-1 go run main.go
```

## Project Structure

```text
lazyinfra/
+-- .agents/      # Project-specific AI agent skills (Go patterns & testing)
+-- .github/
|   +-- workflows/
|       +-- release.yml  # GoReleaser-based release publishing
|       +-- ci.yml       # Automated build and test workflow
|   +-- ISSUE_TEMPLATE/
|       +-- agent_feature.md # Template for community AI agent feature requests
+-- aws/          # AWS SDK clients and service methods
+-- install.sh    # macOS/Linux installer for GitHub release assets
+-- install.ps1   # Windows PowerShell installer for GitHub release assets
+-- ui/           # Bubble Tea root model, messages, updates, and layout
|   +-- views/    # Service-specific views
+-- Agents.md     # Built-in AI Agent specifications
+-- main.go       # Entry point
+-- go.mod
+-- README.md
```

## AI Agent & Workspace Skills

To improve our development workflow and help open-source contributors build robust features, we have designed specifications for a built-in AI Copilot and installed workspace-scoped agent skills:

- **AI Agent Specification**: Read [Agents.md](Agents.md) to learn about the architectural plans for context-aware AWS debugging, Lambda invocation helpers, and logs diagnostics.
- **Developer Skills**: Coding assistants you pair with inside this repository will automatically discover and follow the development guidelines defined in:
  - [.agents/skills/golang-patterns/SKILL.md](.agents/skills/golang-patterns/SKILL.md): Idiomatic Go design patterns (functional options, error flow, resource management).
  - [.agents/skills/golang-testing/SKILL.md](.agents/skills/golang-testing/SKILL.md): Testing standards (table-driven tests, subtests, parallel execution).

## Contributing

Contributions are welcome. The architecture intentionally separates AWS access from UI rendering, and each service fetch runs through asynchronous `tea.Cmd` stubs that return typed messages back into the Bubble Tea update loop.

That design should make new AWS features straightforward to add:

1. Add or extend a method in `aws/`.
2. Return a typed Bubble Tea message in `ui/messages.go`.
3. Trigger the work through a `tea.Cmd`.
4. Render the new state inside the relevant `ui/views/` model.

Open issues, propose improvements, or send a pull request for the next piece of serverless infrastructure you want to make easier from the terminal. Use our custom issue templates to propose new AI Agent features and tools.

