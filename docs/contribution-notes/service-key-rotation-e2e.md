# Concurrent E2E service broker fixtures

Concurrent Cloud Foundry E2E jobs share the configured test organization and space. Each job that provisions a managed service should own a separate service instance and use a unique application identity. The ServiceCredentialBinding E2E fixtures use XSUAA `application` instances with distinct, run-scoped `xsappname` values and explicit `binding-secret` credential configuration. Check service and key quotas when changing these fixtures. These are contributor test-environment settings, not provider defaults or end-user guidance.

Run the fixture/helper tests and compile the E2E package without starting infrastructure:

```sh
GOFIPS140=certified go test ./test -run '^TestRotationFixtures' -count=1
GOFIPS140=certified go test ./test -count=1
GOFIPS140=certified go test -c -tags=e2e -o /tmp/cf-scb-e2e.test ./test/e2e
```

Actual acceptance tests provision CF resources in the approved CI environment. Compile-only or non-overlapping runs do not establish safe concurrent provisioning. Follow the repository's infrastructure-test authorization and cleanup guidance before running them. Keep credentials out of fixture parameters and do not modify shared organization or space resources.
