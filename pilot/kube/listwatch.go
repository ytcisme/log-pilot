package kube

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

type ListWatchCache struct {
	indexer  cache.Indexer
	informer cache.Controller
}

func NewListWatchCache(listWatcher cache.ListerWatcher, objType runtime.Object) (*ListWatchCache, error) {
	return NewListWatchCacheWithEventHandler(listWatcher, objType, cache.ResourceEventHandlerFuncs{})
}

func NewListWatchCacheWithEventHandler(listWatcher cache.ListerWatcher, objType runtime.Object,
	evHandler cache.ResourceEventHandler) (*ListWatchCache, error) {
	if listWatcher == nil {
		return nil, fmt.Errorf("nil ListerWatcher for ListWatchCache")
	}
	if objType == nil {
		return nil, fmt.Errorf("nil runtime.Object for type")
	}
	indexer, informer := cache.NewIndexerInformer(listWatcher, objType, 0,
		evHandler, cache.Indexers{})
	return &ListWatchCache{
		indexer:  indexer,
		informer: informer,
	}, nil
}

func (c *ListWatchCache) Run(stopCh <-chan struct{}) error {
	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}
	return nil
}

func (c *ListWatchCache) Get(key string) (item interface{}, exists bool, err error) {
	return c.indexer.GetByKey(key)
}

func (c *ListWatchCache) GetInNamespace(namespace, key string) (item interface{}, exists bool, err error) {
	return c.indexer.GetByKey(namespace + "/" + key)
}

func (c *ListWatchCache) List() (items []interface{}) {
	return c.indexer.List()
}
