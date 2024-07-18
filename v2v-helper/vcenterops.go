package main

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
)

func ValidateVCenter(ctx context.Context, username, password, host string, disableSSLVerification bool) (*vim25.Client, error) {
	// add protocol to host if not present
	if host[:4] != "http" {
		host = "https://" + host + "/sdk"
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return c, nil
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
		InsecureSkipVerify: true, // Skip verification for demonstration purposes
	})
	if err != nil {
		return "", err
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
