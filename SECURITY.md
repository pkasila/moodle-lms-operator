# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the following versions:

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| < 1.0   | :x:                |

**Note:** This project is in active development. Version support may change as the project evolves.

## Reporting a Vulnerability

We take the security of the Moodle LMS Operator seriously. If you have discovered a security vulnerability, please report it to us responsibly.

### Where to Report

**Please DO NOT report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to:

- **Primary Contact**: pavel.kasila@gmail.com
- **Subject Line**: `[SECURITY] Moodle LMS Operator Vulnerability Report`

### What to Include

Please include the following information in your report:

1. **Description**: A clear description of the vulnerability
2. **Impact**: The potential impact and severity
3. **Steps to Reproduce**: Detailed steps to reproduce the vulnerability
4. **Affected Versions**: Which versions are affected
5. **Proof of Concept**: If possible, include a PoC (code, configuration, etc.)
6. **Suggested Fix**: If you have recommendations for fixing the issue
7. **Your Contact Information**: How we can reach you for follow-up

### Example Report Template

```
Subject: [SECURITY] Privilege Escalation in MoodleTenant Controller

Description:
A vulnerability exists in the MoodleTenant controller that allows...

Impact:
This could allow an attacker to...

Affected Versions:
- main branch (commit abc123)
- All versions prior to x.y.z

Steps to Reproduce:
1. Create a MoodleTenant with the following spec...
2. Observe that...
3. An attacker could then...

Proof of Concept:
[Attach YAML files, scripts, or screenshots]

Suggested Fix:
Adding validation to ensure...

Contact:
Your Name <your.email@example.com>
```

## Response Timeline

We aim to respond to security reports according to the following timeline:

- **Initial Response**: Within 48 hours
- **Triage**: Within 1 week
- **Fix Development**: Depends on severity (see below)
- **Public Disclosure**: After patch is available

### Severity Levels

| Severity | Response Time | Examples |
|----------|---------------|----------|
| **Critical** | 24-48 hours | Remote code execution, privilege escalation to cluster admin |
| **High** | 1 week | Authentication bypass, privilege escalation within namespace |
| **Medium** | 2 weeks | Information disclosure, denial of service |
| **Low** | 1 month | Minor issues with limited impact |

## Security Best Practices

### For Operators (Kubernetes Administrators)

1. **RBAC Configuration**
   - Follow the principle of least privilege
   - Review and restrict the operator's RBAC permissions
   - Use separate namespaces for operator and tenants

2. **Network Policies**
   - Enable network policies in your cluster
   - Review and customize the default network policies
   - Restrict egress to only necessary endpoints

3. **Image Security**
   - Use trusted container registries
   - Scan images for vulnerabilities
   - Keep images up to date
   - Verify image signatures

4. **Secret Management**
   - Use Kubernetes Secrets or external secret managers
   - Enable encryption at rest for Secrets
   - Rotate credentials regularly
   - Never commit secrets to version control

5. **Audit Logging**
   - Enable Kubernetes audit logging
   - Monitor operator logs for suspicious activity
   - Set up alerts for security events

6. **Resource Limits**
   - Set appropriate resource quotas per namespace
   - Configure LimitRanges
   - Monitor resource usage

### For Developers (Contributors)

1. **Code Security**
   - Follow secure coding practices
   - Validate all user inputs
   - Use parameterized queries for database operations
   - Avoid hardcoded credentials

2. **Dependency Management**
   - Keep dependencies up to date
   - Review dependency security advisories
   - Use `go mod verify` to check integrity
   - Scan for known vulnerabilities

3. **Testing**
   - Write security-focused tests
   - Test RBAC configurations
   - Test with malicious inputs
   - Perform fuzzing when applicable

4. **Code Review**
   - All code must be reviewed before merge
   - Security-sensitive changes require extra scrutiny
   - Use static analysis tools

## Known Security Considerations

### Current Security Posture

