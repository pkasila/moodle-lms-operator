# Contributing to Moodle LMS Operator

Thank you for your interest in contributing to the Moodle LMS Operator! This document provides guidelines and instructions for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md) to ensure a welcoming environment for all contributors.

## Getting Started

### Prerequisites

- **Go**: v1.24.6 or later
- **Docker**: v17.03 or later
- **kubectl**: v1.11.3 or later
- **kind**: For local testing (optional but recommended)
- **Kubebuilder**: v4.x
- **Git**: For version control

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/moodle-lms-operator.git
   cd moodle-lms-operator
   ```
3. Add the upstream repository:
   ```bash
   git remote add upstream https://github.com/pkasila/moodle-lms-operator.git
   ```

### Setting Up Development Environment

1. **Install dependencies:**
   ```bash
   go mod download
   ```

2. **Install CRDs in your cluster:**
   ```bash
   make install
   ```

3. **Run the operator locally:**
   ```bash
   make run
   ```

4. **Test with kind (optional):**
   ```bash
   ./hack/test-in-kind.sh
   ```

## Development Workflow

### Creating a Branch

Always create a new branch for your work:

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bug-fix
```

Branch naming conventions:
- `feature/` - New features
- `fix/` - Bug fixes
- `docs/` - Documentation changes
- `refactor/` - Code refactoring
- `test/` - Adding or updating tests

### Making Changes

1. **Keep commits atomic**: Each commit should represent a single logical change
2. **Write descriptive commit messages**:
   ```
   Short summary (50 chars or less)
   
   More detailed explanation if needed. Wrap at 72 characters.
   Explain what and why, not how.
   
   Fixes #123
   ```

3. **Update documentation**: If your changes affect user-facing functionality, update relevant documentation

4. **Add tests**: All new features and bug fixes should include tests

### Syncing with Upstream

Keep your fork up to date:

```bash
git fetch upstream
git checkout main
git merge upstream/main
git push origin main
```

## Coding Standards

### Go Code Style

- Follow standard Go formatting (use `gofmt` and `goimports`)
- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and concise

### Code Organization

- Place new types in `api/v1alpha1/`
- Place controller logic in `internal/controller/`
- Use the existing project structure
- Follow Kubebuilder conventions

### Running Linters

```bash
# Run golangci-lint
golangci-lint run

# Format code
go fmt ./...

# Vet code
go vet ./...
```

### Code Review Checklist

Before submitting, ensure your code:
- [ ] Follows Go idioms and best practices
- [ ] Has appropriate error handling
- [ ] Includes logging for important operations
- [ ] Has proper RBAC markers if adding new permissions
- [ ] Updates CRD markers if changing API types
- [ ] Includes unit tests
- [ ] Passes all existing tests
- [ ] Has updated documentation

## Testing

### Running Unit Tests

```bash
# Run all unit tests
make test

# Run with coverage
make test ARGS="-coverprofile=coverage.out"
go tool cover -html=coverage.out
```

### Running E2E Tests

```bash
# Using kind
make test-e2e

# Or manually with kind
kind create cluster --name moodle-operator-test
make install
make deploy IMG=controller:latest
go test ./test/e2e/... -v
```

### Writing Tests

- Place unit tests alongside the code they test (e.g., `controller_test.go`)
- Use the Ginkgo/Gomega testing framework (already configured)
- Mock external dependencies
- Test both success and failure scenarios
- Aim for >80% code coverage

Example test structure:

```go
var _ = Describe("MoodleTenant Controller", func() {
    Context("When creating a MoodleTenant", func() {
        It("Should create a namespace", func() {
            // Test implementation
        })
        
        It("Should handle errors gracefully", func() {
            // Test implementation
        })
    })
})
```

## Pull Request Process

### Before Submitting

1. **Rebase on latest main:**
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all checks:**
   ```bash
   make test
   make manifests
   make generate
   ```

3. **Verify no uncommitted changes:**
   ```bash
   git status
   ```

### Submitting a Pull Request

1. **Push your branch:**
   ```bash
   git push origin feature/your-feature-name
   ```

2. **Create a pull request** on GitHub with:
   - **Clear title** describing the change
   - **Description** explaining:
     - What changes were made
     - Why the changes were needed
     - How to test the changes
   - **Link to related issues** using "Fixes #123" or "Relates to #456"
   - **Screenshots** if applicable

3. **PR template:**
   ```markdown
   ## Description
   Brief description of changes
   
   ## Type of Change
   - [ ] Bug fix
   - [ ] New feature
   - [ ] Breaking change
   - [ ] Documentation update
   
   ## Testing
   How has this been tested?
   
   ## Checklist
   - [ ] Tests added/updated
   - [ ] Documentation updated
   - [ ] Code follows project style
   - [ ] All tests passing
   ```

### Review Process

- Maintainers will review your PR
- Address feedback in new commits (don't force push during review)
- Once approved, maintainers will merge your PR
- After merge, you can delete your branch

## Issue Guidelines

### Reporting Bugs

Use the bug report template and include:
- **Clear title**: Brief description of the bug
- **Environment**: Kubernetes version, operator version, etc.
- **Steps to reproduce**: Detailed steps to reproduce the issue
- **Expected behavior**: What should happen
- **Actual behavior**: What actually happens
- **Logs**: Relevant operator logs and kubectl output
- **MoodleTenant YAML**: If applicable

### Requesting Features

Use the feature request template and include:
- **Clear title**: Brief description of the feature
- **Problem**: What problem does this solve?
- **Proposed solution**: How should this work?
- **Alternatives**: What other solutions have you considered?
- **Additional context**: Screenshots, examples, etc.

### Asking Questions

- Check existing issues and documentation first
- Use discussions for questions rather than issues
- Provide context and what you've already tried

## Development Tips

### Debugging

```bash
# Run with verbose logging
make run ARGS="-zap-log-level=debug"

# Debug a specific namespace
kubectl logs -n moodle-lms-operator-system deployment/moodle-lms-operator-controller-manager -f

# Describe resources
kubectl describe moodletenant <name>
```

### Useful Make Targets

```bash
make help              # Show all available targets
make manifests         # Generate CRD manifests
make generate          # Generate code
make fmt               # Format code
make vet               # Vet code
make test              # Run tests
make build             # Build manager binary
make docker-build      # Build docker image
make deploy            # Deploy to cluster
```

### Working with CRDs

After modifying types in `api/v1alpha1/`:

```bash
# Regenerate CRDs and code
make manifests generate

# Reinstall CRDs
make install
```

## Getting Help

- **Documentation**: Check the [README](README.md) and [ARCHITECTURE](ARCHITECTURE.md)
- **Discussions**: Use GitHub Discussions for questions
- **Issues**: Search existing issues or create a new one
- **Slack**: Join the Belarusian State University Kubernetes channel (if applicable)

## Recognition

Contributors will be recognized in:
- Release notes for significant contributions
- The project's contributor list on GitHub

Thank you for contributing to the Moodle LMS Operator! 🎉
