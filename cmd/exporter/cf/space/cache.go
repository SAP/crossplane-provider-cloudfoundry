package space

import (
	"context"

	"github.com/cloudfoundry/go-cfclient/v3/resource"
	cpresource "github.com/crossplane/crossplane-runtime/pkg/resource"
)

type Cache struct {
	// sync.RWMutex
	// collected    bool
	// cfClient     *client.Client
	orgGuids     []string
	guidIndex    map[string]*resource.Space
	nameIndex    map[string][]*resource.Space
	orgGuidIndex map[string][]*resource.Space
}

func New(spaces []*resource.Space) *Cache {
	c := &Cache{
		// cfClient:     cfClient,
		// orgGuids:     orgGuids,
		// collected:    false,
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

// func (db *Cache) collectSpaces() error {
// 	db.Lock()
// 	defer db.Unlock()
// 	db.guidIndex = make(map[string]*resource.Space)
// 	db.nameIndex = make(map[string][]*resource.Space)
// 	db.orgGuidIndex = make(map[string][]*resource.Space)
// 	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
// 	defer cancel()

// 	spaces, err := getSpaces(ctx, db.cfClient, []string{})
// 	if err != nil {
// 		return erratt.Errorf("cannot collect spaces: %w", err)
// 	}

// 	for _, space := range spaces {
// 		db.guidIndex[space.GUID] = space
// 		db.nameIndex[space.Name] = append(db.nameIndex[space.Name], space)
// 		db.orgGuidIndex[space.Relationships.Organization.Data.GUID] = append(db.orgGuidIndex[space.Relationships.Organization.Data.GUID], space)
// 	}
// 	db.collected = true
// 	return nil
// }

func (c *Cache) GetByName(name string) []*resource.Space {
	return c.nameIndex[name]
}

func (c *Cache) GetByGUID(guid string) *resource.Space {
	// if !db.collected {
	// 	if err := db.collectSpaces(); err != nil {
	// 		return nil, err
	// 	}
	// }
	// db.RLock()
	// defer db.RUnlock()
	return c.guidIndex[guid]
}

func (c *Cache) GetByOrgGUID(guid string) []*resource.Space {
	// if !db.collected {
	// 	if err := db.collectSpaces(); err != nil {
	// 		return nil, err
	// 	}
	// }
	// db.RLock()
	// defer db.RUnlock()
	return c.orgGuidIndex[guid]
}

func (c *Cache) GetByOrgGUIDs(guids []string) []*resource.Space {
	// if !db.collected {
	// 	if err := db.collectSpaces(); err != nil {
	// 		return nil, err
	// 	}
	// }
	// db.RLock()
	// defer db.RUnlock()
	spaces := make([]*resource.Space, 0)
	for _, guid := range guids {
		spaces = append(spaces, c.orgGuidIndex[guid]...)
	}
	return spaces
}

type key int

var orgGuidsKey key

func (c *Cache) GetNamesByOrgGUIDs(ctx context.Context) []string {
	orgGuids, ok := ctx.Value(orgGuidsKey).([]string)
	if !ok {
		panic("spaceDB.getNamesByOrgGUIDs requires that ctx value with key orgGuidsKey is set")
	}
	spaces := c.GetByOrgGUIDs(orgGuids)
	names := make([]string, len(spaces))
	for i, space := range spaces {
		names[i] = space.Name
	}
	return names
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

func (c *Cache) Export(resChan chan<- cpresource.Object) {
	for _, space := range c.guidIndex {
		resChan <- convertSpaceResource(space)
	}
}
