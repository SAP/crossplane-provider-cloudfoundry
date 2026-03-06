# CloudFoundry Provider Upgrade Tests

This directory contains upgrade tests for the CloudFoundry Crossplane provider using the [xp-testing](https://github.com/crossplane-contrib/xp-testing) framework.

## Overview

Upgrade tests verify that resources created with one version of the provider continue to function correctly after upgrading to a newer version. This ensures backward compatibility and prevents regressions during provider updates.

### Test Types

We maintain two types of upgrade tests:

1. **Base Upgrade Tests** - Standard resource verification (TestUpgradeProvider)
   - Verifies resources remain Ready after upgrade
   - Tests basic CRUD operations continue working
   - Uses resources from `test/upgrade/testdata/baseCrs/`

2. **Custom Upgrade Tests** - Specialized validation (Test_*_External_Name)
   - Validates external-name format compliance with ADR
   - Tests custom pre/post upgrade conditions
   - Uses resources from `test/upgrade/testdata/customCRs/`

## Test Flow

### Base Upgrade Test Flow

1. **Setup** - Create kind cluster with Crossplane and install provider (FROM version)
2. **Import Resources** - Create test resources from `test/upgrade/testdata/baseCrs/`
3. **Verify Before Upgrade** - Ensure all resources are Ready
4. **Upgrade** - Update provider to newer version (TO version)
5. **Verify After Upgrade** - Confirm resources still work correctly
6. **Cleanup** - Delete resources and destroy cluster

### Custom Upgrade Test Flow

1. **Setup** - Create kind cluster with Crossplane and install provider (FROM version)
2. **Import Resources** - Create test resources from `test/upgrade/testdata/customCRs/{testName}/`
3. **Verify Before Upgrade** - Standard verification + custom pre-upgrade checks
4. **Pre-Upgrade Assessment** - Run custom validation (e.g., verify external-name format)
5. **Upgrade** - Update provider to newer version (TO version)
6. **Verify After Upgrade** - Standard verification + custom post-upgrade checks
7. **Post-Upgrade Assessment** - Verify custom conditions (e.g., external-name unchanged)
8. **Cleanup** - Delete resources and destroy cluster

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

### ⚠️ IMPORTANT: Necessary configuration steps before running any tests

Some configuration steps are necessary before you can successfully run any upgrade tests:
- Configure your CF organization
- Configure your CF space

#### CF Organization

Before running any tests, you **must** update the organization name to one you have access:

1. **List your available CF organizations:**
```bash
cf login
cf orgs
```

2. **Update the organization name** in test manifests:
   - For base tests: `test/upgrade/testdata/baseCrs/import.yaml`
   - For custom tests: `test/upgrade/testdata/customCRs/*/import.yaml` (if applicable)
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

#### CF Space

Before running the base tests, you **must** update the space name to one in your organization.
That can either be an existing one you have atleast the SpaceDeveloper role in or you create a new one as describe below:

1. **Optionally create and configure a new CF Space**

Create a space and give your user the SpaceDeveloper role
```bash
cf create-space <SPACE_NAME> -o <ORG_NAME>  # Create a space in your org
cf set-space-role <USERNAME> <ORG_NAME> <SPACE_NAME> SpaceDeveloper # Assign your user the SpaceDeveloper role
```

2. **List your available CF spaces:**
```bash
cf spaces # List spaces
```
3. **Update the spaces name** in test manifests: 
- For base tests: `test/upgrade/testdata/baseCrs/import.yaml`:
- For custom tests: `test/upgrade/testdata/customCRs/*/import.yaml` (if applicable)
```yaml
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
  name: upgrade-test-import-space
spec:
  managementPolicies:
    - Observe
  forProvider:
    name: upgrade-test-space-donotdelete  # ← Change this to you CF space name
    orgRef:
      name: upgrade-test-org 
  providerConfigRef:
    name: default
```

### 1. Set Environment Variables

#### Option A: Use the provided template 
Copy the example environment file and fill in your credentials:

```bash
cp test/upgrade/.env.example test/upgrade/.env
# Edit .env with your actual credentials
source test/upgrade/.env
```

Option B: Export variables directly
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

⚠️ Security Note: Never commit the .env file! It's already in .gitignore to prevent accidental commits of credentials.

### 2. Run the Tests

From the project root directory:
```bash
# Run ALL upgrade tests (base + custom)
make test-upgrade

# Run ONLY base upgrade tests
make test-upgrade-base

# Run ONLY custom upgrade tests
make test-upgrade-custom

# Run specific custom test
export UPGRADE_TEST_FILTER='Test_Space_External_Name'
make test-upgrade-custom
```

**Test your local changes:**
```bash
# Build local provider image first
make build

# Test upgrade FROM a release TO your local changes
export UPGRADE_TEST_FROM_TAG="v0.3.2"
export UPGRADE_TEST_TO_TAG="local"  # Uses your current code

# Run all tests
make test-upgrade

# Or run only custom tests
make test-upgrade-custom
```

### 3. Customize (Optional)

Override defaults as needed:
```bash
# Increase timeout for slow resources
export UPGRADE_TEST_VERIFY_TIMEOUT="45"  # minutes

# Increase wait time after upgrade
export UPGRADE_TEST_WAIT_FOR_PAUSE="2"  # minutes

# Filter to specific test
export UPGRADE_TEST_FILTER="Test_Space_External_Name"

# Then run tests
make test-upgrade-custom
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
| `UPGRADE_TEST_TO_TAG` | Provider version to upgrade to | `v0.3.2` or `local` |

### Optional Variables (with defaults)

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `UPGRADE_TEST_VERIFY_TIMEOUT` | Timeout for resource verification (minutes) | `30` | `45` |
| `UPGRADE_TEST_WAIT_FOR_PAUSE` | Wait time after provider upgrade (minutes) | `1` | `2` |
| `UPGRADE_TEST_FILTER` | Test name filter for custom tests | `.` (all) | `Test_Space_External_Name` |

## Test Resources

### Base Upgrade Tests

Base tests use YAML manifests from `test/upgrade/testdata/baseCrs/`. Currently tested resources:

- **Organization** (import) - Uses `managementPolicies: [Observe]` to import existing org
- **Space** - Lightweight resource for testing basic upgrade flow
- **Domain**
- **SpaceQuota**
- **SpaceRole**
- **SpaceMembers**
- **ServiceInstance**
- **ServiceCredentialBinding**

#### Test Base Resource Dependencies
- **SpaceRole:** A space role can only be assigned to a user if the user is also a member of the space's organization.\
🠊 Assign a user to the space's organization by either creating a SpaceMembers/SpaceRole resource or by using the BTP Cockpit
- **ServiceInstance:** A managed service instance requires a ServicePlan specifying an offering and a plan.
If the combination of offering and plan is not available in your space change it something different.\
🠊 Run `cf marketplace` and update the values in test/upgrade/testdata/baseCrs/service_instance.yaml

#### Adding New Base Test Resources

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

### Custom Upgrade Tests

Custom tests use YAML manifests from `test/upgrade/testdata/customCRs/{testName}/`. Each custom test has its own subdirectory with specific test resources.

#### Current Custom Tests

**Test_Space_External_Name** (`testdata/customCRs/externalNames/`)
- Validates Space external-name follows UUID format
- Verifies external-name doesn't change during upgrade
- Tests external-name ADR compliance

#### Adding New Custom Test Resources

1. Create a subdirectory for your test in `test/upgrade/testdata/customCRs/`:
```bash
mkdir -p test/upgrade/testdata/customCRs/myCustomTest
```

2. Create resource manifests:
```bash
cat > test/upgrade/testdata/customCRs/myCustomTest/space.yaml <<EOF
apiVersion: cloudfoundry.crossplane.io/v1alpha1
kind: Space
metadata:
  name: my-custom-test-space
spec:
  forProvider:
    name: "my-custom-test-space"
    orgGuid: "your-org-guid"
  providerConfigRef:
    name: default
EOF
```

3. Create the test file in `test/upgrade/`:
```go
//go:build upgrade

package upgrade

import (
    "context"
    "testing"
    
    "github.com/SAP/crossplane-provider-cloudfoundry/apis/cloudfoundry/v1alpha1"
    "github.com/SAP/crossplane-provider-cloudfoundry/test"
    "k8s.io/klog/v2"
    "sigs.k8s.io/e2e-framework/pkg/envconf"
)

var (
    myCustomTestDirs = []string{
        "./testdata/customCRs/myCustomTest",
    }
)

func Test_My_Custom_Validation(t *testing.T) {
    const spaceName = "my-custom-test-space"
    
    upgradeTest := test.NewCustomUpgradeTest("my-custom-test", fromPackage, toPackage).
        WithResourceDirectories(myCustomTestDirs).
        PreUpgradeAssessment(
            "verify custom condition before upgrade",
            func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
                // Your pre-upgrade validation logic
                return ctx
            },
        ).
        PostUpgradeAssessment(
            "verify custom condition after upgrade",
            func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
                // Your post-upgrade validation logic
                return ctx
            },
        )
    
    upgradeTest.Run(t)
}
```

4. Run your custom test:
```bash
export UPGRADE_TEST_FILTER='Test_My_Custom_Validation'
make test-upgrade-custom
```

### Resource Selection Tips

**Suggested resources for testing:**
- ✅ Resources you can create/delete with your credentials
- ✅ Resources with minimal dependencies
- ✅ Resources using `managementPolicies: [Observe]` (safest - no creation)

## Test Structure
```
test/
├── upgrade/
│   ├── testdata/
│   │   ├── baseCrs/                      # Base upgrade test resources
│   │   │   ├── import.yaml               # Organization (observe) + Space (observe)
│   │   │   ├── space.yaml                # Space (create)
|   │   │   ├── domain.yaml
|   │   │   ├── space_quota.yaml
|   │   │   ├── space_role.yaml
|   │   │   ├── service_credential_binding.yaml
|   │   │   ├── service_instance.yaml
|   │   │   └── space_members.yaml
│   │   └── customCRs/                    # Custom upgrade test resources
│   │       └── externalNames/            # External-name validation test
│   │           ├── space.yaml
|   |           └── import.yaml
│   ├── main_test.go                      # Test environment setup
│   ├── upgrade_test.go                   # Base upgrade test logic
│   ├── base_upgrade_test.go              # Custom upgrade test framework
│   ├── space_external_name_upgrade_test.go # External-name validation test
│   └── README.md                         # This file
├── e2e/
│   └── crs/                              # E2E resource manifests
│       └── orgspace/
│           ├── import.yaml
│           └── space.yaml
└── testutil.go                           # Shared helper functions
```

## Make Targets

### Test Execution
- `make test-upgrade` - Run ALL upgrade tests (base + custom)
- `make test-upgrade-base` - Run base upgrade tests only
- `make test-upgrade-custom` - Run custom upgrade tests only
- `make test-upgrade-with-version-crs` - Run tests with auto-checkout of CRs from FROM_TAG

### Utilities
- `make test-upgrade-compile` - Verify upgrade tests compile
- `make test-upgrade-clean` - Clean up test artifacts
- `make test-upgrade-restore-crs` - Restore testdata/ to current version
- `make test-upgrade-help` - Show detailed usage examples

### Debugging
- `make test-upgrade-debug` - Run upgrade tests with debugger (connect to localhost:2345)

## Development

### Running Tests Locally
```bash
# Test all upgrade tests with specific versions
export UPGRADE_TEST_FROM_TAG="v0.3.0"
export UPGRADE_TEST_TO_TAG="local"  # Test unreleased code

