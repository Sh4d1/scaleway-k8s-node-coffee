package controllers

import (
	"fmt"
	"net"
	"time"

	dns "github.com/scaleway/scaleway-sdk-go/api/domain/v2beta1"
	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	klog "k8s.io/klog/v2"
)

const (
	waitingPropagation = time.Second * 30
)

func (c *NodeController) syncReverseIP(nodeName string) error {
	if c.reverseIPDomain == "" {
		return nil
	}

	klog.Infof("adding a reverse for IP on node %s", nodeName)

	_, exists, err := c.indexer.GetByKey(nodeName)
	if err != nil {
		klog.Errorf("could not get node %s by key: %v", nodeName, err)
		return err
	}

	dnsAPI := dns.NewAPI(c.scwClient)
	instanceAPI := instance.NewAPI(c.scwClient)

	if !exists {
		if c.scwZoneFound {
			maxPage := uint32(1000)
			records := []*dns.Record{}
			page := int32(0)

			for {
				page = page + 1
				listing, err := dnsAPI.ListDNSZoneRecords(&dns.ListDNSZoneRecordsRequest{
					DNSZone:  c.scwZone,
					Page:     &page,
					PageSize: &maxPage,
					Type:     "A",
				})
				if err != nil {
					klog.Errorf("could not get checking record dns for node %s: %v", nodeName, err)
					return err
				}
				records = append(records, listing.Records...)
				if len(listing.Records) < int(maxPage) {
					break
				}
			}

			var recordToDelete *dns.Record
			for i := range records {
				if records[i].Comment != nil && *records[i].Comment == fmt.Sprintf("k8s node %s", nodeName) {
					recordToDelete = records[i]
					break
				}
			}

			if recordToDelete != nil {
				klog.Infof("try to remove record dns for node %s", nodeName)

				_, err := dnsAPI.UpdateDNSZoneRecords(&dns.UpdateDNSZoneRecordsRequest{
					DNSZone: c.scwZone,
					Changes: []*dns.RecordChange{
						&dns.RecordChange{
							Delete: &dns.RecordChangeDelete{
								ID: &recordToDelete.ID,
							},
						},
					},
				})
				if err != nil {
					klog.Errorf("could delete record dns for node %s: %v", nodeName, err)
					return err
				}

				klog.Infof("try to remove reverse for node %s", nodeName)
				instanceAPI.UpdateIP(&instance.UpdateIPRequest{
					IP:      recordToDelete.Data,
					Reverse: &instance.NullableStringValue{Null: true},
				})

			}
		}

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

	if server.PublicIP.Dynamic {
		klog.Warningf("can't update the reverse of a dynamic IP for node %s", nodeName)
		return nil
	}

	if c.scwZoneFound {
		comment := fmt.Sprintf("k8s node %s", nodeName)
		_, err := dnsAPI.UpdateDNSZoneRecords(&dns.UpdateDNSZoneRecordsRequest{
			DNSZone: c.reverseIPDomain,
			Changes: []*dns.RecordChange{
				&dns.RecordChange{
					Add: &dns.RecordChangeAdd{
						Records: []*dns.Record{
							&dns.Record{
								Data:    server.PublicIP.Address.String(),
								Name:    getReversePrefix(server.PublicIP.Address),
								TTL:     600,
								Type:    "A",
								Comment: &comment,
							},
						},
					},
				},
			},
			DisallowNewZoneCreation: true,
		})
		if err != nil {
			klog.Errorf("could not update record dns for node %s: %v", nodeName, err)
			return err
		}
		klog.Infof("waiting propagation for record dns for node %s", nodeName)
		time.Sleep(waitingPropagation)
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
