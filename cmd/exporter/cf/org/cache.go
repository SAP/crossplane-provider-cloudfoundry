package org

import (
	"github.com/cloudfoundry/go-cfclient/v3/resource"
	cpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
)

type Cache struct {
	guidIndex map[string]*resource.Organization
	nameIndex map[string][]*resource.Organization
}

func New(orgs []*resource.Organization) *Cache {
	c := &Cache{
		guidIndex: make(map[string]*resource.Organization),
		nameIndex: make(map[string][]*resource.Organization),
	}
	for _, org := range orgs {
		c.guidIndex[org.GUID] = org
		c.nameIndex[org.Name] = append(c.nameIndex[org.Name], org)
	}
	return c
}

func (c *Cache) GetByName(name string) ([]*resource.Organization) {
	return c.nameIndex[name]
}

func (c *Cache) GetByGUID(guid string) (*resource.Organization) {
	return c.guidIndex[guid]
}

func (c *Cache) GetGuidsByNames(names []string) ([]string) {
	guids := make([]string, 0)
	for _, name := range names {
		if orgs, ok := c.nameIndex[name]; ok {
			for _, org := range orgs {
				guids = append(guids, org.GUID)
			}
		} else {
			panic("org with name not found in orgDB")
		}
	}
	return guids
}

func (c *Cache) GetNames() ([]string) {
	names := make([]string, len(c.nameIndex))
	i := 0
	for name := range c.nameIndex {
		names[i] = name
		i++
	}
	return names
}

func (c *Cache) GetGUIDs() ([]string) {
	guids := make([]string, len(c.guidIndex))
	i := 0
	for guid := range c.guidIndex {
		guids[i] = guid
		i++
	}
	return guids
}

func (c *Cache) Export(resChan chan<- cpresource.Object) {
	for _, org := range c.guidIndex {
		resChan <- convertOrgResource(org)
	}
}
