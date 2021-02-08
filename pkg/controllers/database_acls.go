package controllers

import (
	"fmt"
	"net"
	"strings"

	rdb "github.com/scaleway/scaleway-sdk-go/api/rdb/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	klog "k8s.io/klog/v2"
)

func (c *NodeController) syncDatabaseACLs(nodeName string) error {
	if len(c.databaseIDs) == 0 {
		return nil
	}

	retryOnError := false

	_, exists, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}

	dbAPI := rdb.NewAPI(c.scwClient)

	for _, dbID := range c.databaseIDs {
		klog.Infof("whitelisting IP on node %s on database %s", nodeName, dbID)

		id, region, err := getRegionalizedID(dbID)
		if err != nil {
			klog.Errorf("could not get id and region from %s: %v", dbID, err)
			continue
		}

		dbInstance, err := dbAPI.GetInstance(&rdb.GetInstanceRequest{
			Region:     scw.Region(region),
			InstanceID: id,
		})
		if err != nil {
			klog.Errorf("could not get rdb instance %s: %v", id, err)
			continue
		}

		acls, err := dbAPI.ListInstanceACLRules(&rdb.ListInstanceACLRulesRequest{
			Region:     dbInstance.Region,
			InstanceID: dbInstance.ID,
		}, scw.WithAllPages())
		if err != nil {
			klog.Errorf("could not get rdb acl rule for instance %s: %v", id, err)
			continue
		}

		var rule *rdb.ACLRule

		for _, acl := range acls.Rules {
			if acl.Description == nodeName {
				rule = acl
				break
			}
		}

		if !exists && rule != nil {
			_, err := dbAPI.DeleteInstanceACLRules(&rdb.DeleteInstanceACLRulesRequest{
				Region:     dbInstance.Region,
				ACLRuleIPs: []string{rule.IP.String()},
				InstanceID: dbInstance.ID,
			})
			if err != nil {
				klog.Errorf("could not delete acl rule for node %s on db %s: %v", nodeName, dbInstance.ID, err)
				retryOnError = true
			}
			continue
		}

		server, err := c.getInstanceFromNodeName(nodeName)
		if err != nil {
			klog.Errorf("could not get instance %s: %v", nodeName, err)
			continue
		}

		if server.PublicIP == nil {
			klog.Warningf("skipping node %s without public IP", nodeName)
			continue
		}
		nodeIP := net.IPNet{
			IP:   server.PublicIP.Address,
			Mask: net.IPv4Mask(255, 255, 255, 255), // TODO better idea?
		}

		if rule == nil || nodeIP.String() != rule.IP.String() {
			_, err := dbAPI.AddInstanceACLRules(&rdb.AddInstanceACLRulesRequest{
				Region:     dbInstance.Region,
				InstanceID: dbInstance.ID,
				Rules: []*rdb.ACLRuleRequest{
					{
						IP:          scw.IPNet{IPNet: nodeIP},
						Description: nodeName,
					},
				},
			})
			if err != nil {
				klog.Errorf("could not add acl rule for node %s with ip %s on db %s: %v", nodeName, nodeIP.String(), dbInstance.ID, err)
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

func getRegionalizedID(r string) (string, string, error) {
	split := strings.Split(r, "/")
	switch len(split) {
	case 1:
		return split[0], "", nil
	case 2:
		return split[1], split[0], nil
	default:
		return "", "", fmt.Errorf("couldn't parse ID %s", r)
	}
}
