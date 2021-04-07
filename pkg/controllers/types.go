package controllers

import (
	"sync"

	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	defaultNumberRetries = 30
)

type NodeController struct {
	Wg sync.WaitGroup

	clientset kubernetes.Interface
	indexer   cache.Indexer
	queue     workqueue.RateLimitingInterface
	informer  cache.Controller

	scwClient *scw.Client

	reverseIPDomain  string
	databaseIDs      []string
	reservedIPs      []string
	securityGroupIDs []string

	numberRetries int
}

type SvcController struct {
	Wg sync.WaitGroup

	indexer  cache.Indexer
	queue    workqueue.RateLimitingInterface
	informer cache.Controller

	scwClient *scw.Client

	securityGroupIDs []string

	numberRetries int
}
