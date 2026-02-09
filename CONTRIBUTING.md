# Contributing to GoliveKit

Thank you for your interest in contributing to GoliveKit! This document provides guidelines for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/golivekit.git`
3. Create a branch: `git checkout -b feature/my-feature`

## Development Setup

```bash
# Install dependencies
go mod download

# Run tests
go test ./... -race -v

# Run linter
go vet ./...

# Run an example
cd examples/counter && go run main.go
```

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions focused and small

## Testing

- Write tests for new functionality
- Run tests with race detector: `go test ./... -race`
- Aim for good coverage on critical paths

```bash
# Run specific package tests
go test ./pkg/core/... -v

# Run specific test
go test ./pkg/core -run TestSocketSend -v
```

## Commit Messages

Use clear, descriptive commit messages:

```
Add WebSocket reconnection with exponential backoff

- Implement retry logic with jitter
- Add max retry configuration
- Update client to handle reconnection
```

## Pull Request Process

1. Ensure tests pass: `go test ./... -race`
2. Update documentation if needed
3. Add entry to CHANGELOG.md for significant changes
4. Request review from maintainers

## Areas for Contribution

- **Bug fixes**: Check open issues
- **Documentation**: Improve README, add examples
- **Tests**: Increase coverage, add edge cases
- **Features**: Discuss in an issue first
- **Performance**: Profile and optimize hot paths

## Questions?

Open an issue for questions or discussion.
