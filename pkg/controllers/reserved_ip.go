package controllers

import (
	"context"
	"fmt"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	klog "k8s.io/klog/v2"
)

func (c *NodeController) syncReservedIP(nodeName string) error {
	if len(c.reservedIPs) == 0 {
		return nil
	}

	klog.Infof("adding a reserved IP on node %s", nodeName)

	_, exists, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}

	if !exists {
		// ip will be detached on delete
		klog.Infof("node %s was deleted, ignoring", nodeName)
		return nil
	}

	server, err := c.getInstanceFromNodeName(nodeName)
	if err != nil {
		klog.Errorf("could not get server %s: %v", nodeName, err)
		return err
	}

	if server.PublicIP == nil {
		klog.Warning("node %s does not have a public IP")
		return nil
	}

	if !server.PublicIP.Dynamic {
		klog.Warningf("node %s already have a public IP", nodeName)
		err = c.addReservedIPLabel(nodeName)
		if err != nil {
			return err
		}

		return nil
	}

	ip, err := c.getFreeIP()
	if err != nil {
		klog.Errorf("could not get a free IP for node %s: %v", nodeName, err)
		return err
	}

	if ip == nil {
		klog.Warningf("no available reserved IPs for node %s", nodeName)
		return nil
	}

	instanceAPI := instance.NewAPI(c.scwClient)

	_, err = instanceAPI.UpdateIP(&instance.UpdateIPRequest{
		IP: ip.ID,
		Server: &instance.NullableStringValue{
			Value: server.ID,
		},
	})
	if err != nil {
		klog.Errorf("could not attach IP %s for node %s: %v", ip.ID, nodeName, err)
		return err
	}

	err = c.addReservedIPLabel(nodeName)
	if err != nil {
		return err
	}

	return nil
}

func (c *NodeController) addReservedIPLabel(nodeName string) error {
	nodeObj, _, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}

	node, ok := nodeObj.(*v1.Node)
	if !ok {
		klog.Errorf("could not get node %s from obejct", nodeName)
		return fmt.Errorf("could not get node %s from obejct", nodeName)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	if value, ok := node.Labels[NodeLabelReservedIP]; ok && value == "true" {
		return nil
	}

	node.Labels[NodeLabelReservedIP] = "true"

	_, err = c.clientset.CoreV1().Nodes().Update(context.Background(), node, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("could not add reserved IP label to node %s: %v", nodeName, err)
		return err
	}
	return nil
}
