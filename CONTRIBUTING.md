# Contributing to StagingBrief

Thanks for your interest in contributing! This guide will help you get started.

## Getting started

### Prerequisites

- Go 1.26 or later
- Docker (for testing the containerized build)

### Local setup

1. Clone the repository:
   ```bash
   git clone https://github.com/bytesfue/stagebrief.git
   cd stagebrief
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Set up your local environment:
   ```bash
   cp .env.example .env
   # Edit .env with your GitLab, OpenAI, and Slack credentials for local testing
   ```

## Development workflow

### Running locally

```bash
make run
```

Or directly:
```bash
go run ./cmd/notify
```

### Building

```bash
make build
# Binary: ./bin/notify
```

### Testing

```bash
go test ./...
go test -v ./...        # Verbose
go test -race ./...     # Race detector
go test -cover ./...    # Coverage
```

We use `httptest` for HTTP client testing. See `internal/llm/client_test.go` and `internal/gitlab/pipelines_test.go` for examples.

### Code quality

```bash
go vet ./...
golangci-lint run ./...
go mod tidy
```

## Submitting changes

1. **Create a branch** for your feature or fix:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Write tests** — aim for meaningful coverage. New features should include tests.

3. **Keep commits clean** — use clear, descriptive commit messages. Reference issues when relevant (e.g., "Fix #42: guard 429 nil-pointer").

4. **Run tests locally** — ensure `go test ./...` passes and `go vet ./...` is clean.

5. **Open a pull request** with a clear description of what you're changing and why.

## Code style

- Follow standard Go conventions (use `gofmt`; most editors can autoformat on save).
- Prefer clarity over brevity.
- Add comments only where the *why* isn't obvious.
- Keep functions small and focused.

## Reporting issues

Found a bug or have a feature request? Open an issue with:
- **For bugs:** steps to reproduce, expected vs. actual behavior, and your environment.
- **For features:** use case, expected behavior, and any context that helps us understand the request.

## Questions?

Open a discussion or issue on GitHub. We're happy to help!

---

**Code of Conduct:** This project adheres to the Contributor Covenant. By participating, you agree to uphold its code of conduct.
