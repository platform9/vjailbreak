// Copyright © 2026 The vjailbreak authors

package cinder

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/platform9/vjailbreak/pkg/vpwned/sdk/storage"
)

type actionCall struct {
	volumeID  string
	connector map[string]any
}

type fakeActionClient struct {
	initCalls []actionCall
	termCalls []actionCall
	connInfo  map[string]any
	initErr   error
	termErr   error
}

func (f *fakeActionClient) InitializeVolumeConnection(_ context.Context, volumeID string, connector map[string]any) (map[string]any, error) {
	f.initCalls = append(f.initCalls, actionCall{volumeID: volumeID, connector: connector})
	return f.connInfo, f.initErr
}

func (f *fakeActionClient) TerminateVolumeConnection(_ context.Context, volumeID string, connector map[string]any) error {
	f.termCalls = append(f.termCalls, actionCall{volumeID: volumeID, connector: connector})
	return f.termErr
}

func TestBuildConnectorFromHBAs(t *testing.T) {
	tests := []struct {
		name    string
		hbas    []string
		host    string
		ip      string
		want    map[string]any
		wantErr bool
	}{
		{
			name: "fc only",
			hbas: []string{
				"fc.20000025b510a086:21000025b510a086",
				"fc.20000025b510a087:21000025b510a087",
			},
			host: "vjailbreak-10-4-2-17",
			ip:   "10.4.2.17",
			want: map[string]any{
				"host":      "vjailbreak-10-4-2-17",
				"ip":        "10.4.2.17",
				"platform":  "x86_64",
				"os_type":   "linux",
				"multipath": true,
				"wwpns":     []string{"21000025b510a086", "21000025b510a087"},
				"wwnns":     []string{"20000025b510a086", "20000025b510a087"},
			},
		},
		{
			name: "iscsi only uses first iqn",
			hbas: []string{
				"iqn.1998-01.com.vmware:esx01-4aa9d624",
				"iqn.1998-01.com.vmware:esx01-second",
			},
			host: "vjailbreak-10-4-2-17",
			ip:   "10.4.2.17",
			want: map[string]any{
				"host":      "vjailbreak-10-4-2-17",
				"ip":        "10.4.2.17",
				"platform":  "x86_64",
				"os_type":   "linux",
				"multipath": true,
				"initiator": "iqn.1998-01.com.vmware:esx01-4aa9d624",
			},
		},
		{
			name: "mixed transports emit both key sets",
			hbas: []string{
				"iqn.1998-01.com.vmware:esx01-4aa9d624",
				"fc.20000025b510a086:21000025b510a086",
			},
			host: "h1",
			ip:   "10.0.0.1",
			want: map[string]any{
				"host":      "h1",
				"ip":        "10.0.0.1",
				"platform":  "x86_64",
				"os_type":   "linux",
				"multipath": true,
				"initiator": "iqn.1998-01.com.vmware:esx01-4aa9d624",
				"wwpns":     []string{"21000025b510a086"},
				"wwnns":     []string{"20000025b510a086"},
			},
		},
		{
			name: "uppercase fc uid is normalised to lowercase",
			hbas: []string{"fc.20000025B510A086:21000025B510A086"},
			host: "h1",
			want: map[string]any{
				"host":      "h1",
				"platform":  "x86_64",
				"os_type":   "linux",
				"multipath": true,
				"wwpns":     []string{"21000025b510a086"},
				"wwnns":     []string{"20000025b510a086"},
			},
		},
		{
			name: "malformed fc uid is skipped, remaining initiator used",
			hbas: []string{"fc.zzzz:yyyy", "iqn.1998-01.com.vmware:esx01"},
			host: "h1",
			want: map[string]any{
				"host":      "h1",
				"platform":  "x86_64",
				"os_type":   "linux",
				"multipath": true,
				"initiator": "iqn.1998-01.com.vmware:esx01",
			},
		},
		{
			name: "empty host falls back to default",
			hbas: []string{"iqn.1998-01.com.vmware:esx01"},
			want: map[string]any{
				"host":      DefaultConnectorHost,
				"platform":  "x86_64",
				"os_type":   "linux",
				"multipath": true,
				"initiator": "iqn.1998-01.com.vmware:esx01",
			},
		},
		{
			name:    "all malformed yields error",
			hbas:    []string{"fc.zzzz:yyyy", "vmhba0", ""},
			wantErr: true,
		},
		{
			name:    "empty input yields error",
			hbas:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildConnectorFromHBAs(tt.hbas, tt.host, tt.ip)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got connector %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("connector mismatch:\n got  %v\n want %v", got, tt.want)
			}
		})
	}
}

