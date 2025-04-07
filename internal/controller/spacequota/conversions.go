package spacequota

import (
	"slices"
	"strings"
	"time"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	"k8s.io/utils/ptr"

	"github.com/SAP/crossplane-provider-cloudfoundry/apis/resources/v1alpha1"
)

// GenerateSpaceQuota returns the current state in the form of
// *v1alpha1.SpaceQuota.
//
//nolint:gocyclo
func GenerateSpaceQuota(resp *resource.SpaceQuota) *v1alpha1.SpaceQuota {
	cr := &v1alpha1.SpaceQuota{}
	if v := resp.Apps.LogRateLimitInBytesPerSecond; v != nil {
		cr.Status.AtProvider.TotalAppLogRateLimit = ptr.To(float64(*v))
	}
	if v := resp.Apps.PerAppTasks; v != nil {
		cr.Status.AtProvider.TotalAppTasks = ptr.To(float64(*v))
	}
	if v := resp.Apps.PerProcessMemoryInMB; v != nil {
		cr.Status.AtProvider.InstanceMemory = ptr.To(float64(*v))
	}
	if v := resp.Apps.TotalInstances; v != nil {
		cr.Status.AtProvider.TotalAppInstances = ptr.To(float64(*v))
	}
	if v := resp.Apps.TotalMemoryInMB; v != nil {
		cr.Status.AtProvider.TotalMemory = ptr.To(float64(*v))
	}
	cr.Status.AtProvider.CreatedAt = ptr.To(resp.CreatedAt.Format(time.RFC3339))
	cr.Status.AtProvider.Name = ptr.To(resp.Name)
	cr.Status.AtProvider.ID = ptr.To(resp.GUID)
	if resp.Relationships.Organization != nil && resp.Relationships.Organization.Data != nil {
		cr.Status.AtProvider.Org = &resp.Relationships.Organization.Data.GUID
	}
	if resp.Relationships.Spaces != nil {
		if l := len(resp.Relationships.Spaces.Data); l != len(cr.Status.AtProvider.Spaces) {
			cr.Status.AtProvider.Spaces = make([]*string, l)
		}
		for i := range resp.Relationships.Spaces.Data {
			cr.Status.AtProvider.Spaces[i] = &resp.Relationships.Spaces.Data[i].GUID
		}
		slices.SortFunc(cr.Status.AtProvider.Spaces, func(a, b *string) int {
			if a != nil && b != nil {
				return strings.Compare(*a, *b)
			}
			return 0
		})
	}
	if v := resp.Routes.TotalReservedPorts; v != nil {
		cr.Status.AtProvider.TotalRoutePorts = ptr.To(float64(*v))
	}
	if v := resp.Routes.TotalRoutes; v != nil {
		cr.Status.AtProvider.TotalRoutes = ptr.To(float64(*v))
	}
	if v := resp.Services.PaidServicesAllowed; v != nil {
		cr.Status.AtProvider.AllowPaidServicePlans = v
	}
	if v := resp.Services.TotalServiceInstances; v != nil {
		cr.Status.AtProvider.TotalServices = ptr.To(float64(*v))
	}
	if v := resp.Services.TotalServiceKeys; v != nil {
		cr.Status.AtProvider.TotalServiceKeys = ptr.To(float64(*v))
	}
	cr.Status.AtProvider.UpdatedAt = ptr.To(resp.UpdatedAt.Format(time.RFC3339))
	return cr
}

// GenerateCreateSpaceQuota returns a create input.
//
//nolint:gocyclo
func GenerateCreateSpaceQuota(cr *v1alpha1.SpaceQuota) *resource.SpaceQuotaCreateOrUpdate {
	if cr == nil {
		return nil
	}
	spec := &cr.Spec.ForProvider
	if spec.Name == nil || spec.Org == nil {
		return nil
	}
	res := resource.NewSpaceQuotaCreate(*spec.Name, *spec.Org)
	if v := spec.TotalAppLogRateLimit; v != nil {
		res = res.WithLogRateLimitInBytesPerSecond(int(*v))
	}

	if v := spec.AllowPaidServicePlans; v != nil {
		res = res.WithPaidServicesAllowed(*v)
	}
	if v := spec.TotalAppTasks; v != nil {
		res = res.WithPerAppTasks(int(*v))
	}
	if v := spec.InstanceMemory; v != nil {
		res = res.WithPerProcessMemoryInMB(int(*v))
	}
	if v := spec.Spaces; v != nil {
		if res.Relationships.Spaces == nil {
			// workaround a bug in cfclient
			res.Relationships.Spaces = &resource.ToManyRelationships{}
		}
		spaces := []string{}
		for i := range v {
			if space := spec.Spaces[i]; space != nil {
				spaces = append(spaces, *space)
			}
		}
		res = res.WithSpaces(spaces...)
	}
	if v := spec.TotalAppInstances; v != nil {
		res = res.WithTotalInstances(int(*v))
	}
	if v := spec.TotalMemory; v != nil {
		res = res.WithTotalMemoryInMB(int(*v))
	}
	if v := spec.TotalRoutePorts; v != nil {
		res = res.WithTotalReservedPorts(int(*v))
	}
	if v := spec.TotalRoutes; v != nil {
		res = res.WithTotalRoutes(int(*v))
	}
	if v := spec.TotalServices; v != nil {
		res = res.WithTotalServiceInstances(int(*v))
	}
	if v := spec.TotalServiceKeys; v != nil {
		res = res.WithTotalServiceKeys(int(*v))
	}
	return res
}

