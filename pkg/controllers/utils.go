package controllers

import (
	"fmt"
	"strings"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"k8s.io/api/core/v1"
)

func (c *NodeController) getInstanceFromNodeName(nodeName string) (*instance.Server, error) {
	instanceAPI := instance.NewAPI(c.scwClient)

	instanceResp, err := instanceAPI.ListServers(&instance.ListServersRequest{
		Name: scw.StringPtr(nodeName),
	})
	if err != nil {
		return nil, err
	}
	if len(instanceResp.Servers) != 1 {
		return nil, fmt.Errorf("got %d servers instead of 1", len(instanceResp.Servers))
	}
	return instanceResp.Servers[0], nil
}

func (c *NodeController) getFreeIP() (*instance.IP, error) {
	instanceAPI := instance.NewAPI(c.scwClient)

	ipsList, err := instanceAPI.ListIPs(&instance.ListIPsRequest{}, scw.WithAllPages())
	if err != nil {
		return nil, err
	}

	for _, ip := range ipsList.IPs {
		if ip.Server == nil && stringInSlice(ip.Address.String(), c.reservedIPs) {
			return ip, nil
		}
	}
	return nil, nil
}

func stringInSlice(s string, slice []string) bool {
	for _, i := range slice {
		if i == s {
			return true
		}
	}
	return false
}

func isPublicSvc(svc *v1.Service) bool {
	return svc.Spec.Type == v1.ServiceTypeLoadBalancer || svc.Spec.Type == v1.ServiceTypeNodePort
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
