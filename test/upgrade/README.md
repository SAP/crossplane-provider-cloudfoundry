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

- **Go+** - For running tests
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

### 3. Customize (Optional)

Override defaults as needed:

```bash
# Test with custom resource directory
export UPGRADE_TEST_CRS_PATH="../e2e/crs-minimal/"

# Increase timeout for slow resources
export UPGRADE_TEST_VERIFY_TIMEOUT="45"  # minutes

# Increase wait time after upgrade
export UPGRADE_TEST_WAIT_FOR_PAUSE="2"  # minutes

# Then run tests
cd test/upgrade
go test -v -tags=upgrade -timeout=60m ./...
```

## Environment Variables

### Required Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `CF_EMAIL` | Email for CF authentication | `user@sap.com` |
| `CF_USERNAME` | CF username | `your-username` |
| `CF_PASSWORD` | CF password | `your-password` |
| `CF_ENDPOINT` | CF API endpoint URL | `https://api.cf.eu12.hana.ondemand.com` |
| `UPGRADE_TEST_FROM_TAG` | Provider version to upgrade from | `v0.3.0` |
| `UPGRADE_TEST_TO_TAG` | Provider version to upgrade to | `v0.3.2` or `main` |

### Optional Variables (with defaults)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `UPGRADE_TEST_CRS_PATH` | Path to test resources directory | `../e2e/crs/` | `../e2e/crs-minimal/` |
| `UPGRADE_TEST_VERIFY_TIMEOUT` | Timeout for resource verification (minutes) | `30` | `45` |
| `UPGRADE_TEST_WAIT_FOR_PAUSE` | Wait time after provider upgrade (minutes) | `1` | `2` |

## Test Resources

Tests use YAML manifests from `test/e2e/crs/`. Currently tested resources:

- **Space** - Lightweight resource for testing basic upgrade flow

### Adding New Test Resources

1. Create a directory under `test/e2e/crs/`:
   ```bash
   mkdir -p test/e2e/crs/myresource
   ```

2. Add YAML manifest(s):
   ```bash
   cat > test/e2e/crs/myresource/myresource.yaml <<EOF
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

**Suggested resources for Initial testing:**
- ✅ Resources you can create/delete with your credentials
- ✅ Resources with minimal dependencies
- ✅ Resources using `managementPolicies: [Observe]` (safest - no creation)


## Test Structure

```
test/
├── upgrade/
|   └── crs/                 # Test resource manifests
│       └── space.yaml
│       └── org.yaml 
│   ├── main_test.go          # Test environment setup
│   ├── upgrade_test.go       # Actual upgrade test logic
│   └── README.md            # This file
├── e2e/
│   └── crs/                 # E2E resource manifests
│       └── orgspace/
│           └── space.yaml
└── test_utils.go            # Helper functions
```


### Crossplane Version

Change Crossplane version in `main_test.go`:
```go
const crossplaneVersion=CHOSEN_VERSION
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

### ⚠️ Important: Update Organization Name

Before running tests, edit `test/upgrade/crs/import.yaml` and change the organization name to one you have access to:

```bash
cf orgs  # List your available orgs
```

Then update `test/upgrade/crs/import.yaml`:
```yaml
forProvider:
  name: your-org-name-here  # ← Change this
```  

## Performance
- TO FILL IN


## Related Documentation

- [xp-testing Framework](https://github.com/crossplane-contrib/xp-testing)
- [Crossplane Documentation](https://docs.crossplane.io)
- [CloudFoundry Provider](https://github.com/SAP/crossplane-provider-cloudfoundry)
- [BTP Provider Upgrade Tests](https://github.com/SAP/crossplane-provider-btp/tree/main/test/upgrade) (reference implementation)
