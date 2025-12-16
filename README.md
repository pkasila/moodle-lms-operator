# Moodle LMS Operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/pkasila/moodle-lms-operator)](https://goreportcard.com/report/github.com/pkasila/moodle-lms-operator)

A Kubernetes Operator for managing multi-tenant Moodle LMS deployments.

> **Disclaimer**: This project is not affiliated with, endorsed by, or supported by Moodle Pty Ltd. Moodle is a trademark of Moodle Pty Ltd.

## Overview

The Moodle LMS Operator automates the deployment and lifecycle management of isolated Moodle instances on Kubernetes. Built with [Kubebuilder](https://book.kubebuilder.io/), it implements cloud-native patterns to provide robust multi-tenancy and operational ease.

### Key Features

- 🏢 **Multi-Tenancy**: Dedicated namespace per tenant for hard isolation between departments
- 🔒 **Security First**: Non-root containers, NetworkPolicies, and secret management
- 📈 **Auto-Scaling**: Horizontal Pod Autoscaler with configurable CPU targets
- 🛡️ **High Availability**: PodDisruptionBudgets and topology spreading across zones
- 💾 **Persistent Storage**: CephFS RWX volumes for shared moodledata
- ⚡ **Performance**: Memcached sidecar for local caching
- 🔄 **Automated Maintenance**: CronJob for Moodle cron tasks
- 🌐 **Ingress Integration**: TLS-enabled Ingress with custom annotations

## Architecture

Each `MoodleTenant` custom resource creates a complete, isolated Moodle stack:

```
MoodleTenant CR
├── Namespace (tenant-specific)
├── Deployment (Init Container + Main + Sidecar)
│   ├── volume-prep (init): Sets CephFS permissions
│   ├── moodle-php (main): PHP-FPM running Moodle
│   └── memcached (sidecar): Local cache
├── Service (ClusterIP)
├── Ingress (TLS + FastCGI)
├── PersistentVolumeClaim (CephFS)
├── HorizontalPodAutoscaler
├── PodDisruptionBudget
├── NetworkPolicy
└── CronJob (Moodle maintenance)
```

For detailed architecture documentation, see [ARCHITECTURE.md](ARCHITECTURE.md).

## Prerequisites

- **Go**: v1.24.6 or later
- **Docker**: v17.03 or later
- **kubectl**: v1.11.3 or later
- **Kubernetes cluster**: v1.11.3 or later
- **Kubebuilder**: v4.x (for development)
- **CephFS StorageClass**: For persistent storage (configurable)

## Quick Start

### Installation

1. **Install the CRDs:**
   ```bash
   make install
   ```

2. **Deploy the operator:**
   ```bash
   make deploy IMG=ghcr.io/pkasila/moodle-lms-operator:latest
   ```

3. **Create a MoodleTenant:**
   ```bash
   kubectl apply -f config/samples/moodle_v1alpha1_moodletenant.yaml
   ```

### Creating a Moodle Tenant

Create a `MoodleTenant` resource to provision a new Moodle instance:

```yaml
apiVersion: moodle.bsu.by/v1alpha1
kind: MoodleTenant
metadata:
  name: biology-dept
spec:
  hostname: biology.bsu.by
  image: ghcr.io/pkasila/moodle:4.5-fpm
  
  resources:
    requests:
      cpu: "500m"
      memory: "1Gi"
    limits:
      cpu: "2000m"
      memory: "2Gi"
  
  hpa:
    enabled: true
    minReplicas: 2
    maxReplicas: 5
    targetCPU: 75
  
  storage:
    size: "500Gi"
    storageClass: "csi-cephfs-sc"
  
  databaseRef:
    host: "postgres-cluster.db-tier.svc"
    name: "biology_moodle"
    user: "biology_user"
    password: "secure-password"
    adminSecret: "postgres-admin"
  
  phpSettings:
    maxExecutionTime: 60
    memoryLimit: "512M"
  
  memcached:
    memoryMB: 128
```

The operator will create:
- A dedicated namespace `moodle-biology-dept`
- All required Kubernetes resources
- Database initialization (if needed)

### Accessing Your Moodle Instance

After deployment, access Moodle via the configured hostname:

```bash
kubectl get ingress -n moodle-biology-dept
```

Visit `https://biology.bsu.by` (or your configured hostname).

## Development

### Running Locally

```bash
# Install CRDs
make install

# Run the operator locally
make run
```

### Running Tests

```bash
# Unit tests
make test

# E2E tests (requires kind)
make test-e2e
```

### Building and Pushing Images

```bash
# Build operator image
make docker-build IMG=<your-registry>/moodle-lms-operator:tag

# Push to registry
make docker-push IMG=<your-registry>/moodle-lms-operator:tag

# Deploy with custom image
make deploy IMG=<your-registry>/moodle-lms-operator:tag
```

## Uninstallation

```bash
# Delete all MoodleTenant instances
kubectl delete moodletenants --all -A

# Uninstall operator
make undeploy

# Remove CRDs
make uninstall
```

## Configuration Reference

### MoodleTenant Spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `hostname` | string | Yes | Hostname for the Moodle instance |
| `image` | string | Yes | Container image for Moodle |
| `resources` | ResourceRequirements | No | CPU/Memory requests and limits |
| `hpa` | HPASpec | No | Horizontal Pod Autoscaler configuration |
| `storage` | StorageSpec | Yes | Persistent storage configuration |
| `databaseRef` | DatabaseRefSpec | Yes | Database connection details |
| `phpSettings` | PHPSettingsSpec | No | PHP runtime configuration |
| `memcached` | MemcachedSpec | No | Memcached sidecar configuration |

For complete API documentation, see the [API Reference](api/v1alpha1/moodletenant_types.go).

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on:
- Development setup
- Coding standards
- Pull request process
- Testing requirements

Run `make help` for all available make targets.

## Documentation

- [Architecture Guide](ARCHITECTURE.md) - Detailed system design and patterns
- [Security Policy](SECURITY.md) - Vulnerability reporting and security practices
- [Code of Conduct](CODE_OF_CONDUCT.md) - Community guidelines
- [Kubebuilder Documentation](https://book.kubebuilder.io/)

## License

Copyright 2025 Pavel Kasila.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

