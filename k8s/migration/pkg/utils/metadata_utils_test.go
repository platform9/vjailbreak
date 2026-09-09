package utils

import (
	"fmt"
	"testing"

	"github.com/gophercloud/gophercloud/v2"
)

func TestIsTransientOpenstackError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "500 internal server error is transient",
			err:  gophercloud.ErrUnexpectedResponseCode{Actual: 500},
			want: true,
		},
		{
			name: "503 service unavailable is transient",
			err:  gophercloud.ErrUnexpectedResponseCode{Actual: 503},
			want: true,
		},
		{
			name: "403 forbidden is not transient",
			err:  gophercloud.ErrUnexpectedResponseCode{Actual: 403},
			want: false,
		},
		{
			name: "404 not found is not transient",
			err:  gophercloud.ErrUnexpectedResponseCode{Actual: 404},
			want: false,
		},
		{
			name: "401 unauthorized is not transient",
			err:  gophercloud.ErrUnexpectedResponseCode{Actual: 401},
			want: false,
		},
		{
			name: "non-gophercloud error is not transient",
			err:  fmt.Errorf("some other error"),
			want: false,
		},
		{
			name: "nil error is not transient",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientOpenstackError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientOpenstackError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
