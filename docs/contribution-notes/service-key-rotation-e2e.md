# Concurrent E2E service broker fixtures

Concurrent Cloud Foundry E2E jobs share the configured test organization and space. Each job that provisions a managed service should own a separate service instance and use a unique application identity. The ServiceCredentialBinding E2E fixtures use XSUAA `application` instances with distinct, run-scoped `xsappname` values and explicit `binding-secret` credential configuration. Check service and key quotas when changing these fixtures. These are contributor test-environment settings, not provider defaults or end-user guidance.

Apply prerequisites in dependency order: first the read-only organization/space imports, then the owned service instance, and only after successful provisioning the dependent resource. Keep these stages local to each independent test; do not serialize CI jobs to avoid shared-space contention. A setup timeout means a prerequisite did not reach its expected state within the bounded assessment window; a permission, invalid-configuration, failed-provisioning, or paused-resource diagnosis should be treated as an actionable setup failure. Diagnostics should report only allowlisted reasons, operation state, identity/readiness booleans, and pause-marker presence. Never print raw condition/event messages, API error bodies, parameters, Secrets, or full resource objects. Partial setup must still use the test's owned-resource teardown.

Run the fixture/helper tests and compile the E2E package without starting infrastructure:

```sh
GOFIPS140=certified go test ./test -run '^TestRotationFixtures' -count=1
GOFIPS140=certified go test ./test -count=1
GOFIPS140=certified go test -c -tags=e2e -o /tmp/cf-scb-e2e.test ./test/e2e
```

Actual acceptance tests provision CF resources in the approved CI environment. Compile-only or non-overlapping runs do not establish safe concurrent provisioning. Follow the repository's infrastructure-test authorization and cleanup guidance before running them. Keep credentials out of fixture parameters and do not modify shared organization or space resources.
