// Copyright © 2024 The vjailbreak authors

package vcenter

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
)

//go:generate mockgen -source=../vcenter/vcenterops.go -destination=../vcenter/vcenterops_mock.go -package=vcenter

type VCenterOperations interface {
	getDatacenters(ctx context.Context) ([]*object.Datacenter, error)
	GetVMByName(ctx context.Context, name string) (*object.VirtualMachine, error)
}

type VCenterClient struct {
	VCClient            *vim25.Client
	VCFinder            *find.Finder
	VCPropertyCollector *property.Collector
}

func validateVCenter(ctx context.Context, username, password, host string, disableSSLVerification bool) (*vim25.Client, error) {
	// add protocol to host if not present
	if host[:4] != "http" {
		host = "https://" + host
	}
	if host[len(host)-4:] != "/sdk" {
		host += "/sdk"
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %v", err)
	}
	u.User = url.UserPassword(username, password)
	// fmt.Println(u)
	// Connect and log in to ESX or vCenter
	// Share govc's session cache
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
	}

	c := new(vim25.Client)
	err = s.Login(ctx, c, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to login: %v", err)
	}
	return c, nil
}

func VCenterClientBuilder(ctx context.Context, username, password, host string, disableSSLVerification bool) (*VCenterClient, error) {
	client, err := validateVCenter(ctx, username, password, host, disableSSLVerification)
	if err != nil {
		return nil, fmt.Errorf("failed to validate vCenter connection: %v", err)
	}
	finder := find.NewFinder(client, false)
	pc := property.DefaultCollector(client)
	return &VCenterClient{VCClient: client, VCFinder: finder, VCPropertyCollector: pc}, nil
}

// This was used to generate the VDDK URL in case it was virt-v2v needed it
// func GenerateVDDKUrl(username, vcenterurl, datacenter, cluster, host string) string {
// 	if vcenterurl[:4] != "http" {
// 		vcenterurl = "http://" + vcenterurl + "/" + datacenter + "/" + cluster + "/" + host + "?no_verify=1"
// 	}
// 	u, err := url.Parse(vcenterurl)
// 	if err != nil {
// 		return ""
// 	}
// 	u.User = url.User(username)
// 	// fmt.Println(u.String())
// 	return u.String()
// }

func GetThumbprint(host string) (string, error) {
	// Get the thumbprint of the vCenter server
	// Establish a TLS connection to the server
	conn, err := tls.Dial("tcp", host+":443", &tls.Config{
		InsecureSkipVerify: true, // Skip verification
	})
	if err != nil {
		return "", fmt.Errorf("failed to connect to vCenter: %v", err)
	}
	defer conn.Close()

	// Get the server's certificates
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("no certificates found")
	}

	// Compute the SHA-1 thumbprint of the first certificate
	cert := certs[0]
	thumbprint := ""

	for idx, thumbyte := range sha1.Sum(cert.Raw) {
		thumbprint += hex.EncodeToString([]byte{thumbyte})
		if idx < len(sha1.Sum(cert.Raw))-1 {
			thumbprint += ":"
		}
	}

	// Return the thumbprint as a hexadecimal string
	return thumbprint, nil
}

// Get all datacenters
func (vcclient *VCenterClient) getDatacenters(ctx context.Context) ([]*object.Datacenter, error) {
	// Find all datacenters
	datacenters, err := vcclient.VCFinder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to get datacenters: %v", err)
	}

	return datacenters, nil
}

// get VM by name
func (vcclient *VCenterClient) GetVMByName(ctx context.Context, name string) (*object.VirtualMachine, error) {
	datacenters, err := vcclient.getDatacenters(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get datacenters: %v", err)
	}
	for _, datacenter := range datacenters {
		vcclient.VCFinder.SetDatacenter(datacenter)
		vm, err := vcclient.VCFinder.VirtualMachine(ctx, name)
		if err == nil {
			return vm, nil
		}
	}
	return nil, fmt.Errorf("VM not found")
}
