# CloudFoundry Provider Upgrade Tests

This directory contains upgrade tests for the CloudFoundry Crossplane provider using the [xp-testing](https://github.com/crossplane-contrib/xp-testing) framework.

## Overview

Upgrade tests verify that resources created with one version of the provider continue to function correctly after upgrading to a newer version. This ensures backward compatibility and prevents regressions during provider updates.

## Test Flow

1. **Setup** - Create kind cluster with Crossplane and install provider (FROM version)
2. **Import Resources** - Create test resources from YAML manifests
3. **Verify Before Upgrade** - Ensure all resources are Ready
4. **Upgrade** - Update provider to newer version (TO version)
5. **Verify After Upgrade** - Confirm resources still work correctly
6. **Cleanup** - Delete resources and destroy cluster

## Prerequisites

### Required Tools

- **Go** - For running tests
- **Docker** - For kind cluster creation
- **kubectl** - For Kubernetes cluster interaction
- **kind** - Automatically installed by test framework

### CloudFoundry Access

You need valid CloudFoundry credentials with appropriate permissions:
- Read access to CF organizations and spaces
- Ability to create/delete spaces (for non-observe tests)
- Access to CF API endpoint

## Quick Start

### 1. Set Environment Variables

```bash
# CloudFoundry credentials
export CF_EMAIL="your-email@sap.com"
export CF_USERNAME="your-cf-username"
export CF_PASSWORD="your-cf-password"
export CF_ENDPOINT="https://api.cf.eu12.hana.ondemand.com"

# Upgrade test versions
export UPGRADE_TEST_FROM_TAG="v0.3.0"  # Version to upgrade FROM
export UPGRADE_TEST_TO_TAG="v0.3.2"    # Version to upgrade TO
```

### 2. Run the Tests

```bash
cd test/upgrade
go test -v -tags=upgrade -timeout=45m ./...
```

## Environment Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `CF_EMAIL` | Yes | Email for CF authentication | `user@sap.com` |
| `CF_USERNAME` | Yes | CF username | `your-username` |
| `CF_PASSWORD` | Yes | CF password | `your-password` |
| `CF_ENDPOINT` | Yes | CF API endpoint URL | `https://api.cf.eu12.hana.ondemand.com` |
| `UPGRADE_TEST_FROM_TAG` | Yes | Provider version to upgrade from | `v0.3.0` |
| `UPGRADE_TEST_TO_TAG` | Yes | Provider version to upgrade to | `v0.3.2` or `main` |

## Test Resources

Tests use YAML manifests from `test/e2e/crs/`. Currently tested resources:

- **Space** - Lightweight resource for testing basic upgrade flow

### Adding New Test Resources

1. Create a directory under `test/upgrade/crs/`:
   ```bash
   mkdir -p test/upgrade/crs/myresource
   ```

2. Add YAML manifest(s):
   ```bash
   cat > test/upgrade/crs/myresource/myresource.yaml <<EOF
   apiVersion: cloudfoundry.crossplane.io/v1alpha1
   kind: MyResource
   metadata:
     name: test-myresource
   spec:
     forProvider:
       # ... resource configuration
     providerConfigRef:
       name: default
   EOF
   ```

3. Run tests - new resources are automatically discovered

### Resource Selection Tips

**Suggested resources for testing:**
- ✅ Resources you can create/delete with your credentials
- ✅ Resources with minimal dependencies
- ✅ Resources using `managementPolicies: [Observe]` (safest - no creation)


## Test Structure

```
test/
├── upgrade/
|    └── crs/   
|        └── space.yaml         # Resources to test
│   ├── main_test.go            # Test environment setup
│   ├── upgrade_test.go         # Actual upgrade test logic
│   └── README.md               # This file
├── e2e/
│   └── crs/                    # Test resource manifests
│       └── space/
│           └── space.yaml
└── test_utils.go               # Helper functions
```

## Configuration

### Changing Test Resources

Edit `main_test.go` to change the resource directory:

```go
const (
    resourceDirectoryRoot = "../upgrade/crs/" 
)
```

### Adjusting Timeouts

In `upgrade_test.go`, modify timeout values:

```go
.Assess(
    "Verify resources before upgrade",
    upgrade.VerifyResources(upgradeTest.ResourceDirectories, time.Minute*30),  // 30 min timeout
)
```

### Crossplane Version

Change Crossplane version in `main_test.go`:

```go
CrossplaneSetup: setup.CrossplaneSetup{
    Version:  "1.20.1",  // Change this
    Registry: setup.DockerRegistry,
}
```

## Troubleshooting

### Common Issues

#### Error: "no non-test Go files"
**Cause:** Missing build tag when compiling

**Solution:** Always use `-tags=upgrade`:
```bash
go test -tags=upgrade ./...
```

#### Error: "CF-NotAuthorized"
**Cause:** Credentials lack permission for the operation

**Solution:** 
- Use resources you have permission for
- Use `managementPolicies: [Observe]` to observe existing resources
- Contact CF admin for necessary permissions

#### Error: "external resource does not exist"
**Cause:** Observing a resource that doesn't exist in CF

**Solution:** 
- Verify resource exists: `cf spaces`, `cf orgs`, etc.
- Use correct names in your YAML manifests
- Create the resource first if testing creation

#### Error: "cannot resolve references"
**Cause:** Resource references another resource that doesn't exist

**Solution:**
- Create referenced resources first (e.g., Organization before Domain)
- Or remove the reference and use direct names

#### Timeout waiting for resources
**Cause:** Resources not becoming Ready within timeout

**Debug:**
```bash
# Check provider logs
kubectl logs -n crossplane-system deployment/provider-cloudfoundry-<hash>

# Check resource status
kubectl get managed -A
kubectl describe <resource-type> <resource-name>
```

### Cleanup

Tests automatically cleanup, but if clusters are orphaned:

```bash
# List all kind clusters
kind get clusters

# Delete specific cluster
kind delete cluster --name e2e-<hash>

# Delete all e2e clusters
kind get clusters | grep e2e | xargs -n1 kind delete cluster --name
```

## Development

### Running Tests Locally

```bash
# Test with specific versions
export UPGRADE_TEST_FROM_TAG="v0.3.0"
export UPGRADE_TEST_TO_TAG="main"  # Test unreleased code

cd test/upgrade
go test -v -tags=upgrade -timeout=45m ./...
```

### Testing Unreleased Changes

To test the current codebase:

```bash
# Build local images
make build

# Set TO version to use local build
export UPGRADE_TEST_TO_TAG="main"

# Run tests
cd test/upgrade
go test -v -tags=upgrade -timeout=45m ./...
```

## Related Documentation

- [xp-testing Framework](https://github.com/crossplane-contrib/xp-testing)
- [Crossplane Documentation](https://docs.crossplane.io)
- [CloudFoundry Provider](https://github.com/SAP/crossplane-provider-cloudfoundry)
- [BTP Provider Upgrade Tests](https://github.com/SAP/crossplane-provider-btp/tree/main/test/upgrade) (reference implementation)

## Support

For issues or questions:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review test logs in `test/upgrade/logs/`
3. Open an issue in the repository
4. Contact the team on Slack

