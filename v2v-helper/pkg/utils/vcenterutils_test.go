// Copyright © 2024 The vjailbreak authors

package utils

import (
	"context"
	"os"
	"testing"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetMigrationParams_DataOnly(t *testing.T) {
	tests := []struct {
		name         string
		dataOnlyVal  string
		wantDataOnly bool
	}{
		{
			name:         "DATA_ONLY=true sets DataOnly to true",
			dataOnlyVal:  "true",
			wantDataOnly: true,
		},
		{
			name:         "DATA_ONLY=false sets DataOnly to false",
			dataOnlyVal:  "false",
			wantDataOnly: false,
		},
		{
			name:         "DATA_ONLY absent sets DataOnly to false",
			dataOnlyVal:  "",
			wantDataOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// VMWARE_MACHINE_OBJECT_NAME is read by GetMigrationConfigMapName
			const vmK8sName = "test-vm"
			os.Setenv("VMWARE_MACHINE_OBJECT_NAME", vmK8sName)
			defer os.Unsetenv("VMWARE_MACHINE_OBJECT_NAME")

			data := map[string]string{
				"SOURCE_VM_NAME": "my-vm",
			}
			if tt.dataOnlyVal != "" {
				data["DATA_ONLY"] = tt.dataOnlyVal
			}

			fakeClient := ctrlfake.NewClientBuilder().WithObjects(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "migration-config-" + vmK8sName,
					Namespace: constants.NamespaceMigrationSystem,
				},
				Data: data,
			}).Build()

			params, err := GetMigrationParams(context.Background(), fakeClient)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantDataOnly, params.DataOnly)
		})
	}
}
