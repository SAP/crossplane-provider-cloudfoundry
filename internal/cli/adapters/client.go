package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	cfv3 "github.com/cloudfoundry/go-cfclient/v3/client"
	cfconfig "github.com/cloudfoundry/go-cfclient/v3/config"
	"gopkg.in/alecthomas/kingpin.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
	"github.com/SAP/crossplane-provider-cloudfoundry/apis/v1beta1"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/cli/pkg/utils"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/clients/role"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/kubernetes"
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/crossplaneimport/provider"
)

var (
	errIsSSHEnabled         = "Could not check if SSH is enabled for the space"
	errListOrganizations    = "Could not list organizations"
	errCreateCFConfig       = "Could not create CF config"
	errCreateK8sClient      = "Could not create Kubernetes provider"
	errGetProviderConfig    = "Could not get provider config"
	errGetSecret            = "Could not get secret"
	errExtractCredentials   = "Credentials key not found in secret data"
	errExtractApiEndpoint   = "API endpoint key not found in secret data"
	errUnmarshalCredentials = "Failed to unmarshal credentials JSON"
	errGetOrgReference      = "Could not get data about referenced organization"
	errGetSpaceReference    = "Could not get data about referenced space"
	errGetDomainReference   = "Could not get data about referenced domain"
)

// CFCredentials implements the Credentials interface
type CFCredentials struct {
	ApiEndpoint string `json:"ApiEndpoint"`
	Email       string `json:"Email"`
	Password    string `json:"Password"`
}

func (c *CFCredentials) GetAuthData() map[string][]byte {
	return map[string][]byte{
		"apiEndpoint": []byte(c.ApiEndpoint),
		"email":       []byte(c.Email),
		"password":    []byte(c.Password),
	}
}

// CFClient implements the ProviderClient interface
type CFClient struct {
	cf cfv3.Client
}

