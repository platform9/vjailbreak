package kube

import (
	"log"

	vjailbreakv1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClient() client.Client {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}

	scheme, err := vjailbreakv1.SchemeBuilder.Build()
	if err != nil {
		log.Fatalf("Failed to build scheme: %v", err)
	}

	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	return kubeClient
}
