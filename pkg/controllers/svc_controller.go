package controllers

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	klog "k8s.io/klog/v2"
)

func NewSvcController(clientset *kubernetes.Clientset) (*SvcController, error) {
	svcListWatcher := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "services", "", fields.Everything())

	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	indexer, informer := cache.NewIndexerInformer(svcListWatcher, &v1.Service{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				if svc, ok := obj.(*v1.Service); ok && isPublicSvc(svc) {
					queue.Add(key)
				}
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				newSvc, newOk := new.(*v1.Service)
				if newOk && isPublicSvc(newSvc) {
					queue.Add(key)
				}
			}
		},
	}, cache.Indexers{})

	scwClient, err := scw.NewClient(scw.WithEnv())
	if err != nil {
		return nil, err
	}

	controller := &SvcController{
		indexer:   indexer,
		informer:  informer,
		queue:     queue,
		scwClient: scwClient,
	}

	// TODO handle validation here ?
	if os.Getenv(SecurityGroupIDs) != "" {
		controller.securityGroupIDs = strings.Split(os.Getenv(SecurityGroupIDs), ",")
	}

	return controller, nil
}

func (c *SvcController) syncNeeded(nodeName string) error {
	var errs []error

	err := c.syncSecurityGroup(nodeName)
	if err != nil {
		klog.Errorf("failed to sync security group for node %s: %v", nodeName, err)
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("got several error")
}

func (c *SvcController) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncNeeded(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *SvcController) handleErr(err error, key interface{}) {
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

func (c *SvcController) Run(stopCh chan struct{}) {
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

func (c *SvcController) runWorker() {
	for c.processNextItem() {
	}
}
