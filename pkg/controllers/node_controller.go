package controllers

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scaleway/scaleway-sdk-go/scw"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	klog "k8s.io/klog/v2"
)

const (
	ReverseIPDomainEnv      = "REVERSE_IP_DOMAIN"
	DatabaseIDsEnv          = "DATABASE_IDS"
	ReservedIPsPoolEnv      = "RESERVED_IPS_POOL"
	SecurityGroupIDs        = "SECURITY_GROUP_IDS"
	NumberRetries           = "NUMBER_RETRIES"
	NodesIPSource           = "NODES_IP_SOURCE"
	NodesIPSourceKubernetes = "kubernetes"
	NodeLabelReservedIP     = "reserved-ip"
)

func NewNodeController(clientset *kubernetes.Clientset) (*NodeController, error) {
	nodeListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "nodes", "", fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(nodeListWatcher, &v1.Node{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {

				oldNode, oldOk := old.(*v1.Node)
				newNode, newOk := new.(*v1.Node)
				if oldOk && newOk {
					if oldNode.ResourceVersion == newNode.ResourceVersion {
						queue.Add(key)
						return
					}
					for _, oldAddress := range oldNode.Status.Addresses {
						for _, newAddress := range newNode.Status.Addresses {
							if oldAddress.Type == newAddress.Type && oldAddress.Address != newAddress.Address {
								queue.Add(key)
								return
							}
						}
					}
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				queue.Add(key)
			}
		},
	}, cache.Indexers{})

	scwClient, err := scw.NewClient(scw.WithEnv())
	if err != nil {
		return nil, err
	}

	controller := &NodeController{
		indexer:       indexer,
		informer:      informer,
		queue:         queue,
		scwClient:     scwClient,
		numberRetries: defaultNumberRetries,
		clientset:     clientset,
	}

	// TODO handle validation here ?
	if os.Getenv(ReverseIPDomainEnv) != "" {
		controller.reverseIPDomain = os.Getenv(ReverseIPDomainEnv)
	}

	if os.Getenv(DatabaseIDsEnv) != "" {
		controller.databaseIDs = strings.Split(os.Getenv(DatabaseIDsEnv), ",")
	}

	if os.Getenv(ReservedIPsPoolEnv) != "" {
		controller.reservedIPs = strings.Split(os.Getenv(ReservedIPsPoolEnv), ",")
	}

	if os.Getenv(SecurityGroupIDs) != "" {
		controller.securityGroupIDs = strings.Split(os.Getenv(SecurityGroupIDs), ",")
	}

	if os.Getenv(NumberRetries) != "" {
		numberRetriesValue, err := strconv.Atoi(os.Getenv(NumberRetries))
		controller.numberRetries = numberRetriesValue
		if err != nil {
			klog.Errorf("could not parse the desired number of retries %s: %v", os.Getenv(NumberRetries), err)
			controller.numberRetries = defaultNumberRetries
		}
	}

	return controller, nil
}

func (c *NodeController) syncNeeded(nodeName string) error {
	var errs []error

	err := c.syncReservedIP(nodeName)
	if err != nil {
		klog.Errorf("failed to sync reserved IP for node %s: %v", nodeName, err)
		errs = append(errs, err)
	}
	err = c.syncReverseIP(nodeName)
	if err != nil {
		klog.Errorf("failed to sync reverse IP for node %s: %v", nodeName, err)
		errs = append(errs, err)
	}
	err = c.syncDatabaseACLs(nodeName)
	if err != nil {
		klog.Errorf("failed to sync database acl for node %s: %v", nodeName, err)
		errs = append(errs, err)
	}
	err = c.syncSecurityGroup(nodeName)
	if err != nil {
		klog.Errorf("failed to sync security group for node %s: %v", nodeName, err)
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("got several error")
}

func (c *NodeController) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncNeeded(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *NodeController) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < c.numberRetries {
		c.queue.AddRateLimited(key)
		return
	}

	c.queue.Forget(key)
	runtime.HandleError(err)
	klog.Infof("too many retries for key %s: %v", key, err)
}

func (c *NodeController) Run(stopCh chan struct{}) {
	defer runtime.HandleCrash()
	defer c.Wg.Done()

	defer c.queue.ShutDown()

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *NodeController) runWorker() {
	for c.processNextItem() {
	}
}
