package ovs

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	dknet "github.com/docker/go-plugins-helpers/network"
	"github.com/samalba/dockerclient"
	"github.com/socketplane/libovsdb"
	"github.com/vishvananda/netlink"
)

const (
	defaultRoute     = "0.0.0.0/0"
	ovsPortPrefix    = "ovs-veth0-"
	bridgePrefix     = "ovsbr-"
	containerEthName = "eth"

	mtuOption        = "net.gopher.ovs.bridge.mtu"
	bridgeNameOption = "net.gopher.ovs.bridge.name"

	defaultMTU = 1500
)

type dockerer struct {
	client *dockerclient.DockerClient
}

type Driver struct {
	dknet.Driver
	dockerer
	ovsdber
	networks map[string]*NetworkState
	OvsdbNotifier
}

// NetworkState is filled in at network creation time
// it contains state that we wish to keep for each network
type NetworkState struct {
	BridgeName  string
	MTU         int
	Gateway     string
	GatewayMask string
}

func (d *Driver) GetCapabilities() (*dknet.CapabilitiesResponse, error) {
	return &dknet.CapabilitiesResponse{Scope: "local"}, nil
}

func (d *Driver) CreateNetwork(r *dknet.CreateNetworkRequest) error {
	log.Debugf("Create network request: %+v", r)

	bridgeName, err := getBridgeName(r)
	if err != nil {
		return err
	}

	mtu, err := getBridgeMTU(r)
	if err != nil {
		return err
	}

	gateway, mask, err := getGatewayIP(r)
	if err != nil {
		return err
	}

	ns := &NetworkState{
		BridgeName:  bridgeName,
		MTU:         mtu,
		Gateway:     gateway,
		GatewayMask: mask,
	}
	d.networks[r.NetworkID] = ns

	log.Debugf("Initializing bridge for network %s", r.NetworkID)
	if err := d.initBridge(r.NetworkID); err != nil {
		delete(d.networks, r.NetworkID)
		return err
	}
	return nil
}

func (d *Driver) DeleteNetwork(r *dknet.DeleteNetworkRequest) error {
	log.Debugf("Delete network request: %+v", r)
	var bridgeName string
	if net, ok := d.networks[r.NetworkID]; ok {
		bridgeName = net.BridgeName
	} else {
		return errors.New("Unknown network")
	}
	log.Debugf("Deleting Bridge %s", bridgeName)
	err := d.deleteBridge(bridgeName)
	if err != nil {
		log.Errorf("Deleting bridge %s failed: %s", bridgeName, err)
		return err
	}
	delete(d.networks, r.NetworkID)
	return nil
}

func (d *Driver) CreateEndpoint(r *dknet.CreateEndpointRequest) (*dknet.CreateEndpointResponse, error) {
	log.Debugf("Create endpoint request: %+v", r)
	return &dknet.CreateEndpointResponse{Interface: &dknet.EndpointInterface{}}, nil
}

func (d *Driver) DeleteEndpoint(r *dknet.DeleteEndpointRequest) error {
	log.Debugf("Delete endpoint request: %+v", r)
	return nil
}

func (d *Driver) EndpointInfo(r *dknet.InfoRequest) (*dknet.InfoResponse, error) {
	res := &dknet.InfoResponse{
		Value: make(map[string]string),
	}
	return res, nil
}

func (d *Driver) Join(r *dknet.JoinRequest) (*dknet.JoinResponse, error) {
	// create and attach local name to the bridge
	localVethPair := vethPair(truncateID(r.EndpointID))
	if err := netlink.LinkAdd(localVethPair); err != nil {
		log.Errorf("failed to create the veth pair named: [ %v ] error: [ %s ] ", localVethPair, err)
		return nil, err
	}
	// Bring the veth pair up
	err := netlink.LinkSetUp(localVethPair)
	if err != nil {
		log.Warnf("Error enabling  Veth local iface: [ %v ]", localVethPair)
		return nil, err
	}
	if val, ok := d.networks[r.NetworkID]; ok {
		bridgeName := val.BridgeName
		err = d.addOvsVethPort(bridgeName, localVethPair.Name, 0)
		if err != nil {
			log.Errorf("error attaching veth [ %s ] to bridge [ %s ]", localVethPair.Name, bridgeName)
			return nil, err
		}
		log.Infof("Attached veth [ %s ] to bridge [ %s ]", localVethPair.Name, bridgeName)
	} else {
		err = errors.New(fmt.Sprintf("No bridge with id [ %s ] for veth [ %s ]", r.NetworkID, localVethPair.Name))
		log.Error(err)
		return nil, err
	}

	// SrcName gets renamed to DstPrefix + ID on the container iface
	res := &dknet.JoinResponse{
		InterfaceName: dknet.InterfaceName{
			SrcName:   localVethPair.PeerName,
			DstPrefix: containerEthName,
		},
		Gateway: d.networks[r.NetworkID].Gateway,
	}
	log.Debugf("Join endpoint %s:%s to %s", r.NetworkID, r.EndpointID, r.SandboxKey)
	return res, nil
}

