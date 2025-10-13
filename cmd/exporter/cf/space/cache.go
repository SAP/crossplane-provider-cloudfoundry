package space

import (
	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

type Cache struct {
	guidIndex    map[string]*resource.Space
	nameIndex    map[string][]*resource.Space
	orgGuidIndex map[string][]*resource.Space
}

func New(spaces []*resource.Space) *Cache {
	c := &Cache{
		guidIndex:    make(map[string]*resource.Space),
		nameIndex:    make(map[string][]*resource.Space),
		orgGuidIndex: make(map[string][]*resource.Space),
	}
	for _, space := range spaces {
		c.guidIndex[space.GUID] = space
		c.nameIndex[space.Name] = append(c.nameIndex[space.Name], space)
		c.orgGuidIndex[space.Relationships.Organization.Data.GUID] = append(c.orgGuidIndex[space.Relationships.Organization.Data.GUID], space)
	}
	return c
}

func (c *Cache) GetByName(name string) []*resource.Space {
	return c.nameIndex[name]
}

func (c *Cache) GetByGUID(guid string) *resource.Space {
	return c.guidIndex[guid]
}

func (c *Cache) GetByOrgGUID(guid string) []*resource.Space {
	return c.orgGuidIndex[guid]
}

func (c *Cache) GetByOrgGUIDs(guids []string) []*resource.Space {
	spaces := make([]*resource.Space, 0)
	for _, guid := range guids {
		spaces = append(spaces, c.orgGuidIndex[guid]...)
	}
	return spaces
}

func (c *Cache) GetGuidsByNames(names []string) []string {
	guids := make([]string, 0)
	for _, name := range names {
		if orgs, ok := c.nameIndex[name]; ok {
			for _, org := range orgs {
				guids = append(guids, org.GUID)
			}
		} else {
			panic("space with name not found in spaceDB")
		}
	}
	return guids
}

func (c *Cache) GetNames() []string {
	names := make([]string, len(c.nameIndex))
	i := 0
	for name := range c.nameIndex {
		names[i] = name
		i++
	}
	return names
}

func (c *Cache) Export(evHandler export.EventHandler) {
	for _, space := range c.guidIndex {
		evHandler.Resource(convertSpaceResource(space))
	}
}