// GenerateUpdateSpaceQuota returns a create input.
//
//nolint:gocyclo
func GenerateUpdateSpaceQuota(cr *v1alpha1.SpaceQuota) *resource.SpaceQuotaCreateOrUpdate {
	if cr == nil {
		return nil
	}
	spec := &cr.Spec.ForProvider
	if spec.Name == nil {
		return nil
	}
	res := resource.NewSpaceQuotaUpdate()
	res.WithName(*spec.Name)
	if v := spec.TotalAppLogRateLimit; v != nil {
		res = res.WithLogRateLimitInBytesPerSecond(int(*v))
	}

	if v := spec.AllowPaidServicePlans; v != nil {
		res = res.WithPaidServicesAllowed(*v)
	}
	if v := spec.TotalAppTasks; v != nil {
		res = res.WithPerAppTasks(int(*v))
	}
	if v := spec.InstanceMemory; v != nil {
		res = res.WithPerProcessMemoryInMB(int(*v))
	}
	if v := spec.TotalAppInstances; v != nil {
		res = res.WithTotalInstances(int(*v))
	}
	if v := spec.TotalMemory; v != nil {
		res = res.WithTotalMemoryInMB(int(*v))
	}
	if v := spec.TotalRoutePorts; v != nil {
		res = res.WithTotalReservedPorts(int(*v))
	}
	if v := spec.TotalRoutes; v != nil {
		res = res.WithTotalRoutes(int(*v))
	}
	if v := spec.TotalServices; v != nil {
		res = res.WithTotalServiceInstances(int(*v))
	}
	if v := spec.TotalServiceKeys; v != nil {
		res = res.WithTotalServiceKeys(int(*v))
	}
	return res
}

// spaceStatus type is a helper type for comparing the expected and
// observer Spaces assigned to a SpaceQuota. A given space can be
// specified in the spec field (expected) or in the status field
// (observed).
type spaceStatus struct {
	// A given space is specified in the Spec.ForProvider field.
	inSpec bool
	// A given space is specified in the Status.AtProvider field.
	inStatus bool
}

// spaceStatuses type maps a spaceStatus to Spaces identified by the
// ID of a Space.
type spaceStatuses map[string]spaceStatus

// toCreate method of spaceStatus collects the IDs of the Spaces
// that are to be created. These are the spaces that are inSpec but
// not inStatus, that is they are expected but not observed.
func (ss spaceStatuses) toCreate() []string {
	result := []string{}
	for guid, status := range ss {
		if status.inSpec && !status.inStatus {
			result = append(result, guid)
		}
	}
	return result
}

// toDelete method of spaceStatus collects the IDs of the Spaces that
// are to be delete. These are the spaces that are not inSpec but
// inStatus, that is they are not expected but observed.
func (ss spaceStatuses) toDelete() []string {
	result := []string{}
	for guid, status := range ss {
		if !status.inSpec && status.inStatus {
			result = append(result, guid)
		}
	}
	return result
}

// getSpaceStatusHelper function generates the spaceStatuses type
// based on the ID of the spaces which are expected (specSpaces) and
// which are observed (statusSpaces).
func getSpaceStatusHelper(specSpaces, statusSpaces []*string) spaceStatuses {
	result := spaceStatuses{}
	for i := range specSpaces {
		if sp := specSpaces[i]; sp != nil {
			result[*sp] = struct {
				inSpec   bool
				inStatus bool
			}{
				inSpec: true,
			}
		}
	}
	for i := range statusSpaces {
		if sp := statusSpaces[i]; sp != nil {
			sStatus, found := result[*sp]
			if found {
				sStatus = spaceStatus{
					inSpec:   sStatus.inSpec,
					inStatus: true,
				}
			} else {
				sStatus = spaceStatus{
					inStatus: true,
				}
			}
			result[*sp] = sStatus
		}
	}
	return result
}

// getSpaceStatus function returns the spaceStatuses type for a given
// SpaceQuote managed resource.
func getSpaceStatus(cr *v1alpha1.SpaceQuota) spaceStatuses {
	return getSpaceStatusHelper(cr.Spec.ForProvider.Spaces, cr.Status.AtProvider.Spaces)
}
