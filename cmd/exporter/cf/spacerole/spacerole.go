package spacerole

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
	SpaceRole = spaceRole{}
)

func init() {
	resources.RegisterKind(SpaceRole)
}

type spaceRole struct{}

var _ resources.Kind = spaceRole{}

func (sr spaceRole) Param() configparam.ConfigParam {
	return nil
}

func (sr spaceRole) KindName() string {
	return "spacerole"
}

func (sr spaceRole) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler, resolveReferences bool) error {
	spaceRoles, _, err := userrole.GetSpaceRoles(ctx, cfClient)
	if err != nil {
		return err
	}
	if spaceRoles.Len() == 0 {
		evHandler.Warn(erratt.New("no spacerole found"))
	} else {
		for _, spaceRole := range spaceRoles.AllByGUIDs() {
			evHandler.Resource(convertSpaceRoleResource(ctx, cfClient, spaceRole, evHandler, resolveReferences))
		}
	}
	return nil
}
