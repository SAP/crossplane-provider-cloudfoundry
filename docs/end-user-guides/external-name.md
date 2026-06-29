---
sidebar_position: 5
---

# External name

`External name` in `Crossplane` is a key concept that maps `Crossplane` resources to their corresponding external resources in the managed infrastructure.

## What is External Name

The `External name` is an annotation (`crossplane.io/external-name`) that stores the identifier of the actual resource in the external system. It bridges the gap between:

- Crossplane resource name: The Kubernetes-style name in your cluster
- External resource ID: The actual identifier in the provider's API (e.g. Space GUID)

In the Cloud Foundry provider you can use the `External name` annotation to import existing resources.

## Cloud Foundry resources

To import existing Cloud Foundry resources you need to add annotation with existing resource identifier

```yaml
...
metadata.annotations.crossplane.io/external-name: <resource_uniq_ID>
...
```

## Generated Data Below

### App

- Follows Standard: yes
- Format: App GUID (UUID format)
- How to find:

  - UI: In the BTP Cockpit, navigate to your app and find the ID after app/ in the URL
  - CLI: `cf app <APP_NAME> --guid`

### Domain

- Follows Standard: yes
- Format: Domain GUID (UUID format)
- How to find:

  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf domains` (see GUID column)

### OrgMembers

- Follows Standard: no (uses compound key <org-guid>/<role-type>, not a single GUID)
- Format: <org-guid>/<role-type>
- How to find:

  - UI: BTP Cockpit → Subaccounts → [Select Subaccount] → Cloud Foundry → Organization → Org ID + Settings → Org Members
  - CLI: cf org <ORG_NAME> --guid (field: guid) combined with spec.forProvider.roleType

### OrgQuota

- Follows Standard: yes
- Format: GUID (UUID v4)
- How to find:

  - UI: Cloud Foundry > Quota Definitions > <quota name> (GUID in URL or details)
  - CLI: cf curl /v3/organization_quotas?names=<name> (field: guid)

### OrgRole

- Follows Standard: yes
- Format: Org Role GUID (UUID format)
- How to find:

  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf org-users <ORG> -v` and find the GUID in the output

### Organization

- Follows Standard: yes
- Format: Organization GUID (UUID format)
- How to find:

  - UI: In the BTP Cockpit, navigate to your org and find the ID in the URL
  - CLI: Use `cf org <org-name> --guid`

### Route

- Follows Standard: yes
- Format: Route GUID (UUID format)
- How to find:

  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf routes` and find the GUID in the output

### ServiceCredentialBinding

- Follows Standard: yes
- Format: Service Credential Binding GUID (UUID format)
- How to find:

  - For type: key
  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf service-keys <SERVICE_INSTANCE>` and look up the key GUID via `cf curl /v3/service_credential_bindings?names=<KEY_NAME>`
  - For type: app
  - UI: Open app > Service Bindings > Service Binding GUID column
  - CLI: `cf service <SERVICE_INSTANCE>` > Showing bound apps > guid column

### ServiceInstance

- Follows Standard: yes
- Format: ServiceInstance GUID (UUID format)
- How to find:

  - UI: In the BTP Cockpit, open the service instance detail view; the GUID is shown in the "Instance ID" field
  - CLI: `cf service <SERVICE_INSTANCE_NAME> --guid`

### ServiceRouteBinding

- Follows Standard: yes
- Format: Service Route Binding GUID (UUID format)
- How to find:

  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf <SERVICE_INSTANCE> -v` and find the GUID in the output.

### Space

- Follows Standard: yes
- Format: Space GUID (UUID format)
- How to find:

  - UI: Global Account → Account Explorer → Subaccounts → Select Subaccount → Spaces → Select Space → View URL: `https://<cockpit_url>/cockpit#/globalaccount/<global_account_id>/subaccount/<subaccount_id>/org/<org_id>/space/<SPACE_ID>/applications`
  - CLI: Use CF CLI: `cf space <SPACE> --guid`

### SpaceMembers

- Follows Standard: no (uses compound key <space-guid>/<role-type>, not a single GUID)
- Format: <space-guid>/<role-type>
- How to find:

  - UI: BTP Cockpit → Subaccounts → [Select Subaccount] → Cloud Foundry → Space → Space ID + Settings → Space Members
  - CLI: cf space <SPACE_NAME> --guid (field: guid) combined with spec.forProvider.roleType

### SpaceQuota

- Follows Standard: yes
- Format: Space Quota GUID (UUID format)
- How to find:

  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf space-quota <QUOTA-NAME> -v` and find the GUID in the output

### SpaceRole

- Follows Standard: yes
- Format: Space Role GUID (UUID format)
- How to find:

  - UI: Not available in the BTP Cockpit
  - CLI: Use CF CLI: `cf space-users <ORG> <SPACE> -v` and find the GUID in the output
