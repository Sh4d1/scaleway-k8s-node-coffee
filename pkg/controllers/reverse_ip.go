package controllers

import (
	"fmt"
	"net"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	klog "k8s.io/klog/v2"
)

func (c *Controller) syncReverseIP(nodeName string) error {
	if c.reverseIPDomain == "" {
		return nil
	}

	klog.Infof("adding a reverse for IP on node %s", nodeName)

	_, exists, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}

	if !exists {
		klog.Infof("node %s was deleted, ignoring", nodeName)
		return nil
	}

	server, err := c.getInstanceFromNodeName(nodeName)
	if err != nil {
		klog.Errorf("could not get server %s: %v", nodeName, err)
		return err
	}

	instanceAPI := instance.NewAPI(c.scwClient)

	if server.PublicIP == nil {
		klog.Warning("node %s does not have a public IP")
		return nil
	}

	if server.PublicIP.Dynamic {
		klog.Warningf("can't update the reverse of a dynamic IP for node %s", nodeName)
		return nil
	}

	_, err = instanceAPI.UpdateIP(&instance.UpdateIPRequest{
		IP: server.PublicIP.Address.String(),
		Reverse: &instance.NullableStringValue{
			Value: fmt.Sprintf("%s.%s", getReversePrefix(server.PublicIP.Address), c.reverseIPDomain),
		},
	})
	if err != nil {
		klog.Errorf("could not update reverse on IP %s for node %s: %v", server.PublicIP.Address.String(), nodeName, err)
		return err
	}

	return nil
}

func getReversePrefix(ip net.IP) string {
	ip = ip.To4()
	// TODO better handling, error prone
	return fmt.Sprintf("%d-%d-%d-%d", ip[3], ip[2], ip[1], ip[0])
}
