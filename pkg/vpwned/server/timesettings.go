package server

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeSettingsNamespace = "migration-system"
	pf9EnvConfigMapName   = "pf9-env"
)

var timeSettingsDeployments = []string{
	"migration-controller-manager",
	"migration-vpwned-sdk",
	"vjailbreak-ui",
}

func patchPf9EnvTZ(ctx context.Context, k8sClient client.Client, tz string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: pf9EnvConfigMapName, Namespace: timeSettingsNamespace}, cm)
		if err != nil {
			newCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: pf9EnvConfigMapName, Namespace: timeSettingsNamespace},
				Data:       map[string]string{"TZ": tz},
			}
			return k8sClient.Create(ctx, newCM)
		}
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data["TZ"] = tz
		return k8sClient.Update(ctx, cm)
	})
}

func rolloutRestartDeployment(ctx context.Context, k8sClient client.Client, name, ns string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		deployment := &appsv1.Deployment{}
		if err := k8sClient.Get(ctx, k8stypes.NamespacedName{Name: name, Namespace: ns}, deployment); err != nil {
			return err
		}
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}
		deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)
		return k8sClient.Update(ctx, deployment)
	})
}
