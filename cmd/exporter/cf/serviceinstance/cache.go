package serviceinstance

import (
	"context"
	"sync"

	"github.com/SAP/crossplane-provider-cloudfoundry/internal/exporttool/cli/export"

	"github.com/cloudfoundry/go-cfclient/v3/client"
	"github.com/cloudfoundry/go-cfclient/v3/resource"
)

type Cache struct {
	guidIndex map[string]*resource.ServiceInstance
}

func New(serviceInstances []*resource.ServiceInstance) *Cache {
	c := &Cache{
		guidIndex: make(map[string]*resource.ServiceInstance),
	}
	for _, si := range serviceInstances {
		c.guidIndex[si.GUID] = si
	}
	return c
}

func (c *Cache) GetByGUID(guid string) *resource.ServiceInstance {
	return c.guidIndex[guid]
}

func (c *Cache) Export(ctx context.Context, cfClient *client.Client, evHandler export.EventHandler) {
	wg := sync.WaitGroup{}
	tokenChan := make(chan struct{}, 10)
	for _, serviceInstance := range c.guidIndex {
		wg.Add(1)
		tokenChan <- struct{}{}
		go func() {
			defer wg.Done()
			si := convertServiceInstanceResource(ctx, cfClient, serviceInstance, evHandler)
			if si != nil {
				evHandler.Resource(si)
			}
			<-tokenChan
		}()
	}
	wg.Wait()
}
