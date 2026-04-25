<div align="center">

# 🤝 Contributing to StreamSpace

**Help us build the future of container streaming.**

</div>

---

## 🏗️ Project Structure

```
streamspace/
├── api/                # Control Plane API (Go)
├── agents/             # Execution Agents
│   └── k8s-agent/      # Kubernetes Agent (Go)
├── ui/                 # Web UI (React)
├── manifests/          # Kubernetes manifests
├── chart/              # Helm chart
└── docs/               # Documentation
```

## 🛠️ Development Setup

### Prerequisites

- Go 1.21+
- Node.js 18+
- Docker & Kubernetes (k3s recommended)

### Quick Start

1. **Clone the repo**:

    ```bash
    git clone https://github.com/streamspace-dev/streamspace.git
    cd streamspace
    ```

2. **Install Dependencies**:

    ```bash
    cd ui && npm install
    cd ../api && go mod download
    cd ../agents/k8s-agent && go mod download
    ```

3. **Run Tests**:

    ```bash
    # API
    cd api && go test ./...

    # K8s Agent
    cd agents/k8s-agent && go test ./...

    # UI
    cd ui && npm test
    ```

## 📝 Coding Standards

### Go (API & Agents)

- Follow [Effective Go](https://golang.org/doc/effective_go.html).
- Use `gofmt` and `golint`.
- **Architecture**: Respect the Control Plane / Agent separation. Agents should be stateless executors.

### React (UI)

- Use TypeScript.
- Follow Functional Component patterns.
- Use Material-UI for components.

## 🧪 Testing Guidelines

- **Unit Tests**: Required for all new logic.
- **Integration Tests**: Run `./tests/scripts/run-integration-tests.sh` before submitting PRs.
- **Documentation**: Update relevant docs in `docs/` if architecture changes.

## 📦 Pull Request Process

1. Fork the repository.
2. Create a feature branch (`feature/my-cool-feature`).
3. Commit changes.
4. Push to your fork.
5. Open a Pull Request.

---

<div align="center">
  <sub>StreamSpace Contribution Guide</sub>
</div>
