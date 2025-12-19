# CloudFoundry Provider Upgrade Tests

This directory contains upgrade tests for the CloudFoundry Crossplane provider using the [xp-testing](https://github.com/crossplane-contrib/xp-testing) framework.

## Overview

Upgrade tests verify that resources created with one version of the provider continue to function correctly after upgrading to a newer version. This ensures backward compatibility and prevents regressions during provider updates.

## Test Flow

1. **Setup** - Create kind cluster with Crossplane and install provider (FROM version)
2. **Import Resources** - Create test resources from YAML manifests stored in directory `test/upgrade/testdata/baseCrs`
3. **Verify Before Upgrade** - Ensure all resources are Ready
4. **Upgrade** - Update provider to newer version (TO version)
5. **Verify After Upgrade** - Confirm resources still work correctly
6. **Cleanup** - Delete resources and destroy cluster

## Prerequisites

### Required Tools

- **Go 1.23+** - For running tests
- **Docker** - For kind cluster creation
- **kubectl** - For Kubernetes cluster interaction
- **kind** - Automatically installed by test framework

### CloudFoundry Access

You need valid CloudFoundry credentials with appropriate permissions:
- Read access to CF organizations and spaces
- Ability to create/delete spaces (for non-observe tests)
- Access to CF API endpoint

## Quick Start

### ⚠️ IMPORTANT: Configure Your CF Organization First

Before running any tests, you **must** update the organization name to one you have access to:

1. **List your available CF organizations:**
```bash
   cf login
   cf orgs
```

2. **Update the organization name** in `test/upgrade/testdata/baseCrs/import.yaml`:
```yaml
   apiVersion: cloudfoundry.crossplane.io/v1alpha1
   kind: Organization
   metadata:
     name: upgrade-test-org
   spec:
     managementPolicies:
       - Observe
     forProvider:
       name: your-org-name-here  # ← Change this to your CF org name
```

### 1. Set Environment Variables
```bash
# CloudFoundry credentials
export CF_EMAIL="your_email"
export CF_USERNAME="your-cf-username"
export CF_PASSWORD="your-cf-password"
export CF_ENDPOINT="https://api.cf.eu12.hana.ondemand.com"

# Upgrade test versions
export UPGRADE_TEST_FROM_TAG="v0.3.0"  # Version to upgrade FROM
export UPGRADE_TEST_TO_TAG="v0.3.2"    # Version to upgrade TO
```

### 2. Run the Tests

From the project root directory:
```bash
# Basic usage - test upgrade between two released versions
make test-upgrade
```

**Test your local changes:**
```bash
# Build local provider image first
make build

# Test upgrade FROM a release TO your local changes
export UPGRADE_TEST_FROM_TAG="v0.3.2"
export UPGRADE_TEST_TO_TAG="main"  # Uses your current code
make test-upgrade
```

### 3. Customize (Optional)

Override defaults as needed:
```bash

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
| `CF_EMAIL` | Email for CF authentication | `your_email` |
| `CF_USERNAME` | CF username | `your-username` |
| `CF_PASSWORD` | CF password | `your-password` |
| `CF_ENDPOINT` | CF API endpoint URL | `https://api.cf.eu12.hana.ondemand.com` |
| `UPGRADE_TEST_FROM_TAG` | Provider version to upgrade from | `v0.3.0` |
| `UPGRADE_TEST_TO_TAG` | Provider version to upgrade to | `v0.3.2` or `main` |

### Optional Variables (with defaults)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `UPGRADE_TEST_VERIFY_TIMEOUT` | Timeout for resource verification (minutes) | `30` | `45` |
| `UPGRADE_TEST_WAIT_FOR_PAUSE` | Wait time after provider upgrade (minutes) | `1` | `2` |

## Test Resources

Tests use YAML manifests from `test/upgrade/testdata/baseCrs/`. Currently tested resources:

- **Organization** (import) - Uses `managementPolicies: [Observe]` to import existing org
- **Space** - Lightweight resource for testing basic upgrade flow

### Adding New Test Resources

1. Create your resource manifest in `test/upgrade/testdata/baseCrs/`:
```bash
   cat > test/upgrade/testdata/baseCrs/myresource.yaml <<EOF
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

2. Run tests - new resources are automatically discovered

### Resource Selection Tips

**Suggested resources for testing:**
- ✅ Resources you can create/delete with your credentials
- ✅ Resources with minimal dependencies
- ✅ Resources using `managementPolicies: [Observe]` (safest - no creation)

## Test Structure
```
test/
├── upgrade/
|.  ├── crs/    
│   ├── testdata/
|   |   └── baseCrs/             # Test resource manifests
|   |      ├── import.yaml       # Organization (observe)
|   │      └── space.yaml        # Space (create)
│   ├── main_test.go          # Test environment setup
│   ├── upgrade_test.go       # Actual upgrade test logic
│   └── README.md             # This file
├── e2e/
│   └── crs/                  # E2E resource manifests
│       └── orgspace/
│           ├── import.yaml
│           └── space.yaml
└── test_utils.go             # Helper functions
```

### Crossplane Version

Change Crossplane version in `main_test.go`:
```go
const crossplaneVersion = "CHOSEN_VERSION"
```

## Troubleshooting

### Common Issues

#### Error: "external resource does not exist"
**Cause:** The organization name in `import.yaml` doesn't exist or you don't have access

**Solution:** 
1. Run `cf orgs` to see your available organizations
2. Update `test/upgrade/testdata/baseCrs/import.yaml` with a valid org name
3. Ensure you have at least read access to the organization

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

#### Error: "cannot resolve references"
**Cause:** Resource references another resource that doesn't exist

**Solution:**
- Ensure referenced resources are created first
- Check that Organization exists and is imported before creating Space

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

## Related Documentation

- [xp-testing Framework](https://github.com/crossplane-contrib/xp-testing)
- [Crossplane Documentation](https://docs.crossplane.io)
- [CloudFoundry Provider](https://github.com/SAP/crossplane-provider-cloudfoundry)
- [BTP Provider Upgrade Tests](https://github.com/SAP/crossplane-provider-btp/tree/main/test/upgrade) (reference implementation)