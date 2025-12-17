# CI/CD Pipeline Documentation

## Overview

This repository includes GitHub Actions workflows for continuous integration and deployment.

## Workflows

### 1. CI Pipeline (`.github/workflows/ci.yml`)

**Triggers:**
- Push to `main`, `develop`, or `master` branches
- Pull requests to `main`, `develop`, or `master`
- Manual trigger via `workflow_dispatch`

**Jobs:**

#### Lint & Format Check
- Runs `gofmt` to check code formatting
- Runs `go vet` for static analysis
- Runs `staticcheck` (if available)

#### Unit Tests
- Runs unit tests with race detector
- Generates coverage report
- Uploads coverage to Codecov (optional)

#### Integration Tests
- Spins up PostgreSQL, Redis, and RabbitMQ using GitHub Actions services
- Runs integration tests
- Generates coverage report

#### Build
- Builds application for multiple platforms:
  - Linux (amd64)
  - Windows (amd64)
  - macOS (amd64, arm64)
- Uploads build artifacts

#### Security Scan
- Runs Gosec security scanner
- Generates security report
- Uploads report as artifact

#### Docker Build
- Builds Docker image (if Dockerfile exists)
- Pushes to registry on main branch
- Uses build cache for faster builds

#### Coverage Report
- Generates HTML coverage report
- Uploads as artifact
- Comments on PRs with coverage (optional)

### 2. Release Workflow (`.github/workflows/release.yml`)

**Triggers:**
- GitHub release creation
- Manual trigger with version input

**Actions:**
- Runs full test suite
- Builds binaries for all platforms
- Creates checksums
- Creates GitHub release with binaries

## Setup Instructions

### 1. Required Secrets

Add these secrets to your GitHub repository (Settings → Secrets and variables → Actions):

**For Docker Registry (optional):**
- `DOCKER_USERNAME` - Docker Hub username
- `DOCKER_PASSWORD` - Docker Hub password or access token
- `DOCKER_REGISTRY` - Registry URL (e.g., `docker.io/username` or `ghcr.io/username/repo`)

**For Codecov (optional):**
- `CODECOV_TOKEN` - Codecov upload token

### 2. Environment Variables

The workflows use GitHub Actions services for integration tests. No additional setup needed.

### 3. Customization

**To skip Docker build:**
- Remove or comment out the `docker-build` job in `ci.yml`

**To add more platforms:**
- Add more `GOOS`/`GOARCH` combinations in the build job

**To use different test commands:**
- Modify the test steps to use your Makefile targets:
  ```yaml
  run: make test-unit
  ```

## Local Testing

Test the CI pipeline locally using [act](https://github.com/nektos/act):

```bash
# Install act
brew install act  # macOS
# or download from https://github.com/nektos/act/releases

# Run lint job
act -j lint

# Run unit tests
act -j test-unit

# Run full pipeline
act
```

## Workflow Status Badge

Add this to your README.md:

```markdown
![CI](https://github.com/your-username/your-repo/workflows/CI/badge.svg)
```

## Troubleshooting

### Integration tests fail
- Ensure Docker is available in GitHub Actions (it is by default)
- Check service health checks are passing
- Verify environment variables are set correctly

### Build fails
- Check Go version matches `go.mod`
- Verify all dependencies are in `go.sum`
- Run `go mod tidy` locally

### Docker build fails
- Ensure Dockerfile exists
- Check Docker secrets are set
- Verify registry permissions

