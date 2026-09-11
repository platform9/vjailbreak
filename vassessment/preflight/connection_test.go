package preflight

import "testing"

func TestValidateConnectionInput(t *testing.T) {
	cases := []struct {
		name    string
		in      ConnectionInput
		wantErr bool
	}{
		{"valid hostname", ConnectionInput{Host: "vcenter01.acme.internal", Username: "svc@vsphere.local", Password: "secret"}, false},
		{"valid IP", ConnectionInput{Host: "10.20.4.5", Username: "svc", Password: "secret"}, false},
		{"valid https URL", ConnectionInput{Host: "https://vcenter01.acme.internal", Username: "svc", Password: "secret"}, false},
		{"missing host", ConnectionInput{Username: "svc", Password: "secret"}, true},
		{"missing username", ConnectionInput{Host: "vcenter01.acme.internal", Password: "secret"}, true},
		{"missing password", ConnectionInput{Host: "vcenter01.acme.internal", Username: "svc"}, true},
		{"invalid host chars", ConnectionInput{Host: "vcenter01!!!", Username: "svc", Password: "secret"}, true},
		{"host with space", ConnectionInput{Host: "vcenter 01", Username: "svc", Password: "secret"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateConnectionInput(c.in)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateConnectionInput(%+v) error = %v, wantErr %v", c.in, err, c.wantErr)
			}
		})
	}
}
