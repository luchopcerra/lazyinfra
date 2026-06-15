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

### Homebrew (macOS/Linux)

```bash
brew install luchopcerra/tap/lazyinfra
```

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
+-- aws/          # AWS SDK clients and service methods
+-- ui/           # Bubble Tea root model, messages, updates, and layout
|   +-- views/    # Service-specific views
+-- main.go       # Entry point
+-- go.mod
+-- README.md
```

## Contributing

Contributions are welcome. The architecture intentionally separates AWS access from UI rendering, and each service fetch runs through asynchronous `tea.Cmd` stubs that return typed messages back into the Bubble Tea update loop.

That design should make new AWS features straightforward to add:

1. Add or extend a method in `aws/`.
2. Return a typed Bubble Tea message in `ui/messages.go`.
3. Trigger the work through a `tea.Cmd`.
4. Render the new state inside the relevant `ui/views/` model.

Open issues, propose improvements, or send a pull request for the next piece of serverless infrastructure you want to make easier from the terminal.
