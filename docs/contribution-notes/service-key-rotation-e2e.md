# Concurrent E2E service broker fixtures

Concurrent Cloud Foundry E2E jobs share the configured test organization and space. Each job that provisions a managed service should own a separate service instance and use a unique application identity. The ServiceCredentialBinding E2E fixtures use XSUAA `application` instances with distinct, run-scoped `xsappname` values and explicit `binding-secret` credential configuration. Check service-instance and service-key quotas when changing these fixtures. These are contributor test-environment settings, not provider defaults or end-user guidance.

Apply prerequisites in dependency order: first the read-only organization/space imports, then the owned service instance, and only after successful provisioning the dependent binding. Keep these stages local to each independent test; do not serialize CI jobs to avoid shared-space contention. Phase logs identify progress, elapsed time, and allowlisted readiness/operation facts. A setup timeout means a prerequisite did not reach its expected state within the bounded assessment window; permission, invalid-configuration, failed-provisioning, and paused-resource diagnoses are actionable failures. Diagnostics must not print raw condition/event messages, API error bodies, parameters, Secrets, or full resource objects. Partial setup must still use the test's owned-resource teardown.

The rotation assertions compare the broker-returned, nonempty XSUAA `clientsecret` across intentional generations, while independently checking that the current generation's credentials are correctly published to the owned Secret. Secret resourceVersion changes or JSON-versus-flattened representation changes are not proof of changed credentials. Retired-key cleanup is complete only after the key is absent from Cloud Foundry **and** its GUID is absent from the managed resource's retired-key status; the active key and its published credentials are checked separately. Teardown waits for owned resources to finalize and verifies tracked Cloud Foundry identities without directly deleting CF resources.

Run local helper tests and compile the E2E package without starting infrastructure:

```sh
GOFIPS140=certified go test ./test -run '^TestRotation' -count=1
GOFIPS140=certified go test ./test -count=1
GOFIPS140=certified go test -c -tags=e2e -o /tmp/cf-scb-e2e.test ./test/e2e
```

A local compile or an individual/serial smoke run is not evidence of concurrent provisioning. Required acceptance uses the approved CI matrix at the exact candidate SHA, with both rotation jobs' provisioning/assessment intervals overlapping against the same configured org/space. Repeat the concurrent round and inspect phase timestamps, conclusions, quota/permission failures, and teardown results. Do not launch two acceptance Make commands in one checkout: they share rendered fixtures and the default cluster. Actual acceptance tests provision CF resources in the approved CI environment; follow the repository's infrastructure-test authorization and cleanup guidance before running them. Keep credentials out of fixture parameters and do not modify shared organization or space resources.
