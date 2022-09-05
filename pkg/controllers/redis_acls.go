package controllers

import (
	"fmt"
	"net"
	"os"

	redis "github.com/scaleway/scaleway-sdk-go/api/redis/v1alpha1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
)

func (c *NodeController) syncRedisACLs(nodeName string) error {
	if len(c.redisIDs) == 0 {
		return nil
	}

	var node *v1.Node

	retryOnError := false

	nodeObj, exists, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}
	if exists {
		var ok bool
		node, ok = nodeObj.(*v1.Node)
		if !ok {
			klog.Errorf("could not get node %s from object", nodeName)
			return fmt.Errorf("could not get node %s from object", nodeName)
		}
	}

	dbAPI := redis.NewAPI(c.scwClient)

	for _, redisID := range c.redisIDs {
		klog.Infof("whitelisting IP on node %s on redis instance %s", nodeName, redisID)

		id, zone, err := getRegionalizedID(redisID)
		if err != nil {
			klog.Errorf("could not get id and zone from %s: %v", redisID, err)
			continue
		}

		dbInstance, err := dbAPI.GetCluster(&redis.GetClusterRequest{
			Zone:      scw.Zone(zone),
			ClusterID: id,
		})
		if err != nil {
			klog.Errorf("could not get redis instance %s: %v", id, err)
			continue
		}

		var rule *redis.ACLRule

		for _, acl := range dbInstance.ACLRules {
			if *acl.Description == nodeName {
				rule = acl
				break
			}
		}

		if !exists && rule != nil {
			err := dbAPI.DeleteACLRule(&redis.DeleteACLRuleRequest{
				Zone:  dbInstance.Zone,
				ACLID: rule.IP.String(),
			})
			if err != nil {
				klog.Errorf("could not delete acl rule for node %s on redis instance %s: %v", nodeName, dbInstance.ID, err)
				retryOnError = true
			}
			continue
		}

		var nodePublicIP net.IP

		if os.Getenv(NodesIPSource) == NodesIPSourceKubernetes {
			for _, addr := range node.Status.Addresses {
				if addr.Type == v1.NodeExternalIP {
					nodePublicIP = net.ParseIP(addr.Address)
					if len(nodePublicIP) == net.IPv6len {
						// prefer ipv4 over ipv6 since Database are only accessible via ipv4
						continue
					}
					break
				}
			}
		} else {
			server, err := c.getInstanceFromNodeName(nodeName)
			if err != nil {
				klog.Errorf("could not get instance %s: %v", nodeName, err)
				continue
			}

			if server.PublicIP == nil {
				klog.Warningf("skipping node %s without public IP", nodeName)
				continue
			}

			nodePublicIP = server.PublicIP.Address
		}

		if nodePublicIP == nil {
			klog.Warningf("skipping node %s without public IP", nodeName)
			continue
		}

		nodeIP := net.IPNet{
			IP:   nodePublicIP,
			Mask: net.IPv4Mask(255, 255, 255, 255), // TODO better idea?
		}

		if rule == nil || nodeIP.String() != rule.IP.String() {
			_, err := dbAPI.AddACLRules(&redis.AddACLRulesRequest{
				Zone:      dbInstance.Zone,
				ClusterID: dbInstance.ID,
				ACLRules: []*redis.ACLRuleSpec{
					{
						IP:          scw.IPNet{IPNet: nodeIP},
						Description: nodeName,
					},
				},
			})
			if err != nil {
				klog.Errorf("could not add acl rule for node %s with ip %s on redis instance %s: %v", nodeName, nodeIP.String(), dbInstance.ID, err)
				retryOnError = true
				continue
			}
		}
	}

	if retryOnError {
		return fmt.Errorf("got retryable error")
	}

	return nil
}
