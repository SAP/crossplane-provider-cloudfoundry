//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/crossplane-contrib/xp-testing/pkg/envvar"
	"k8s.io/klog"
	"sigs.k8s.io/e2e-framework/pkg/env"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/pkg/errors"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients"
)

var (
	UUT_IMAGES_KEY     = "UUT_IMAGES"
	UUT_CONFIG_KEY     = "package"
	UUT_CONTROLLER_KEY = "controller"
	ENDPOINT_KEY       = "apiEndpoint"
	CREDENTIALS_KEY    = "credentials"
)
var (
	testenv       env.Environment
	testOrgName   = "cf-ci-e2e"
	testDomain    = "e2e.orchestrator.io"
	testAppDomain = "cfapps.eu12.hana.ondemand.com"
	testSpaceName = "e2e-space"
	testQuotaName = "e2e-space-quota"
)

func resetTestOrg(ctx context.Context, t *testing.T) {
	cfClient, err := getCfClient()
	if err != nil {
		t.Fatalf("cannot connect to cloudfoundry")
	}

	org, err := orgID(ctx, cfClient, testOrgName)
	if err != nil {
		t.Fatalf("test org %s not accessible", testOrgName)
	}
	_ = deleteRoute(ctx, cfClient, org, testDomain, "app-host")
	_ = deleteRoute(ctx, cfClient, org, testAppDomain, "app-route-host-domainref")
	_ = deleteRoute(ctx, cfClient, org, testAppDomain, "app-route-host-domainname")
	_ = deleteDomain(ctx, cfClient, org, testDomain)
	_ = deleteSpace(ctx, cfClient, org, testSpaceName)
	_ = deleteQuota(ctx, cfClient, org, testQuotaName)
}

func getProviderConfigSecretData() map[string]string {
	secretData := map[string]string{
		CREDENTIALS_KEY: envvar.GetOrPanic("CF_CREDENTIALS"),
		ENDPOINT_KEY:    envvar.GetOrPanic("CF_ENVIRONMENT"),
	}
	return secretData

}

func getCfClient() (*client.Client, error) {
	secretData := getProviderConfigSecretData()

	endpoint := secretData[ENDPOINT_KEY]
	creds := secretData[CREDENTIALS_KEY]

	var s clients.CfCredentials
	if err := json.Unmarshal([]byte(creds), &s); err != nil {
		return nil, errors.Wrap(err, "cannot extract cloud foundry credentials from env variable")
	}
	cfg, err := config.New(endpoint, config.UserPassword(s.Email, s.Password), config.SkipTLSValidation())
	if err != nil {
		return nil, errors.Wrap(err, "cannot configure cloudfoundry client")
	}

	return client.New(cfg)
}

func orgID(ctx context.Context, cfClient *client.Client, org string) (string, error) {
	s, err := cfClient.Organizations.Single(ctx,
		&client.OrganizationListOptions{
			Names: client.Filter{Values: []string{org}},
		})

	if err != nil {
		return "", err
	}

	return s.GUID, nil
}

func deleteSpace(ctx context.Context, cfClient *client.Client, org string, space string) error {
	s, err := cfClient.Spaces.Single(ctx,
		&client.SpaceListOptions{
			OrganizationGUIDs: client.Filter{Values: []string{org}},
			Names:             client.Filter{Values: []string{space}},
		})
	if err == nil {
		klog.V(4).Info("found test space! cleaning up")
		_, err = cfClient.Spaces.Delete(ctx, s.GUID)
		return err
	}

	return nil

}

func deleteDomain(ctx context.Context, cfClient *client.Client, org string, domain string) error {
	d, err := cfClient.Domains.Single(ctx,
		&client.DomainListOptions{
			OrganizationGUIDs: client.Filter{Values: []string{org}},
			Names:             client.Filter{Values: []string{domain}},
		})
	if err == nil {
		_, err = cfClient.Domains.Delete(ctx, d.GUID)
		return err
	}
	return err
}

func deleteRoute(ctx context.Context, cfClient *client.Client, org string, domain string, route string) error {
	d, err := cfClient.Domains.Single(ctx,
		&client.DomainListOptions{
			OrganizationGUIDs: client.Filter{Values: []string{org}},
			Names:             client.Filter{Values: []string{domain}},
		})
	if err == nil {
		s, err := cfClient.Routes.Single(ctx,
			&client.RouteListOptions{
				OrganizationGUIDs: client.Filter{Values: []string{org}},
				DomainGUIDs:       client.Filter{Values: []string{d.GUID}},
				Hosts:             client.Filter{Values: []string{route}},
			})

		if err == nil {
			klog.V(4).Info("found test route! cleaning up")
			_, err = cfClient.Routes.Delete(ctx, s.GUID)
			return err
		}
		return nil
	}
	return err
}
func deleteQuota(ctx context.Context, cfClient *client.Client, org string, quota string) error {
	s, err := cfClient.SpaceQuotas.Single(ctx,
		&client.SpaceQuotaListOptions{
			OrganizationGUIDs: client.Filter{Values: []string{org}},
			Names:             client.Filter{Values: []string{quota}},
		})
	if err == nil {
		klog.V(4).Info("found test spaceQuota! cleaning up")
		if s.Relationships.Spaces != nil {
			for _, space := range s.Relationships.Spaces.Data {
				_ = cfClient.SpaceQuotas.Remove(ctx, s.GUID, space.GUID)
			}
		}
		_, err = cfClient.SpaceQuotas.Delete(ctx, s.GUID)
		return err
	}
	return nil
}