1. **Namespace Isolation**
   - ✅ Each tenant gets a dedicated namespace
   - ✅ NetworkPolicies enforce isolation
   - ⚠️ Ensure your cluster supports NetworkPolicy

2. **Container Security**
   - ✅ Non-root containers (except init container)
   - ✅ Read-only root filesystem where possible
   - ⚠️ Init container requires root for permission setup

3. **Secret Management**
   - ✅ Database credentials stored in Secrets
   - ⚠️ Consider using external secret management (Vault, etc.)

4. **Network Security**
   - ✅ TLS for Ingress
   - ✅ NetworkPolicy with default-deny
   - ⚠️ Ensure certificates are properly managed

5. **RBAC**
   - ✅ Minimal required permissions
   - ⚠️ Operator needs cluster-wide namespace creation rights

### Limitations

1. **Database Security**
   - The operator does not encrypt database connections by default
   - Consider using TLS for database connections
   - Implement network policies to restrict database access

2. **Multi-Cluster**
   - This operator is designed for single-cluster deployments
   - Multi-cluster scenarios require additional security considerations

3. **Backup Security**
   - Backup functionality is not yet implemented
   - Backups may contain sensitive data and should be encrypted

## Security Updates

### How We Communicate Security Issues

1. **GitHub Security Advisories**: For published vulnerabilities
2. **Release Notes**: Security fixes are clearly marked
3. **Mailing List**: For critical security updates (planned)

### How to Stay Updated

- Watch this repository on GitHub
- Subscribe to release notifications
- Follow security advisories

## Compliance

### Data Protection

This operator processes the following sensitive data:

- **Database credentials**: Stored in Kubernetes Secrets
- **TLS certificates**: Managed via cert-manager or manual provisioning
- **Moodle user data**: Stored in PostgreSQL and PersistentVolumes

If you're subject to compliance requirements (GDPR, HIPAA, etc.):

- Ensure data encryption at rest
- Implement proper access controls
- Configure audit logging
- Review data retention policies
- Implement backup and disaster recovery

### Standards Compliance

We strive to follow:

- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [OWASP Kubernetes Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Kubernetes_Security_Cheat_Sheet.html)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/security-checklist/)

## Vulnerability Disclosure Policy

### Our Commitment

- We will acknowledge receipt of your report within 48 hours
- We will provide regular updates on our progress
- We will work with you to understand and resolve the issue
- We will credit you in the security advisory (unless you prefer to remain anonymous)

### Coordinated Disclosure

We practice coordinated disclosure:

1. **Private Discussion**: We discuss the issue privately with the reporter
2. **Fix Development**: We develop and test a fix
3. **Release**: We release a patched version
4. **Public Disclosure**: We publish a security advisory after users have had time to update

We ask that you:

- Give us reasonable time to fix the issue before public disclosure
- Do not exploit the vulnerability maliciously
- Do not access, modify, or delete data that doesn't belong to you

### Hall of Fame

We recognize security researchers who help us improve security:

- (No vulnerabilities reported yet)

## Security Tooling

### Recommended Tools

For developers and operators:

- **Static Analysis**: `gosec`, `golangci-lint`
- **Dependency Scanning**: `govulncheck`, Dependabot
- **Container Scanning**: Trivy, Clair, Snyk
- **Runtime Security**: Falco, KubeArmor
- **Policy Enforcement**: OPA/Gatekeeper, Kyverno

### CI/CD Security

Our CI/CD pipeline includes:

- Automated dependency updates (Dependabot)
- Container image scanning
- SAST (Static Application Security Testing)
- License compliance checking

## Contact

For security concerns:

- **Email**: pavel.kasila@gmail.com
- **PGP Key**: Available on request

For general questions:

- **GitHub Issues**: For non-security bugs and features
- **GitHub Discussions**: For questions and discussion

---

**Last Updated**: December 17, 2025

Thank you for helping keep the Moodle LMS Operator and its users safe!
