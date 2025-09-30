package main

import (
	"context"
	"sync"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/configparam"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/widget"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/config"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	cpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
)

func getCFConfig() (*config.Config, error) {
	apiUrl, err := apiUrlParam.(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return nil, err
	}
	username, err := usernameParam.(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return nil, err
	}
	password, err := passwordParam.(*configparam.StringParam).ValueOrAsk()
	if err != nil {
		return nil, err
	}
	return config.New(apiUrl, config.UserPassword(username, password))
}

func getOrganizations(ctx context.Context, cfClient *client.Client) ([]*resource.Organization, error) {
	return cfClient.Organizations.ListAll(ctx, client.NewOrganizationListOptions())
}

func selectOrganizations(ctx context.Context, cfClient *client.Client, title string) ([]string, error) {
	orgs, err := getOrganizations(ctx, cfClient)
	if err != nil {
		return nil, err
	}
	orgMap := make(map[string]*resource.Organization)
	orgNames := make([]string, len(orgs))
	for i, org := range orgs {
		orgMap[org.Name] = org
		orgNames[i] = org.Name
	}
	selectedOrgs := widget.MultiInput(title, orgNames)
	orgGUIDs := make([]string, len(selectedOrgs))
	for i, selectedOrg := range selectedOrgs {
		org, ok := orgMap[selectedOrg]
		if !ok {
			panic("orgMap does not contain selectedOrg")
		}
		orgGUIDs[i] = org.GUID
	}
	return orgGUIDs, nil
}

func getSpaces(ctx context.Context, cfClient *client.Client) ([]*resource.Space, error) {
	// orgNames, err := selectOrganizations(ctx, cfClient, "Collect spaces in organization")
	orgNames, err := orgsParam.(*configparam.StringSliceParam).ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	listOptions := client.NewSpaceListOptions()
	listOptions.OrganizationGUIDs.Values, err = orgCache.getGuidsByNames(orgNames)
	if err != nil {
		return nil, err
	}

	return cfClient.Spaces.ListAll(ctx, listOptions)
}

func getServiceInstances(ctx context.Context, cfClient *client.Client) ([]*resource.ServiceInstance, error) {
	orgNames, err := orgsParam.(*configparam.StringSliceParam).ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	orgGuids, err := orgCache.getGuidsByNames(orgNames)
	if err != nil {
		return nil, err
	}

	ctx = context.WithValue(ctx, orgGuidsKey, orgGuids)
	spaceNames, err := spacesParam.(*configparam.StringSliceParam).ValueOrAsk(ctx)
	if err != nil {
		return nil, err
	}

	listOptions := client.NewServiceInstanceListOptions()
	listOptions.OrganizationGUIDs.Values = orgGuids
	listOptions.SpaceGUIDs.Values, err = spaceCache.getGuidsByNames(spaceNames)
	if err != nil {
		return nil, err
	}

	return cfClient.ServiceInstances.ListAll(ctx, listOptions)
}

func exportOrgs(cfClient *client.Client, resChan chan<- cpresource.Object) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	orgs, err := getOrganizations(ctx, cfClient)
	if err != nil {
		return erratt.Errorf("cannot list organizations: %w", err)
	}
	for _, org := range orgs {
		resChan <- convertOrgResource(org)
	}
	return nil
}

func exportSpaces(cfClient *client.Client, resChan chan<- cpresource.Object) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	spaces, err := getSpaces(ctx, cfClient)
	if err != nil {
		return erratt.Errorf("cannot get spaces: %w", err)
	}
	for _, space := range spaces {
		resChan <- convertSpaceResource(space)
	}
	return nil
}

func exportServiceInstances(cfClient *client.Client, resChan chan<- cpresource.Object, errChan chan<- erratt.ErrorWithAttrs) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	serviceInstances, err := getServiceInstances(ctx, cfClient)
	if err != nil {
		return erratt.Errorf("cannot get serviceInstances: %w", err)
	}
	wg := sync.WaitGroup{}
	tokenChan := make(chan struct{}, 10)
	for _, serviceInstance := range serviceInstances {
		wg.Add(1)
		tokenChan <- struct{}{}
		go func() {
			defer wg.Done()
			si := convertServiceInstanceResource(ctx, cfClient, serviceInstance, errChan)
			if si != nil {
				resChan <- si
			}
			<-tokenChan
		}()
	}
	wg.Wait()
	return nil
}
