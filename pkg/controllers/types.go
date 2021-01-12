package controllers

import (
	"sync"

	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	Wg sync.WaitGroup

	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller

	scwClient *scw.Client

	reverseIPDomain string
	databaseIDs     []string
	reservedIPs     []string

	numberRetries int
}
