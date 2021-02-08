package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sh4d1/scaleway-k8s-node-coffee/pkg/controllers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	klog "k8s.io/klog/v2"
)

var (
	kubeconfig string
	masterURL  string
)

func init() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", os.Getenv("KUBECONFIG"), "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", os.Getenv("MASTER_URL"), "URL of the Kubernetes API server. Optional")
}

func main() {
	flag.Parse()
	klog.Infof("Collecting coffee beans")

	restConfig, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubeconfig for shoot: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Fatalf("could not build kubernetes clientset: %v", err)
	}

	nodeController, err := controllers.NewNodeController(clientset)
	if err != nil {
		klog.Fatalf("could not create node controller: %v", err)
	}
	svcController, err := controllers.NewSvcController(clientset)
	if err != nil {
		klog.Fatalf("could not create svc controller: %v", err)
	}

	stop := make(chan struct{})
	klog.Infof("Starting the coffee machine")
	nodeController.Wg.Add(1)
	go nodeController.Run(stop)
	svcController.Wg.Add(1)
	go svcController.Run(stop)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT|syscall.SIGTERM)
	<-c
	klog.Infof("Stopping the coffee machine")
	nodeController.Wg.Wait()
	svcController.Wg.Wait()
	close(stop)
}
