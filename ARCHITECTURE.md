# Architecture Guide

This document describes the architecture, design patterns, and technical decisions behind the Moodle LMS Operator.

## Table of Contents

- [Overview](#overview)
- [Design Principles](#design-principles)
- [Architecture Diagram](#architecture-diagram)
- [Components](#components)
- [Kubernetes Patterns](#kubernetes-patterns)
- [Resource Management](#resource-management)
- [Security Architecture](#security-architecture)
- [High Availability](#high-availability)
- [Storage Strategy](#storage-strategy)
- [Networking](#networking)
- [Database Integration](#database-integration)
- [Future Enhancements](#future-enhancements)

## Overview

The Moodle LMS Operator is a Kubernetes-native solution built using the [Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) and [Kubebuilder](https://book.kubebuilder.io/) framework. It automates the deployment and lifecycle management of multi-tenant Moodle instances.

### Goals

- **Isolation**: Provide hard multi-tenancy with namespace-level isolation
- **Automation**: Eliminate manual deployment and configuration
- **Scalability**: Support horizontal scaling and high availability
- **Security**: Enforce security best practices by default
- **Maintainability**: Simplify Day 2 operations (upgrades, backups, monitoring)

## Design Principles

1. **Declarative Configuration**: Users declare desired state; the operator ensures actual state matches
2. **Reconciliation Loop**: Continuously monitor and reconcile resources
3. **Idempotency**: Operations can be safely repeated without side effects
4. **Fail-Safe**: Graceful degradation and proper error handling
5. **Cloud-Native**: Leverage Kubernetes primitives and patterns
6. **Single Responsibility**: Each component has a clear, focused purpose

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Kubernetes API                          │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                    ┌───────▼────────┐
                    │  MoodleTenant  │  (Custom Resource)
                    │    API (CRD)   │
                    └───────┬────────┘
                            │
                    ┌───────▼────────┐
                    │   Controller    │  (Reconciliation Loop)
                    │    Manager      │
                    └───────┬────────┘
                            │
        ┏━━━━━━━━━━━━━━━━━━━┻━━━━━━━━━━━━━━━━━━━┓
        ▼                                        ▼
┌───────────────┐                     ┌──────────────────┐
│   Namespace   │                     │   Finalizers     │
│   Creation    │                     │   & Cleanup      │
└───────┬───────┘                     └──────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────┐
│              Per-Tenant Namespace Resources              │
├─────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  Deployment  │  │   Service    │  │   Ingress    │  │
│  │              │  │  (ClusterIP) │  │    (TLS)     │  │
│  │  ┌────────┐  │  └──────────────┘  └──────────────┘  │
│  │  │ Init   │  │                                        │
│  │  │Container│ │  ┌──────────────┐  ┌──────────────┐  │
│  │  └────────┘  │  │     PVC      │  │     HPA      │  │
│  │  ┌────────┐  │  │   (CephFS)   │  │              │  │
│  │  │ Moodle │  │  └──────────────┘  └──────────────┘  │
│  │  │PHP-FPM │  │                                        │
│  │  └────────┘  │  ┌──────────────┐  ┌──────────────┐  │
│  │  ┌────────┐  │  │NetworkPolicy │  │     PDB      │  │
│  │  │Memcached│ │  │              │  │              │  │
│  │  └────────┘  │  └──────────────┘  └──────────────┘  │
│  └──────────────┘                                        │
│                     ┌──────────────┐                     │
│                     │   CronJob    │                     │
│                     │(Moodle Cron) │                     │
│                     └──────────────┘                     │
└─────────────────────────────────────────────────────────┘
```

## Components

### 1. MoodleTenant Custom Resource Definition (CRD)

The `MoodleTenant` CRD defines the desired state of a Moodle instance.

**Key fields:**
- `hostname`: DNS name for the Moodle instance
- `image`: Container image (typically PHP-FPM)
- `resources`: CPU and memory requests/limits
- `hpa`: Auto-scaling configuration
- `storage`: Persistent volume specifications
- `databaseRef`: Database connection details
- `phpSettings`: PHP runtime configuration
- `memcached`: Cache configuration

### 2. Controller

The controller implements the reconciliation loop using the controller-runtime library.

**Responsibilities:**
- Watch for MoodleTenant resource changes
- Create/update/delete child resources
- Maintain finalizers for cleanup
- Update resource status
- Handle errors and retries

**Reconciliation Logic:**

```go
func (r *MoodleTenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch MoodleTenant
    // 2. Handle deletion (finalizer)
    // 3. Ensure namespace exists
    // 4. Create/update all child resources:
    //    - PVC
    //    - Deployment (with init, main, sidecar)
    //    - Service
    //    - Ingress
    //    - NetworkPolicy
    //    - HPA
    //    - PDB
    //    - CronJob
    // 5. Update status
    // 6. Requeue if needed
}
```

### 3. Generated Resources

For each `MoodleTenant`, the controller creates:

| Resource | Purpose | Naming Pattern |
|----------|---------|----------------|
| Namespace | Isolation boundary | `moodle-{tenant-name}` |
| PVC | Persistent storage | `moodledata-{tenant-name}` |
| Deployment | Pod management | `moodle-{tenant-name}` |
| Service | Internal networking | `moodle-{tenant-name}` |
| Ingress | External access | `moodle-{tenant-name}` |
| NetworkPolicy | Traffic control | `moodle-{tenant-name}` |
| HPA | Auto-scaling | `moodle-{tenant-name}` |
| PDB | Disruption budget | `moodle-{tenant-name}` |
| CronJob | Maintenance tasks | `moodle-cron-{tenant-name}` |

## Kubernetes Patterns

### 1. Namespace-per-Tenant Pattern

**Pattern**: Hard multi-tenancy through namespace isolation

**Benefits:**
- Strong security boundaries
- Resource quotas per tenant
- RBAC isolation
- Network policy enforcement

**Implementation:**
```go
namespace := &corev1.Namespace{
    ObjectMeta: metav1.ObjectMeta{
        Name: fmt.Sprintf("moodle-%s", moodleTenant.Name),
        Labels: map[string]string{
            "app.kubernetes.io/name": "moodle",
            "app.kubernetes.io/instance": moodleTenant.Name,
            "app.kubernetes.io/managed-by": "moodle-lms-operator",
        },
    },
}
```

### 2. Init Container Pattern

**Pattern**: Prepare the environment before the main application starts

**Use Case**: Set proper file permissions on CephFS volumes

**Implementation:**
```go
InitContainers: []corev1.Container{{
    Name:  "volume-prep",
    Image: "busybox:latest",
    Command: []string{"sh", "-c", "chown -R 33:33 /moodledata && chmod -R 755 /moodledata"},
    SecurityContext: &corev1.SecurityContext{
        RunAsUser: pointer.Int64(0), // Must run as root
    },
    VolumeMounts: []corev1.VolumeMount{{
        Name:      "moodledata",
        MountPath: "/moodledata",
    }},
}}
```

### 3. Sidecar Pattern

**Pattern**: Extend main container functionality with a helper container

**Use Case**: Provide local memcached instance for Moodle caching

**Implementation:**
```go
Containers: []corev1.Container{
    // Main Moodle container
    {
        Name: "moodle-php",
        // ...
    },
    // Sidecar memcached
    {
        Name:  "memcached",
        Image: "memcached:1.6-alpine",
        Args:  []string{"-m", fmt.Sprintf("%d", memcachedMemoryMB)},
        Ports: []corev1.ContainerPort{{
            ContainerPort: 11211,
            Name:          "memcached",
        }},
    },
}
```

### 4. Operator Pattern

**Pattern**: Extend Kubernetes API with custom controllers

**Benefits:**
- Domain-specific automation
- Declarative management
- Self-healing systems
- Reduced operational burden

## Resource Management

### Owner References

All child resources have owner references to the parent `MoodleTenant`:

```go
ctrl.SetControllerReference(moodleTenant, resource, r.Scheme)
```

**Benefits:**
- Automatic garbage collection
- Clear resource hierarchy
- Simplified cleanup

### Finalizers

Finalizers ensure proper cleanup before resource deletion:

```go
const finalizerName = "moodle.bsu.by/finalizer"

// Add finalizer on creation
if !controllerutil.ContainsFinalizer(moodleTenant, finalizerName) {
    controllerutil.AddFinalizer(moodleTenant, finalizerName)
}

// Handle deletion
if !moodleTenant.DeletionTimestamp.IsZero() {
    // Perform cleanup (database, external resources)
    // Remove finalizer
    controllerutil.RemoveFinalizer(moodleTenant, finalizerName)
}
```

## Security Architecture

### Principle of Least Privilege

1. **Non-root containers**: All containers run as unprivileged users
   ```go
   SecurityContext: &corev1.SecurityContext{
       RunAsNonRoot: pointer.Bool(true),
       RunAsUser:    pointer.Int64(33), // www-data
   }
   ```

2. **Read-only filesystem**: Where possible
3. **Drop capabilities**: Remove unnecessary Linux capabilities

### Network Policies

**Default deny** ingress and egress:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
spec:
  policyTypes:
  - Ingress
  - Egress
  # Explicit allow rules only
```

**Allowed traffic:**
- Ingress from Ingress controller
- Egress to database
- Egress to external services (HTTP/HTTPS)
- Pod-to-pod within namespace (Moodle ↔ Memcached)

### Secret Management

- Database credentials stored in Kubernetes Secrets
- Never log sensitive information
- Use references, not embedded secrets

## High Availability

### Horizontal Pod Autoscaler (HPA)

Automatically scale based on CPU utilization:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
spec:
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 75
```

### PodDisruptionBudget (PDB)

Ensure availability during disruptions:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
spec:
  minAvailable: 1
```

### Topology Spread Constraints

Distribute pods across nodes and zones:

```yaml
topologySpreadConstraints:
- maxSkew: 1
  topologyKey: kubernetes.io/hostname
  whenUnsatisfiable: ScheduleAnyway
- maxSkew: 1
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: ScheduleAnyway
```

## Storage Strategy

### CephFS for Shared Storage

**Why CephFS:**
- ReadWriteMany (RWX) support for multi-pod access
- High performance distributed filesystem
- Native Kubernetes CSI driver

**Volume Structure:**
```
/moodledata/
├── filedir/        # Moodle file storage
├── localcache/     # Local cache files
├── sessions/       # PHP sessions
└── temp/           # Temporary files
```

**PVC Configuration:**
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
spec:
  accessModes:
  - ReadWriteMany
  storageClassName: csi-cephfs-sc
  resources:
    requests:
      storage: 500Gi
```

## Networking

### Service

ClusterIP service for internal access:

```yaml
apiVersion: v1
kind: Service
spec:
  type: ClusterIP
  ports:
  - port: 9000
    targetPort: 9000
    name: php-fpm
```

### Ingress

NGINX Ingress with FastCGI:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "FCGI"
    nginx.ingress.kubernetes.io/fastcgi-index: "index.php"
    nginx.ingress.kubernetes.io/fastcgi-params-configmap: "moodle-fastcgi-params"
spec:
  tls:
  - hosts:
    - biology.bsu.by
    secretName: biology-tls
```

## Database Integration

### Database Reference

The operator expects an external PostgreSQL database:

```yaml
databaseRef:
  host: postgres-cluster.db-tier.svc
  name: biology_moodle
  user: biology_user
  password: secure-password
  adminSecret: postgres-admin  # For initialization
```

### Initialization

On first deployment:
1. Connect using admin credentials
2. Create database if not exists
3. Create user if not exists
4. Grant permissions
5. Moodle handles schema creation

## Future Enhancements

### Planned Features

1. **Backup & Restore**
   - Automated database backups (via CronJob)
   - S3/object storage integration
   - Point-in-time recovery

2. **Observability**
   - Prometheus metrics
   - Custom metrics (active users, course count)
   - Grafana dashboards
   - Structured logging

3. **Advanced Scheduling**
   - Node affinity for database locality
   - Pod priority classes
   - Custom scheduler integration

4. **Multi-Database Support**
   - MySQL/MariaDB support
   - Database cluster integration (Patroni, CloudNativePG)

5. **Webhooks**
   - Validation webhook for spec validation
   - Mutating webhook for defaults
   - Conversion webhook for API versioning

6. **Status Conditions**
   - Ready condition
   - Progressing condition
   - Error conditions
   - Observed generation

7. **Plugin Management**
   - Automated plugin installation
   - Version management
   - Security updates

### Technical Debt

- Add comprehensive E2E tests
- Improve error messages and events
- Add status subresource updates
- Implement resource pruning for unused resources
- Add admission webhooks for validation

## References

- [Kubernetes Patterns, 2nd Edition](https://www.oreilly.com/library/view/kubernetes-patterns-2nd/9781098131678/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [Operator SDK](https://sdk.operatorframework.io/)
- [Programming Kubernetes](https://www.oreilly.com/library/view/programming-kubernetes/9781492047094/)
- [Moodle Documentation](https://docs.moodle.org/)

## Questions?

For questions about the architecture, please:
- Open a [GitHub Discussion](https://github.com/pkasila/moodle-lms-operator/discussions)
- Review the [code documentation](https://pkg.go.dev/bsu.by/moodle-lms-operator)
- Check the [Contributing Guide](CONTRIBUTING.md)
