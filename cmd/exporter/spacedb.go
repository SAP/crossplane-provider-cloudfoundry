package main

import (
	"context"
	"sync"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

type spaceDB struct {
	sync.RWMutex
	collected    bool
	cfClient     *client.Client
	guidIndex    map[string]*resource.Space
	nameIndex    map[string][]*resource.Space
	orgGuidIndex map[string][]*resource.Space
}

func newSpaceDB(cfClient *client.Client) *spaceDB {
	return &spaceDB{
		cfClient:     cfClient,
		collected:    false,
		guidIndex:    make(map[string]*resource.Space),
		nameIndex:    make(map[string][]*resource.Space),
		orgGuidIndex: make(map[string][]*resource.Space),
	}
}

func (db *spaceDB) collectSpaces() error {
	db.Lock()
	defer db.Unlock()
	db.guidIndex = make(map[string]*resource.Space)
	db.nameIndex = make(map[string][]*resource.Space)
	db.orgGuidIndex = make(map[string][]*resource.Space)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	spaces, err := getSpaces(ctx, db.cfClient)
	if err != nil {
		return erratt.Errorf("cannot collect spaces: %w", err)
	}

	for _, space := range spaces {
		db.guidIndex[space.GUID] = space
		db.nameIndex[space.Name] = append(db.nameIndex[space.Name], space)
		db.orgGuidIndex[space.Relationships.Organization.Data.GUID] = append(db.orgGuidIndex[space.Relationships.Organization.Data.GUID], space)
	}
	db.collected = true
	return nil
}

func (db *spaceDB) getByName(name string) ([]*resource.Space, error) {
	if !db.collected {
		if err := db.collectSpaces(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	return db.nameIndex[name], nil
}

func (db *spaceDB) getByGUID(guid string) (*resource.Space, error) {
	if !db.collected {
		if err := db.collectSpaces(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	return db.guidIndex[guid], nil
}

func (db *spaceDB) getByOrgGUID(guid string) ([]*resource.Space, error) {
	if !db.collected {
		if err := db.collectSpaces(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	return db.orgGuidIndex[guid], nil
}

func (db *spaceDB) getByOrgGUIDs(guids []string) ([]*resource.Space, error) {
	if !db.collected {
		if err := db.collectSpaces(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	spaces := make([]*resource.Space, 0)
	for _, guid := range guids {
		spaces = append(spaces, db.orgGuidIndex[guid]...)
	}
	return spaces, nil
}

type key int

var orgGuidsKey key

func (db *spaceDB) getNamesByOrgGUIDs(ctx context.Context) ([]string, error) {
	orgGuids, ok := ctx.Value(orgGuidsKey).([]string)
	if !ok {
		panic("spaceDB.getNamesByOrgGUIDs requires that ctx value with key orgGuidsKey is set")
	}
	spaces, err := db.getByOrgGUIDs(orgGuids)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(spaces))
	for i, space := range spaces {
		names[i] = space.Name
	}
	return names, nil
}

func (db *spaceDB) getGuidsByNames(names []string) ([]string, error) {
	if !db.collected {
		if err := db.collectSpaces(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	guids := make([]string, 0)
	for _, name := range names {
		if orgs, ok := db.nameIndex[name]; ok {
			for _, org := range orgs {
				guids = append(guids, org.GUID)
			}
		} else {
			return nil, erratt.New("space with name not found in spaceDB", "name", name)
		}
	}
	return guids, nil
}
