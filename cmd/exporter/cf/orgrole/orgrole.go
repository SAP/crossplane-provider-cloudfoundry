package orgrole

import (
	"context"

	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/resources"
	"github.com/SAP/crossplane-provider-cloudfoundry/cmd/exporter/cf/userrole"

	"github.com/SAP/xp-clifford/cli/configparam"
	"github.com/SAP/xp-clifford/cli/export"
	"github.com/SAP/xp-clifford/erratt"
	"github.com/cloudfoundry/go-cfclient/v3/client"
)

var (
	OrgRole = orgRole{}
)

func init() {
	resources.RegisterKind(OrgRole)
}

type orgRole struct{}

var _ resources.Kind = orgRole{}

func (om orgRole) Param() configparam.ConfigParam {
	return nil
}

func (om orgRole) KindName() string {
	return "orgrole"
}

func (om orgRole) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	orgRoles, _, err := userrole.GetOrgRoles(ctx, cfClient)
	if err != nil {
		return err
	}
	if orgRoles.Len() == 0 {
		evHandler.Warn(erratt.New("no orgrole found"))
	} else {
		for _, orgRole := range orgRoles.AllByGUIDs() {
			evHandler.Resource(convertOrgRoleResource(ctx, cfClient, orgRole, evHandler, resolveReferences))
		}
	}
	return nil
}
