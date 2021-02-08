package controllers

import (
	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
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
		klog.Warning("node %s already have a public IP")
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

	return nil
}
