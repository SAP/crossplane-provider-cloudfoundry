//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

func main() {
	buildID := os.Getenv("BUILD_ID")
	if buildID == "" {
		fmt.Println("BUILD_ID not set, defaulting to 0000")
		buildID = "0000"
	}

	cfClient, err := newCfClient()
	if err != nil {
		fmt.Printf("Failed to create CF client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	// cf-ci-e2e is a shared, long-lived org — only its child resources are
	// cleaned up here. The org itself is never deleted.
	orgName := "cf-ci-e2e"

	org, err := cfClient.Organizations.Single(ctx, &client.OrganizationListOptions{
		Names: client.Filter{Values: []string{orgName}},
	})
	if err != nil {
		fmt.Printf("Test org %s not found, skipping cleanup: %v\n", orgName, err)
		return
	}

	scopedSuffix := "-" + buildID
	logCleanupErr("e2e-scb-key"+scopedSuffix, deleteSCB(ctx, cfClient, "e2e-scb-key"+scopedSuffix, "e2e-service-instance"+scopedSuffix))

	for _, domain := range []string{
		"cfapps.eu12.hana.ondemand.com",
		"v6.cfapps.eu12.hana.ondemand.com",
	} {
		for _, host := range []string{
			"service-route-e2e" + scopedSuffix,
			"app-route-host-domainref" + scopedSuffix,
			"app-route-host-domainname" + scopedSuffix,
			"route-import-e2e" + scopedSuffix,
		} {
			logCleanupErr(host, deleteRoute(ctx, cfClient, org.GUID, domain, host))
		}
	}

	for _, app := range []string{
		"e2e-app" + scopedSuffix,
		"e2e-app-2" + scopedSuffix,
	} {
		logCleanupErr(app, deleteApp(ctx, cfClient, org.GUID, app))
	}

	for _, si := range []string{
		"e2e-service-instance" + scopedSuffix,
		"e2e-ups" + scopedSuffix,
		"e2e-ups-no-credentials" + scopedSuffix,
		"e2e-serviceroutebinding-serviceinstance" + scopedSuffix,
	} {
		logCleanupErr(si, deleteServiceInstance(ctx, cfClient, org.GUID, si))
	}

	logCleanupErr("e2e-space"+scopedSuffix, deleteSpace(ctx, cfClient, org.GUID, "e2e-space"+scopedSuffix))

	logCleanupErr("e2e-space-quota"+scopedSuffix, deleteQuota(ctx, cfClient, org.GUID, "e2e-space-quota"+scopedSuffix))

	// Delete import test spaces (under cf-ci-e2e org)
	logCleanupErr("e2e-test-space-import"+scopedSuffix, deleteSpace(ctx, cfClient, org.GUID, "e2e-test-space-import"+scopedSuffix))

	fmt.Println("Cleanup completed")
}

// logCleanupErr logs but does not propagate — cleanup is best-effort.
func logCleanupErr(resource string, err error) {
	if err != nil {
		fmt.Printf("Cleanup %s: %v\n", resource, err)
	}
}

func newCfClient() (*client.Client, error) {
	endpoint := os.Getenv("CF_ENVIRONMENT")
	creds := os.Getenv("CF_CREDENTIALS")

	var s clients.CfCredentials
	if err := json.Unmarshal([]byte(creds), &s); err != nil {
		return nil, fmt.Errorf("cannot extract CF credentials: %w", err)
	}

	cfg, err := config.New(endpoint, config.UserPassword(s.Email, s.Password), config.SkipTLSValidation())
	if err != nil {
		return nil, fmt.Errorf("cannot configure CF client: %w", err)
	}

	return client.New(cfg)
}

func deleteSCB(ctx context.Context, cfClient *client.Client, name, serviceInstanceName string) error {
	scb, err := cfClient.ServiceCredentialBindings.Single(ctx, &client.ServiceCredentialBindingListOptions{
		Names:                client.Filter{Values: []string{name}},
		ServiceInstanceNames: client.Filter{Values: []string{serviceInstanceName}},
	})
	if err != nil {
		return err
	}
	_, err = cfClient.ServiceCredentialBindings.Delete(ctx, scb.GUID)
	return err
}

func deleteRoute(ctx context.Context, cfClient *client.Client, orgID, domain, host string) error {
	d, err := cfClient.Domains.Single(ctx, &client.DomainListOptions{
		Names: client.Filter{Values: []string{domain}},
	})
	if err != nil {
		return nil
	}
	r, err := cfClient.Routes.Single(ctx, &client.RouteListOptions{
		OrganizationGUIDs: client.Filter{Values: []string{orgID}},
		DomainGUIDs:       client.Filter{Values: []string{d.GUID}},
		Hosts:             client.Filter{Values: []string{host}},
	})
	if err != nil {
		return nil
	}
	_, err = cfClient.Routes.Delete(ctx, r.GUID)
	return err
}

func deleteApp(ctx context.Context, cfClient *client.Client, orgID, name string) error {
	a, err := cfClient.Applications.Single(ctx, &client.AppListOptions{
		OrganizationGUIDs: client.Filter{Values: []string{orgID}},
		Names:             client.Filter{Values: []string{name}},
	})
	if err != nil {
		return nil
	}
	_, err = cfClient.Applications.Delete(ctx, a.GUID)
	return err
}

func deleteServiceInstance(ctx context.Context, cfClient *client.Client, orgID, name string) error {
	si, err := cfClient.ServiceInstances.Single(ctx, &client.ServiceInstanceListOptions{
		OrganizationGUIDs: client.Filter{Values: []string{orgID}},
		Names:             client.Filter{Values: []string{name}},
	})
	if err != nil {
		return nil
	}
	_, err = cfClient.ServiceInstances.Delete(ctx, si.GUID)
	return err
}

func deleteSpace(ctx context.Context, cfClient *client.Client, orgID, name string) error {
	s, err := cfClient.Spaces.Single(ctx, &client.SpaceListOptions{
		OrganizationGUIDs: client.Filter{Values: []string{orgID}},
		Names:             client.Filter{Values: []string{name}},
	})
	if err != nil {
		return nil
	}
	_, err = cfClient.Spaces.Delete(ctx, s.GUID)
	return err
}

func deleteQuota(ctx context.Context, cfClient *client.Client, orgID, name string) error {
	q, err := cfClient.SpaceQuotas.Single(ctx, &client.SpaceQuotaListOptions{
		OrganizationGUIDs: client.Filter{Values: []string{orgID}},
		Names:             client.Filter{Values: []string{name}},
	})
	if err != nil {
		return nil
	}
	if q.Relationships.Spaces != nil {
		for _, space := range q.Relationships.Spaces.Data {
			_ = cfClient.SpaceQuotas.Remove(ctx, q.GUID, space.GUID)
		}
	}
	_, err = cfClient.SpaceQuotas.Delete(ctx, q.GUID)
	return err
}
