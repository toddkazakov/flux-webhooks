package kube

import (
	imagereflectv1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewClient(kubeconfigArgs *genericclioptions.ConfigFlags) (client.WithWatch, error) {
	cfg, err := kubeconfigArgs.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	scheme := apiruntime.NewScheme()
	_ = imagereflectv1.AddToScheme(scheme)
	_ = metav1.AddMetaToScheme(scheme)
	_ = notificationv1.AddToScheme(scheme)

	return client.NewWithWatch(cfg, client.Options{
		Scheme: scheme,
	})
}
