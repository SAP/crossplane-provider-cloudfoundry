package main

import (
	"context"
	"sync"
	"time"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/erratt"
	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

type orgDB struct {
	sync.RWMutex
	collected bool
	cfClient  *client.Client
	guidIndex map[string]*resource.Organization
	nameIndex map[string][]*resource.Organization
}

func newOrgDB(cfClient *client.Client) *orgDB {
	return &orgDB{
		cfClient: cfClient,
		collected: false,
		guidIndex: make(map[string]*resource.Organization),
		nameIndex: make(map[string][]*resource.Organization),
	}
}

func (db *orgDB) collectOrgs() error {
	db.Lock()
	defer db.Unlock()
	db.guidIndex = make(map[string]*resource.Organization)
	db.nameIndex = make(map[string][]*resource.Organization)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	orgs, err := getOrganizations(ctx, db.cfClient)
	if err != nil {
		return erratt.Errorf("cannot collect organizations: %w", err)
	}
	for _, org := range orgs {
		db.guidIndex[org.GUID] = org
		db.nameIndex[org.Name] = append(db.nameIndex[org.Name], org)
	}
	db.collected = true
	return nil
}

func (db *orgDB) getByName(name string) ([]*resource.Organization, error) {
	if !db.collected {
		if err := db.collectOrgs(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	return db.nameIndex[name], nil
}

func (db *orgDB) getByGUID(guid string) (*resource.Organization, error) {
	if !db.collected {
		if err := db.collectOrgs(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	return db.guidIndex[guid], nil
}

func (db *orgDB) getGuidsByNames(names []string) ([]string, error) {
	if !db.collected {
		if err := db.collectOrgs(); err != nil {
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
			return nil, erratt.New("org with name not found in orgDB", "name", name)
		}
	}
	return guids, nil
}

func (db *orgDB) getNames(_ context.Context) ([]string, error) {
	if !db.collected {
		if err := db.collectOrgs(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	names := make([]string, len(db.nameIndex))
	i := 0
	for name := range db.nameIndex {
		names[i] = name
		i++
	}
	return names, nil
}

func (db *orgDB) getGUIDs() ([]string, error) {
	if !db.collected {
		if err := db.collectOrgs(); err != nil {
			return nil, err
		}
	}
	db.RLock()
	defer db.RUnlock()
	guids := make([]string, len(db.guidIndex))
	i := 0
	for guid := range db.guidIndex {
		guids[i] = guid
		i++
	}
	return guids, nil
}