func TestCreateOrUpdateInitiatorGroupStashesConnector(t *testing.T) {
	m := &CinderMapper{Client: &fakeActionClient{}, Host: "h1", IP: "10.0.0.1"}
	mctx, err := m.CreateOrUpdateInitiatorGroup(context.Background(), "ignored", []string{"iqn.1998-01.com.vmware:esx01"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	connector, ok := mctx[ConnectorKey].(map[string]any)
	if !ok {
		t.Fatalf("mapping context missing %q entry: %v", ConnectorKey, mctx)
	}
	if connector["host"] != "h1" || connector["ip"] != "10.0.0.1" {
		t.Fatalf("connector host/ip not propagated: %v", connector)
	}
}

func TestMapVolumeToGroup(t *testing.T) {
	vol := storage.Volume{
		Name:         "volume-abc-cinder",
		OpenstackVol: storage.OpenstackVolume{ID: "abc"},
	}
	connector := map[string]any{"host": "h1", "initiator": "iqn.x"}
	mctx := storage.MappingContext{ConnectorKey: connector}

	t.Run("happy path calls initialize with volume id and connector", func(t *testing.T) {
		client := &fakeActionClient{connInfo: map[string]any{"driver_volume_type": "fibre_channel"}}
		m := &CinderMapper{Client: client}
		got, err := m.MapVolumeToGroup(context.Background(), "ignored", vol, mctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Name != vol.Name {
			t.Fatalf("volume not passed through: %v", got)
		}
		if len(client.initCalls) != 1 {
			t.Fatalf("expected 1 initialize call, got %d", len(client.initCalls))
		}
		if client.initCalls[0].volumeID != "abc" {
			t.Fatalf("wrong volume id: %s", client.initCalls[0].volumeID)
		}
		if !reflect.DeepEqual(client.initCalls[0].connector, connector) {
			t.Fatalf("connector not passed through: %v", client.initCalls[0].connector)
		}
	})

	t.Run("client error is wrapped and surfaced", func(t *testing.T) {
		client := &fakeActionClient{initErr: errors.New("backend down")}
		m := &CinderMapper{Client: client}
		if _, err := m.MapVolumeToGroup(context.Background(), "", vol, mctx); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing cinder volume id is rejected", func(t *testing.T) {
		client := &fakeActionClient{}
		m := &CinderMapper{Client: client}
		noID := storage.Volume{Name: "v"}
		if _, err := m.MapVolumeToGroup(context.Background(), "", noID, mctx); err == nil {
			t.Fatal("expected error for missing volume id")
		}
		if len(client.initCalls) != 0 {
			t.Fatal("initialize must not be called without a volume id")
		}
	})

	t.Run("missing connector is rejected", func(t *testing.T) {
		client := &fakeActionClient{}
		m := &CinderMapper{Client: client}
		if _, err := m.MapVolumeToGroup(context.Background(), "", vol, storage.MappingContext{}); err == nil {
			t.Fatal("expected error for missing connector")
		}
	})
}

func TestUnmapVolumeFromGroup(t *testing.T) {
	vol := storage.Volume{
		Name:         "volume-abc-cinder",
		OpenstackVol: storage.OpenstackVolume{ID: "abc"},
	}
	connector := map[string]any{"host": "h1", "wwpns": []string{"21000025b510a086"}}
	mctx := storage.MappingContext{ConnectorKey: connector}

	t.Run("happy path calls terminate with same connector", func(t *testing.T) {
		client := &fakeActionClient{}
		m := &CinderMapper{Client: client}
		if err := m.UnmapVolumeFromGroup(context.Background(), "ignored", vol, mctx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(client.termCalls) != 1 {
			t.Fatalf("expected 1 terminate call, got %d", len(client.termCalls))
		}
		if client.termCalls[0].volumeID != "abc" {
			t.Fatalf("wrong volume id: %s", client.termCalls[0].volumeID)
		}
		if !reflect.DeepEqual(client.termCalls[0].connector, connector) {
			t.Fatalf("connector not passed through: %v", client.termCalls[0].connector)
		}
	})

	t.Run("client error is wrapped and surfaced", func(t *testing.T) {
		client := &fakeActionClient{termErr: errors.New("backend down")}
		m := &CinderMapper{Client: client}
		if err := m.UnmapVolumeFromGroup(context.Background(), "", vol, mctx); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing cinder volume id is rejected", func(t *testing.T) {
		client := &fakeActionClient{}
		m := &CinderMapper{Client: client}
		if err := m.UnmapVolumeFromGroup(context.Background(), "", storage.Volume{Name: "v"}, mctx); err == nil {
			t.Fatal("expected error for missing volume id")
		}
	})
}
