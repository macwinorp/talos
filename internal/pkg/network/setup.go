/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"context"
	"log"
	"syscall"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
	"github.com/vishvananda/netlink"
)

// InitNetwork handles the initial network setup.
// This includes doing ifup for lo and eth0 and
// the initial ip addressing via dhcp for eth0
func InitNetwork() (err error) {
	return defaultNetworkSetup()
}

// SetupNetwork configures the network interfaces
// based on userdata. If the userdata does not contain
// any network configuration, the default network setup
// done in InitNetwork is maintained and dhcpd is kicked
// off for eth0.
// nolint: gocyclo
func SetupNetwork(data *userdata.UserData) (err error) {
	if data == nil || data.Networking == nil || data.Networking.OS == nil {
		return nil
	}

	for _, netconf := range data.Networking.OS.Devices {
		// ifup / create bonded interface
		if netconf.Bond != nil {
			if err = setupBonding(netconf); err != nil {
				log.Printf("failed to bring up bonded interface: %+v", err)
				continue
			}
		} else {
			if err = setupSingleLink(netconf); err != nil {
				log.Printf("failed to bring up single link interface: %+v", err)
				continue
			}
		}

		if netconf.DHCP {
			// TODO: this calls out to 'networkd' inline
			if _, err = NewService().Dhclient(context.Background(), netconf.Interface); err != nil {
				log.Printf("failed to obtain dhcp lease for %s: %+v", netconf.Interface, err)
				continue
			}
		}
		if netconf.CIDR != "" {
			if err = StaticAddress(netconf); err != nil {
				log.Printf("failed to set address for %s: %+v", netconf.Interface, err)
				continue
			}
		}
	}

	return nil
}

// Maybe look at adjusting this to accept an interface value from a kernel arg
func defaultNetworkSetup() (err error) {
	log.Println("bringing up lo")
	if err = ifup("lo", 0); err != nil {
		return err
	}
	// TODO should this be lo0
	// Set up the appropriate addr on loopback
	log.Println("setting static ip for lo")
	if err = StaticAddress(userdata.Device{Interface: "lo", CIDR: "127.0.0.1/8"}); err != nil && err != syscall.EEXIST {
		return err
	}

	iface := defaultInterface()
	log.Printf("bringing up %s\n", iface)
	if err = ifup(iface, 0); err != nil {
		return err
	}
	// TODO: this calls out to 'networkd' inline
	if _, err = NewService().Dhclient(context.Background(), iface); err != nil {
		return err
	}

	return nil
}

func ifup(ifname string, mtu int) (err error) {
	var link netlink.Link
	if link, err = netlink.LinkByName(ifname); err != nil {
		return err
	}
	attrs := link.Attrs()
	switch attrs.OperState {
	case netlink.OperUnknown:
		fallthrough
	case netlink.OperDown:
		if err = netlink.LinkSetUp(link); err != nil && err != syscall.EEXIST {
			log.Printf("im failing here in operdown for %s", ifname)
			return err
		}
	case netlink.OperUp:
	default:
		return errors.Errorf("cannot handle current state of %s: %s", ifname, attrs.OperState.String())
	}

	// Configure MTU if specified
	if mtu > 0 {
		if err = netlink.LinkSetMTU(link, mtu); err != nil {
			return err
		}
	}

	return nil
}

func defaultInterface() string {
	netif := DefaultInterface
	if option := kernel.ProcCmdline().Get(constants.KernelParamDefaultInterface).First(); option != nil {
		netif = *option
	}

	return netif
}
