package netmgr

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brwallis/srlinux-go/pkg/gnmi"
	"github.com/brwallis/srlinux-go/pkg/net"
	"github.com/brwallis/srlinux-go/pkg/srlinux"
	"github.com/brwallis/srlinux-kubemgr/internal/agent"
	gpb "github.com/openconfig/gnmi/proto/gnmi"
	"k8s.io/client-go/kubernetes"
	log "k8s.io/klog"
)

// NetMgr deals with physical interfaces passed into vSRL, the default network-instance, and BGP peers
func NetMgr(clientSet *kubernetes.Clientset, nodeIP string, kubeMgr *agent.Agent) error {
	var err error
	var interfaces []net.Interface

	// Create the default network instance
	err = srlinux.AddNetworkInstance("default", "default")
	if err != nil {
		log.Fatalf("Unable to create default network-instance: error %e", err)
	}

	interfaces = net.ListInterfaces()
	for _, nic := range interfaces {
		// Bind the vfio driver
		//net.BindVFIO(nic.PCIAddress)
		// Create the interface in SR Linux
		srlinux.AddInterface(nic.UdevName, "nic", "routed", "default")
		// Update the subinterface with the addressing we learned by probing Linux
		addresses := []srlinux.IPPrefix{}
		subInterfaceIPs := []srlinux.SubInterfaceIP{}
		for _, ipInfo := range nic.IPInfo {
			// TODO: Check if the IP is v4 or v6
			addresses = append(addresses, srlinux.IPPrefix{
				Prefix: ipInfo.IPCIDR,
			})
		}
		// Check if we got an address for the interface
		if len(addresses) < 1 {
			// We didn't find an address, allocate one
			log.Infof("Didn't find address from passed through interface: %s, creating one myself...", nic.UdevName)
			octets := strings.Split(nodeIP, ".")
			underLayIP := fmt.Sprintf("10.77.%s.%s", octets[2], octets[3])
			underLayCIDR := fmt.Sprintf("%s/24", underLayIP)
			log.Infof("Created address %s", underLayCIDR)
			addresses = append(addresses, srlinux.IPPrefix{
				Prefix: underLayCIDR,
			})
		}
		subInterfaceIPs = append(subInterfaceIPs, srlinux.SubInterfaceIP{
			Addresses: addresses,
		})
		out, err := json.MarshalIndent(subInterfaceIPs, "", "  ")
		if err != nil {
			log.Infof("Unable to Marshal interface: %e", err)
		}
		// Set the interface
		gnmiSubInterfacePath := fmt.Sprintf("/interface[name=%s]/subinterface[index=0]/ipv4", nic.UdevName)
		gnmiSubInterface := &gpb.TypedValue{
			Value: &gpb.TypedValue_JsonIetfVal{
				JsonIetfVal: out,
			},
		}
		log.Infof("Setting interface addresses for interface: %s, address: %v", nic.UdevName, subInterfaceIPs)
		_, err = gnmi.Set(gnmiSubInterfacePath, gnmiSubInterface)
		if err != nil {
			log.Errorf("Unable to set interface address: %e", err)
		}
	}

	log.Infof("Setting router ID for default network instance...")
	defaultInstance := srlinux.NetworkInstance{
		// TODO Change this later, only lets us run one instance of vSRL per node,
		// we could use the loopback CNI with whereabouts instead
		RouterID: nodeIP,
	}
	out, err := json.MarshalIndent(defaultInstance, "", " ")
	if err != nil {
		log.Fatalf("Unable to Marshal default network instance: %e", err)
	}
	gnmiNetworkInstance := &gpb.TypedValue{
		Value: &gpb.TypedValue_JsonIetfVal{
			JsonIetfVal: out,
		},
	}

	gnmiNetworkInstancePath := fmt.Sprintf("/network-instance[name=%s]", "default")
	log.Infof("Update default network-instance: path %v, value %#v", gnmiNetworkInstancePath, defaultInstance)
	_, err = gnmi.Set(gnmiNetworkInstancePath, gnmiNetworkInstance)

	// Add this vSRL instance to the list of underlay peers
	// err = storage.UnderlayPeerManagement()
	return err
}
