package controllers

import (
	"fmt"
	"net"
	"strings"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"
)

func (c *SvcController) syncSecurityGroup(svcName string) error {
	if len(c.securityGroupIDs) == 0 {
		return nil
	}

	svcObj, exists, err := c.indexer.GetByKey(svcName)
	if err != nil {
		klog.Errorf("could not get service %s by key: %v", svcName, err)
		return err
	}

	if !exists {
		klog.Warningf("service %s does not exists, ignoring", svcName)
		return nil
	}

	svc, ok := svcObj.(*v1.Service)
	if !ok {
		klog.Errorf("could not get service %s from obejct", svcName)
		return fmt.Errorf("could not get service %s from obejct", svcName)
	}

	instanceAPI := instance.NewAPI(c.scwClient)

	gotErr := false

	for _, id := range c.securityGroupIDs {
		klog.Infof("syncing security group %s with service %s", id, svcName)
		sgID, zone, err := getZonalID(id)
		if err != nil {
			klog.Errorf("could not get id and zone from %s: %v", sgID, err)
			gotErr = true
			continue
		}

		sgRulesResp, err := instanceAPI.ListSecurityGroupRules(&instance.ListSecurityGroupRulesRequest{
			SecurityGroupID: sgID,
			Zone:            scw.Zone(zone),
		}, scw.WithAllPages())
		if err != nil {
			klog.Errorf("could not list rules for security group %s: %v", sgID, err)
			gotErr = true
			continue
		}

		for _, port := range svc.Spec.Ports {
			found := false
			if port.NodePort == 0 {
				continue
			}
			for _, sgRule := range sgRulesResp.Rules {
				if sgRule.Action != instance.SecurityGroupRuleActionAccept || sgRule.Direction != instance.SecurityGroupRuleDirectionInbound || sgRule.Protocol.String() != string(port.Protocol) {
					continue
				}
				if sgRule.DestPortFrom != nil && sgRule.DestPortTo != nil && *sgRule.DestPortFrom == *sgRule.DestPortTo && *sgRule.DestPortFrom == uint32(port.NodePort) {
					found = true
					break
				}
			}
			if !found {
				_, err := instanceAPI.CreateSecurityGroupRule(&instance.CreateSecurityGroupRuleRequest{
					SecurityGroupID: sgID,
					Zone:            scw.Zone(zone),
					Action:          instance.SecurityGroupRuleActionAccept,
					Direction:       instance.SecurityGroupRuleDirectionInbound,
					Protocol:        instance.SecurityGroupRuleProtocol(port.Protocol),
					IPRange: scw.IPNet{
						IPNet: net.IPNet{
							IP:   net.ParseIP("0.0.0.0"),
							Mask: net.IPv4Mask(0, 0, 0, 0), // TODO better idea?
						},
					},
					DestPortFrom: scw.Uint32Ptr(uint32(port.NodePort)),
					DestPortTo:   scw.Uint32Ptr(uint32(port.NodePort)),
				})
				if err != nil {
					klog.Errorf("could not create security group rule for svc %s port %s: %v", svcName, port.NodePort, err)
					gotErr = true
					continue
				}
			}
		}
	}

	if gotErr {
		return fmt.Errorf("got some errors")
	}

	return nil

}

func (c *NodeController) syncSecurityGroup(nodeName string) error {
	if len(c.securityGroupIDs) == 0 {
		return nil
	}

	_, exists, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}

	server, err := c.getInstanceFromNodeName(nodeName)
	if err != nil {
		klog.Warningf("could not get instance %s: %v", nodeName, err)
		if exists {
			return err
		}
		// end here if node does not exists anymore and we couldn't get the server
		// in order to delete the old IP
		return nil
	}

	instanceAPI := instance.NewAPI(c.scwClient)

	gotErr := false

	for _, id := range c.securityGroupIDs {
		klog.Infof("syncing security group %s with node %s", id, nodeName)
		sgID, zone, err := getZonalID(id)
		if err != nil {
			klog.Errorf("could not get id and zone from %s: %v", sgID, err)
			gotErr = true
			continue
		}
		if zone != "" && zone != server.Zone.String() {
			klog.Warningf("ignoring security group %s as it's not in the same zone as the node %s", sgID, nodeName)
			continue
		}

		sgRulesResp, err := instanceAPI.ListSecurityGroupRules(&instance.ListSecurityGroupRulesRequest{
			SecurityGroupID: sgID,
			Zone:            server.Zone,
		}, scw.WithAllPages())
		if err != nil {
			klog.Errorf("could not list rules for security group %s: %v", sgID, err)
			gotErr = true
			continue
		}

		toDelete := []string{}
		foundPrivate := false
		foundPublic := false

		for _, sgRule := range sgRulesResp.Rules {
			if server.PublicIP != nil && sgRule.IPRange.IP.Equal(server.PublicIP.Address) {
				foundPublic = true
				if !exists {
					toDelete = append(toDelete, sgRule.ID)
				}
			}
			if server.PrivateIP != nil && *server.PrivateIP != "" && sgRule.IPRange.IP.Equal(net.ParseIP(*server.PrivateIP)) {
				foundPrivate = true
				if !exists {
					toDelete = append(toDelete, sgRule.ID)
				}
			}
		}

		for _, delID := range toDelete {
			err := instanceAPI.DeleteSecurityGroupRule(&instance.DeleteSecurityGroupRuleRequest{
				Zone:                server.Zone,
				SecurityGroupID:     sgID,
				SecurityGroupRuleID: delID,
			})
			if err != nil {
				klog.Errorf("could not delete security group rule %s for SG %s: %v", delID, sgID, err)
				gotErr = true
				continue
			}
		}

		toAdd := []net.IP{}
		if !foundPrivate && exists {
			toAdd = append(toAdd, net.ParseIP(*server.PrivateIP))
		}
		if !foundPublic && exists {
			toAdd = append(toAdd, server.PublicIP.Address)
		}

		for _, ip := range toAdd {
			_, err := instanceAPI.CreateSecurityGroupRule(&instance.CreateSecurityGroupRuleRequest{
				SecurityGroupID: sgID,
				Zone:            server.Zone,
				Action:          instance.SecurityGroupRuleActionAccept,
				Direction:       instance.SecurityGroupRuleDirectionInbound,
				Protocol:        instance.SecurityGroupRuleProtocolANY,
				IPRange: scw.IPNet{
					IPNet: net.IPNet{
						IP:   ip,
						Mask: net.IPv4Mask(255, 255, 255, 255), // TODO better idea?
					},
				},
			})
			if err != nil {
				klog.Errorf("could not add security group rule for node %s on %s: %v", nodeName, sgID, err)
				gotErr = true
				continue
			}
		}
	}

	if gotErr {
		return fmt.Errorf("got some errors")
	}

	return nil
}

func getZonalID(r string) (string, string, error) {
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
