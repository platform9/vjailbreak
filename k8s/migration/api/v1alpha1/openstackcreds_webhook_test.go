/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("OpenstackCreds Webhook", func() {

	Context("When creating OpenstackCreds under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {
			// Validation logic deferred to #2347
		})

		It("Should admit if all required fields are provided", func() {
			// Validation logic deferred to #2347
		})
	})

})

// TestOpenstackCredsCustomValidator tests the type-assertion logic in the validator.
func TestOpenstackCredsCustomValidator(t *testing.T) {
	v := &OpenstackCredsCustomValidator{}
	ctx := context.Background()
	validObj := &OpenstackCreds{}
	wrongObj := &corev1.Pod{}

	tests := []struct {
		name    string
		op      string
		wantErr bool
	}{
		{"ValidateCreate correct type", "create", false},
		{"ValidateUpdate correct type", "update", false},
		{"ValidateDelete correct type", "delete", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.op {
			case "create":
				_, err = v.ValidateCreate(ctx, validObj)
			case "update":
				_, err = v.ValidateUpdate(ctx, nil, validObj)
			case "delete":
				_, err = v.ValidateDelete(ctx, validObj)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}

	wrongTests := []struct {
		name string
		op   string
	}{
		{"ValidateCreate wrong type", "create"},
		{"ValidateUpdate wrong type", "update"},
		{"ValidateDelete wrong type", "delete"},
	}
	for _, tt := range wrongTests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.op {
			case "create":
				_, err = v.ValidateCreate(ctx, wrongObj)
			case "update":
				_, err = v.ValidateUpdate(ctx, nil, wrongObj)
			case "delete":
				_, err = v.ValidateDelete(ctx, wrongObj)
			}
			if err == nil {
				t.Errorf("expected error for wrong type, got nil")
			}
		})
	}
}
