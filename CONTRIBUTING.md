# Contributing to StreamSpace

Thank you for your interest in contributing to StreamSpace! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful, inclusive, and professional in all interactions.

## How to Contribute

### Reporting Bugs

1. Check existing issues to avoid duplicates
2. Use the bug report template
3. Include:
   - StreamSpace version
   - Kubernetes version
   - Steps to reproduce
   - Expected vs actual behavior
   - Logs and error messages

### Suggesting Features

1. Check existing feature requests
2. Use the feature request template
3. Describe:
   - Use case and problem
   - Proposed solution
   - Alternatives considered
   - Impact on existing functionality

### Pull Requests

1. **Fork** the repository
2. **Create a branch**: `git checkout -b feature/my-feature`
3. **Make changes** with clear, focused commits
4. **Test** your changes thoroughly
5. **Document** new features or changes
6. **Submit PR** with clear description

#### PR Guidelines

- Keep PRs focused on a single feature/fix
- Write clear commit messages
- Update documentation
- Add tests for new functionality
- Ensure CI passes
- Request review from maintainers

### Development Setup

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed setup instructions.

Quick start:

```bash
# Clone your fork
git clone https://github.com/yourusername/streamspace.git
cd streamspace

# Install dependencies
make install-deps

# Run tests
make test

# Start local development
make dev
```

## Project Structure

```
streamspace/
├── controller/     # Go workspace controller
├── api/           # API backend (Go/Python)
├── ui/            # React frontend
├── manifests/     # Kubernetes manifests
├── chart/         # Helm chart
├── docs/          # Documentation
└── scripts/       # Utility scripts
```

## Coding Standards

### Go (Controller/API)

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` and `golint`
- Write tests for new code
- Document public APIs
- Handle errors explicitly

### JavaScript/TypeScript (UI)

- Use ESLint and Prettier
- Follow React best practices
- Write component tests
- Use TypeScript for type safety

### Kubernetes Manifests

- Use descriptive names
- Add resource limits
- Include labels and annotations
- Document via comments

## Testing

### Unit Tests

```bash
# Controller
cd controller && make test

# API
cd api && go test ./...

# UI
cd ui && npm test
```

### Integration Tests

```bash
# Full integration test suite
./scripts/run-integration-tests.sh
```

### Manual Testing

```bash
# Deploy to test cluster
kubectl create namespace streamspace-dev
helm install streamspace-dev ./chart -n streamspace-dev -f test-values.yaml

# Test session creation
kubectl apply -f examples/test-session.yaml
```

## Documentation

- Update README.md for user-facing changes
- Update docs/ for architectural changes
- Add inline comments for complex logic
- Include examples for new features

## Release Process

Maintainers will:

1. Update version in `chart/Chart.yaml`
2. Update CHANGELOG.md
3. Create git tag
4. Build and push Docker images
5. Publish Helm chart
6. Create GitHub release

## Getting Help

- **Documentation**: Check docs/ first
- **Discord**: https://discord.gg/streamspace
- **GitHub Discussions**: For questions and ideas
- **GitHub Issues**: For bugs and feature requests

## Recognition

Contributors are recognized in:
- CONTRIBUTORS.md
- Release notes
- Project README

Thank you for contributing to StreamSpace! 🚀
