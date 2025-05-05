// Copyright © 2024 The vjailbreak authors

package types

import (
    "k8s.io/apimachinery/pkg/runtime"
    utilruntime "k8s.io/apimachinery/pkg/util/runtime"
    clientgoscheme "k8s.io/client-go/kubernetes/scheme"
    vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
)

var (
    scheme = runtime.NewScheme()
)

func init() {
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))
    utilruntime.Must(vjailbreakv1alpha1.AddToScheme(scheme))
}

// Re-export types from vjailbreak v1alpha1
type VMwareMachine = vjailbreakv1alpha1.VMwareMachine
type VMwareCredsSpec = vjailbreakv1alpha1.VMwareCredsSpec