func (c *CFClient) GetResourcesByType(ctx context.Context, resourceType string, filter map[string]string) ([]interface{}, error) {
	switch resourceType {
	case v1alpha1.Space_Kind:
		return c.getSpaces(ctx, filter)
	case v1alpha1.Org_Kind:
		return c.getOrganizations(ctx, filter)
	case v1alpha1.App_Kind:
		return c.getApps(ctx, filter)
	case v1alpha1.RouteKind:
		return c.getRoutes(ctx, filter)
	case v1alpha1.SpaceMembersKind:
		return c.getSpaceMembers(ctx, filter)
	case v1alpha1.OrgMembersKind:
		return c.getOrgMembers(ctx, filter)
	case v1alpha1.ServiceInstance_Kind:
		return c.getServiceInstances(ctx, filter)
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (c *CFClient) getSpaces(ctx context.Context, filter map[string]string) ([]interface{}, error) {
	// Get name filter
	name, ok := filter["name"]
	if !ok {
		return nil, fmt.Errorf("name filter is required for spaces")
	}
	orgName, ok := filter["org"]
	if !ok {
		return nil, fmt.Errorf("org-reference filter is required for spaces")
	}

	// get referenced org
	orgRefFilter := cfv3.OrganizationListOptions{Names: cfv3.Filter{Values: []string{orgName}}}
	orgRef, err := c.cf.Organizations.ListAll(ctx, &orgRefFilter)
	kingpin.FatalIfError(err, "%s", errGetOrgReference)

	if len(orgRef) == 0 || orgRef[0].GUID == "" {
		kingpin.FatalIfError(fmt.Errorf("organization %s not found", orgName), "%s", errGetOrgReference)
	}

	if orgRef[0].GUID == "" {
		kingpin.FatalIfError(fmt.Errorf("organization %s not found", orgName), "%s", errGetOrgReference)
	}

	// define filter-option with orgRef for query
	opt := &cfv3.SpaceListOptions{OrganizationGUIDs: cfv3.Filter{Values: []string{orgRef[0].GUID}}}

	// Get all spaces from CF
	responseCollection, err := c.cf.Spaces.ListAll(ctx, opt)
	if err != nil {
		return nil, err
	}

	// Filter spaces by name and org-reference
	var results []interface{}
	var SSHlist []bool
	for _, space := range responseCollection {
		// Check if the space name matches
		if utils.IsFullMatch(name, space.Name) {
			results = append(results, space)
			isSSHEnabled, err := c.cf.SpaceFeatures.IsSSHEnabled(ctx, space.GUID)
			kingpin.FatalIfError(err, "%s", errIsSSHEnabled)
			SSHlist = append(SSHlist, isSSHEnabled)
		}
	}

	// Combine results and SSHlist into a slice of interfaces
	combinedResults := make([]interface{}, len(results))
	for i := range results {
		combinedResults[i] = map[string]interface{}{
			"result": results[i],
			"SSH":    SSHlist[i],
		}
	}

	return combinedResults, nil
}

func (c *CFClient) getOrganizations(ctx context.Context, filter map[string]string) ([]interface{}, error) {
	// Get GUID filter
	name, ok := filter["name"]
	if !ok {
		return nil, fmt.Errorf("name filter is required for organizations")
	}
	utils.PrintLine("Fetching organizations with name:", name, 30)

	// Get organizations from CF
	organizations, err := c.cf.Organizations.ListAll(ctx, &cfv3.OrganizationListOptions{})
	kingpin.FatalIfError(err, "%s", errListOrganizations)
	if len(organizations) == 0 {
		utils.PrintLine("Cannot get organizations with name:", name, 30)

		return nil, fmt.Errorf("no organizations found")

	}

	// Filter organizations by name
	var results []interface{}
	for _, organization := range organizations {
		// Check if the organization name matches
		if utils.IsFullMatch(name, organization.Name) {
			results = append(results, organization)
		}
	}

	return results, nil
}

// getSpaceReference retrieves a space reference by name
func (c *CFClient) getSpaceReference(ctx context.Context, filter map[string]string) (string, error) {
	spaceName, ok := filter["space"]
	if !ok {
		return "", fmt.Errorf("space-reference filter is required")
	}

	spaceRefFilter := cfv3.SpaceListOptions{Names: cfv3.Filter{Values: []string{spaceName}}}
	spaceRef, err := c.cf.Spaces.ListAll(ctx, &spaceRefFilter)
	if err != nil {
		return "", fmt.Errorf("%s: %w", errGetSpaceReference, err)
	}

	if len(spaceRef) == 0 || spaceRef[0].GUID == "" {
		return "", fmt.Errorf("%s: space %s not found", errGetSpaceReference, spaceName)
	}

	return spaceRef[0].GUID, nil
}

func (c *CFClient) getApps(ctx context.Context, filter map[string]string) ([]interface{}, error) {

	// Get name filter
	name, ok := filter["name"]
	if !ok {
		return nil, fmt.Errorf("name filter is required for apps")
	}

	// get referenced space
	spaceGUID, err := c.getSpaceReference(ctx, filter)
	if err != nil {
		return nil, err
	}

	utils.PrintLine("Fetching apps in space:", spaceGUID, 30)
	// define filter-option with spaceRef for query
	opt := &cfv3.AppListOptions{SpaceGUIDs: cfv3.Filter{Values: []string{spaceGUID}}}

	// Get apps from CF
	responseCollection, err := c.cf.Applications.ListAll(ctx, opt)
	if err != nil {
		return nil, err
	}

	// Filter spaces by name and org-reference
	var results []interface{}
	for _, app := range responseCollection {
		// Check if the app name matches
		if utils.IsFullMatch(name, app.Name) {
			results = append(results, app)
		}
	}

	return results, nil
}

func (c *CFClient) getRoutes(ctx context.Context, filter map[string]string) ([]interface{}, error) {
	// Get host filter
	host, ok := filter["host"]
	if !ok {
		return nil, fmt.Errorf("host filter is required for routes")
	}
	domainName, ok := filter["domain"]
	if !ok {
		return nil, fmt.Errorf("domain-reference filter is required for routes")
	}

	// get referenced space
	spaceGUID, err := c.getSpaceReference(ctx, filter)
	if err != nil {
		return nil, err
	}

	// get referenced domain
	domainRefFilter := cfv3.DomainListOptions{Names: cfv3.Filter{Values: []string{domainName}}}
	domainRef, err := c.cf.Domains.ListAll(ctx, &domainRefFilter)
	kingpin.FatalIfError(err, "%s", errGetDomainReference)

	if domainRef[0].GUID == "" {
		kingpin.FatalIfError(fmt.Errorf("domain %s not found", domainName), "%s", errGetDomainReference)
	}

	// define filter-option with spaceRef for query
	opt := &cfv3.RouteListOptions{
		SpaceGUIDs:  cfv3.Filter{Values: []string{spaceGUID}},
		DomainGUIDs: cfv3.Filter{Values: []string{domainRef[0].GUID}},
	}

	// Get domains from CF
	responseCollection, err := c.cf.Routes.ListAll(ctx, opt)
	if err != nil {
		return nil, err
	}

	// Filter domains by name and org-reference
	var results []interface{}
	for _, route := range responseCollection {
		// Check if the app name matches
		if utils.IsFullMatch(host, route.Host) {
			results = append(results, route)
		}
	}

	return results, nil
}

func (c *CFClient) getServiceInstances(ctx context.Context, filter map[string]string) ([]interface{}, error) {

	// Get name filter
	name, ok := filter["name"]
	if !ok {
		return nil, fmt.Errorf("name filter is required for service instances")
	}

	utils.PrintLine("Fetching service instances:", name, 30)
	// get referenced space
	spaceGUID, err := c.getSpaceReference(ctx, filter)
	if err != nil {
		return nil, err
	}
	utils.PrintLine("Fetching service instances in space ...", spaceGUID, 30)

	// define filter-option with spaceRef for query
	opt := &cfv3.ServiceInstanceListOptions{SpaceGUIDs: cfv3.Filter{Values: []string{spaceGUID}}}

	if serviceType, ok := filter["type"]; ok {
		opt.Type = serviceType
	}

	// Get service instances from CF
	responseCollection, err := c.cf.ServiceInstances.ListAll(ctx, opt)
	if err != nil {
		return nil, err
	}

	utils.PrintLine("# service instances", strconv.Itoa(len(responseCollection)), 30)

	// Filter service instances by name
	var results []interface{}
	for _, serviceInstance := range responseCollection {
		if utils.IsFullMatch(name, serviceInstance.Name) {
			results = append(results, serviceInstance)
		}
	}

	return results, nil
}

func (c *CFClient) GetServicePlan(ctx context.Context, guid string) (*v1alpha1.ServicePlanParameters, error) {
	sp, err := c.cf.ServicePlans.Get(ctx, guid)
	if err != nil {
		return nil, fmt.Errorf("failed to get service plan: %w", err)
	}

	// Get service offering details
	so, err := c.cf.ServiceOfferings.Get(ctx, sp.Relationships.ServiceOffering.Data.GUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get service offering: %w", err)
	}

	return &v1alpha1.ServicePlanParameters{
		ID:       &sp.GUID,
		Offering: &so.Name,
		Plan:     &sp.Name,
	}, nil
}

func (c *CFClient) GetServiceCredentials(ctx context.Context, guid string, serviceType string) (*json.RawMessage, error) {
	// Get credentials based on service type
	if serviceType == "managed" {
		params, err := c.cf.ServiceInstances.GetManagedParameters(ctx, guid)
		if err != nil {
			return nil, fmt.Errorf("failed to get managed service parameters: %w", err)
		}
		return params, nil
	} else {
		creds, err := c.cf.ServiceInstances.GetUserProvidedCredentials(ctx, guid)
		if err != nil {
			return nil, fmt.Errorf("failed to get user-provided service credentials: %w", err)
		}
		return creds, nil
	}
}

// getSpaceMembers fetches space members based on the provided filter
func (c *CFClient) getSpaceMembers(ctx context.Context, filter map[string]string) ([]interface{}, error) {
	// Space members require a space filter
	spaceName, ok := filter["space"]
	if !ok {
		return nil, fmt.Errorf("space filter is required for fetching space members")
	}

	typeFilter, ok := filter["role_type"]
	if !ok {
		return nil, fmt.Errorf("role type filter is required for fetching space members")
	}

	spaceRefFilter := cfv3.SpaceListOptions{Names: cfv3.Filter{Values: []string{spaceName}}}
	space, err := c.cf.Spaces.Single(ctx, &spaceRefFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get space: %w", err)
	}

	// Get all roles for the space
	opts := cfv3.NewRoleListOptions()
	opts.SpaceGUIDs.EqualTo(space.GUID)

	roleTypes := []string{
		v1alpha1.SpaceDevelopers,
		v1alpha1.SpaceManagers,
		v1alpha1.SpaceAuditors,
		v1alpha1.SpaceSupporters,
	}
	results := make([]any, 0, len(roleTypes))

	for _, roleType := range roleTypes {
		if !utils.IsFullMatch(typeFilter, roleType) {
			continue
		}

		utils.PrintLine("Fetching space members of ", roleType, 30)
		// Get the space reference
		opts.WithSpaceRoleType(role.SpaceRoleType(roleType))

		_, users, err := c.cf.Roles.ListIncludeUsersAll(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get space roles: %w", err)
		}

		members := make([]*v1alpha1.Member, 0, len(users))

		for _, user := range users {
			members = append(members, &v1alpha1.Member{
				Username: *user.Username,
				Origin:   *user.Origin,
			})
		}

		spaceMembers := &v1alpha1.SpaceMembersParameters{
			SpaceReference: v1alpha1.SpaceReference{Space: &space.GUID, SpaceName: &space.Name},
			RoleType:       roleType,
			MemberList: v1alpha1.MemberList{
				Members: members,
			},
		}
		results = append(results, *spaceMembers)
	}

	return results, nil
}

// getOrgMembers fetches org members based on the provided filter
func (c *CFClient) getOrgMembers(ctx context.Context, filter map[string]string) ([]interface{}, error) {
	// Org members require an org filter
	orgName, ok := filter["org"]
	if !ok {
		return nil, fmt.Errorf("org filter is required for fetching org members")
	}

	typeFilter, ok := filter["role_type"]
	if !ok {
		return nil, fmt.Errorf("role type filter is required for fetching org members")
	}

	orgRefFilter := cfv3.OrganizationListOptions{Names: cfv3.Filter{Values: []string{orgName}}}
	org, err := c.cf.Organizations.Single(ctx, &orgRefFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get org: %w", err)
	}

	// Get all roles for the org
	opts := cfv3.NewRoleListOptions()
	opts.OrganizationGUIDs.EqualTo(org.GUID)

	roleTypes := []string{
		v1alpha1.OrgUsers,
		v1alpha1.OrgManagers,
		v1alpha1.OrgAuditors,
		v1alpha1.OrgBillingManagers,
	}
	results := make([]any, 0, len(roleTypes))

	for _, roleType := range roleTypes {
		if !utils.IsFullMatch(typeFilter, roleType) {
			continue
		}

		utils.PrintLine("Fetching org members of ", roleType, 30)
		// Get the org reference
		opts.WithOrganizationRoleType(role.OrgRoleType(roleType))

		_, users, err := c.cf.Roles.ListIncludeUsersAll(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to get org roles: %w", err)
		}

		members := make([]*v1alpha1.Member, 0, len(users))

		for _, user := range users {
			members = append(members, &v1alpha1.Member{
				Username: *user.Username,
				Origin:   *user.Origin,
			})
		}

		orgMembers := &v1alpha1.OrgMembersParameters{
			OrgReference: v1alpha1.OrgReference{Org: &org.GUID, OrgName: &org.Name},
			RoleType:     roleType,
			MemberList: v1alpha1.MemberList{
				Members: members,
			},
		}
		results = append(results, *orgMembers)
	}

	return results, nil
}

// CFClientAdapter implements the ClientAdapter interface
type CFClientAdapter struct{}

func (a *CFClientAdapter) BuildClient(ctx context.Context, credentials provider.Credentials) (provider.ProviderClient, error) {
	cfCreds, ok := credentials.(*CFCredentials)
	config, err := cfconfig.New(cfCreds.ApiEndpoint, cfconfig.UserPassword(cfCreds.Email, cfCreds.Password))
	kingpin.FatalIfError(err, "%s", errCreateCFConfig)

	if !ok {
		return nil, fmt.Errorf("invalid credentials type")
	}

	// Build CF provider
	cfClientInstance, err := cfv3.New(config)
	if err != nil {
		return nil, err
	}

	return &CFClient{cf: *cfClientInstance}, nil
}

func (a *CFClientAdapter) GetCredentials(ctx context.Context, kubeConfigPath string, providerConfigRef provider.ProviderConfigRef, scheme *runtime.Scheme) (provider.Credentials, error) {
	providerConfig := &v1beta1.ProviderConfig{}

	resourceRef := types.NamespacedName{
		Name:      providerConfigRef.Name,
		Namespace: providerConfigRef.Namespace,
	}

	k8sClient, err := kubernetes.NewK8sClient(kubeConfigPath, scheme)
	kingpin.FatalIfError(err, "%s", errCreateK8sClient)

	// Get the specific ProviderConfig resource and store it in providerConfig
	err = k8sClient.Get(ctx, resourceRef, providerConfig)
	kingpin.FatalIfError(err, "%s", errGetProviderConfig)

	secret := &corev1.Secret{}

	// Get the K8s-Secret and store in secret
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      providerConfig.Spec.Credentials.SecretRef.Name,
		Namespace: providerConfig.Spec.Credentials.SecretRef.Namespace,
	}, secret)
	kingpin.FatalIfError(err, "%s", errGetSecret)

	// Extract and decode the credentials JSON
	credentials, exists := secret.Data[providerConfig.Spec.Credentials.SecretRef.Key]
	if !exists {
		panic(errExtractCredentials)
	}

	// CF Endpoint can be either directly in providerConfig or in a separate secret
	var apiEndpoint string
	if providerConfig.Spec.APIEndpoint != nil {
		// Get the API endpoint from the provider config directly
		apiEndpoint = *providerConfig.Spec.APIEndpoint
	} else {
		// Get the API endpoint from a secret
		apiSecret := &corev1.Secret{}

		// Get the K8s-Secret containing the CF-Endpoint and store in apiSecret
		err = k8sClient.Get(ctx, types.NamespacedName{
			Name:      providerConfig.Spec.Endpoint.SecretRef.Name,
			Namespace: providerConfig.Spec.Endpoint.SecretRef.Namespace,
		}, apiSecret)
		kingpin.FatalIfError(err, "%s", errGetSecret)

		apiEndpointRaw, exists := apiSecret.Data[providerConfig.Spec.Endpoint.SecretRef.Key]
		if !exists {
			panic(errExtractApiEndpoint)
		}
		apiEndpoint = string(apiEndpointRaw)
	}

	var creds CFCredentials
	err = json.Unmarshal(credentials, &creds)
	kingpin.FatalIfError(err, "%s", errUnmarshalCredentials)

	return &CFCredentials{
		ApiEndpoint: apiEndpoint,
		Email:       creds.Email,
		Password:    creds.Password,
	}, nil
}