cd test/upgrade
go test -v -tags=upgrade -timeout=45m ./...

# Run only base tests
go test -v -tags=upgrade -timeout=45m -run TestUpgradeProvider ./...

# Run only custom tests
go test -v -tags=upgrade -timeout=45m -run 'Test_.*_External_Name' ./...

# Run specific custom test
go test -v -tags=upgrade -timeout=45m -run Test_Space_External_Name ./...
```

### Crossplane Version

Change Crossplane version in `main_test.go`:
```go
const crossplaneVersion = "CHOSEN_VERSION"
```

## Troubleshooting

### Common Issues

#### Error: "external resource does not exist"
**Cause:** The organization name in manifests doesn't exist or you don't have access

**Solution:** 
1. Run `cf orgs` to see your available organizations
2. Update manifests in `test/upgrade/testdata/baseCrs/` and `test/upgrade/testdata/customCRs/*/` 
3. Ensure you have at least read access to the organization

#### Error: "no non-test Go files"
**Cause:** Missing build tag when compiling

**Solution:** Always use `-tags=upgrade`:
```bash
go test -tags=upgrade ./...
```

#### Error: "External name does not match expected UUID format"
**Cause:** Custom test detected external-name format violation

**Solution:**
- This is expected behavior if your provider doesn't follow the external-name ADR
- Check the resource's external-name annotation
- Review external-name handling in the controller code

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

## Related Documentation

- [xp-testing Framework](https://github.com/crossplane-contrib/xp-testing)
- [Crossplane Documentation](https://docs.crossplane.io)
- [CloudFoundry Provider](https://github.com/SAP/crossplane-provider-cloudfoundry)
- [BTP Provider Upgrade Tests](https://github.com/SAP/crossplane-provider-btp/tree/main/test/upgrade) (reference implementation)
- [External-Name ADR](../docs/adr/) (internal documentation for external-name patterns)