func (d *Driver) Leave(r *dknet.LeaveRequest) error {
	log.Debugf("Leave request: %+v", r)
	localVethPair := vethPair(truncateID(r.EndpointID))
	if err := netlink.LinkDel(localVethPair); err != nil {
		log.Errorf("unable to delete veth on leave: %s", err)
	}
	portID := fmt.Sprintf(ovsPortPrefix + truncateID(r.EndpointID))
	if val, ok := d.networks[r.NetworkID]; ok {
		bridgeName := val.BridgeName
		err := d.ovsdber.deletePort(bridgeName, portID)
		if err != nil {
			log.Errorf("OVS port [ %s ] delete transaction failed on bridge [ %s ] due to: %s", portID, bridgeName, err)
			return err
		}
		log.Infof("Deleted OVS port [ %s ] from bridge [ %s ]", portID, bridgeName)
	} else {
		err := errors.New(fmt.Sprintf("No bridge with id [ %s ] for port [ %s ]", r.NetworkID, portID))
		log.Error(err)
		return err
	}
	log.Debugf("Leave %s:%s", r.NetworkID, r.EndpointID)
	return nil
}

func NewDriver(protocol string, target string) (*Driver, error) {
	docker, err := dockerclient.NewDockerClient("unix:///var/run/docker.sock", nil)
	if err != nil {
		return nil, fmt.Errorf("could not connect to docker: %s", err)
	}

	// initiate the ovsdb manager port binding
	var ovsdb *libovsdb.OvsdbClient
	retries := 3
	for i := 0; i < retries; i++ {
		ovsdb, err = libovsdb.ConnectUsingProtocol(protocol, target)
		if err == nil {
			break
		}
		log.Errorf("could not connect to openvswitch on [ %s ]: %s. Retrying in 5 seconds", target, err)
		time.Sleep(5 * time.Second)
	}

	if ovsdb == nil {
		return nil, fmt.Errorf("could not connect to open vswitch")
	}

	d := &Driver{
		dockerer: dockerer{
			client: docker,
		},
		ovsdber: ovsdber{
			ovsdb: ovsdb,
		},
		networks: make(map[string]*NetworkState),
	}
	// Initialize ovsdb cache at rpc connection setup
	d.ovsdber.initDBCache()
	return d, nil
}

// Create veth pair. Peername is renamed to eth0 in the container
func vethPair(suffix string) *netlink.Veth {
	return &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: ovsPortPrefix + suffix},
		PeerName:  "ethc" + suffix,
	}
}

// Enable a netlink interface
func interfaceUp(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		log.Debugf("Error retrieving a link named [ %s ]", iface.Attrs().Name)
		return err
	}
	return netlink.LinkSetUp(iface)
}

func truncateID(id string) string {
	return id[:5]
}

func getBridgeMTU(r *dknet.CreateNetworkRequest) (int, error) {
	bridgeMTU := defaultMTU
	if r.Options != nil {
		if generic, ok := r.Options["com.docker.network.generic"].(map[string]interface{}); !ok {
			return bridgeMTU, nil
		} else if mtu, ok := generic[mtuOption].(int); ok {
			bridgeMTU = mtu
		}
	}
	return bridgeMTU, nil
}

func getBridgeName(r *dknet.CreateNetworkRequest) (string, error) {
	bridgeName := bridgePrefix + truncateID(r.NetworkID)
	if r.Options != nil {
		if generic, ok := r.Options["com.docker.network.generic"].(map[string]interface{}); !ok {
			return bridgeName, nil
		} else if name, ok := generic[bridgeNameOption].(string); ok {
			bridgeName = name
		}
	}
	return bridgeName, nil
}

func getGatewayIP(r *dknet.CreateNetworkRequest) (string, string, error) {
	// FIXME: Dear future self, I'm sorry for leaving you with this mess, but I want to get this working ASAP
	// This should be an array
	// We need to handle case where we have
	// a. v6 and v4 - dual stack
	// auxilliary address
	// multiple subnets on one network
	// also in that case, we'll need a function to determine the correct default gateway based on it's IP/Mask
	var gatewayIP string

	if len(r.IPv6Data) > 0 {
		if r.IPv6Data[0] != nil {
			if r.IPv6Data[0].Gateway != "" {
				gatewayIP = r.IPv6Data[0].Gateway
			}
		}
	}
	// Assumption: IPAM will provide either IPv4 OR IPv6 but not both
	// We may want to modify this in future to support dual stack
	if len(r.IPv4Data) > 0 {
		if r.IPv4Data[0] != nil {
			if r.IPv4Data[0].Gateway != "" {
				gatewayIP = r.IPv4Data[0].Gateway
			}
		}
	}

	if gatewayIP == "" {
		return "", "", fmt.Errorf("No gateway IP found")
	}
	parts := strings.Split(gatewayIP, "/")
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("Cannot split gateway IP address")
	}
	return parts[0], parts[1], nil
}
