package cmd

import (
	"context"
	"fmt"
	imagereflectv1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	"github.com/spf13/cobra"
	"github.com/toddkazakov/flux-webhooks/kube"
	"log/slog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

var dockerhubListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List Docker Hub Webhooks",
	Long:    "The list command is used to retrieve a list of all webhooks which will be created",
	PreRunE: initLogger,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
		defer cancel()

		webhooks, err := ListRepositoryReceivers(ctx)
		if err != nil {
			return err
		}

		fmt.Println("Repository\t\t\tName\t\t\tPath")
		for _, receiver := range webhooks {
			fmt.Println(fmt.Sprintf("%s\t\t\t%s\t\t\t%s", receiver.Repository, receiver.Name, receiver.Path))
		}

		return nil
	},
}

func ListRepositoryReceivers(ctx context.Context) (map[string]RepositoryReceiver, error) {
	var listOpts []client.ListOption
	repoList := &imagereflectv1.ImageRepositoryList{}
	receivers := &notificationv1.ReceiverList{}

	kubeClient, err := kube.NewClient(kubeconfigArgs)
	err = kubeClient.List(ctx, repoList, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("listing image repositories failed: %w", err)
	}

	repositories := make(map[string]imagereflectv1.ImageRepository)
	for _, repo := range repoList.Items {
		key := fmt.Sprintf("%s/%s/%s/%s", repo.APIVersion, repo.Kind, repo.Namespace, repo.Name)
		repositories[key] = repo
		slog.Debug("added repository", "repository", key)
	}

	err = kubeClient.List(ctx, receivers, listOpts...)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks failed: %w", err)
	}

	webhooks := make(map[string]RepositoryReceiver)
	for _, receiver := range receivers.Items {
		receiverKey := fmt.Sprintf("%s/%s/%s/%s", receiver.APIVersion, receiver.Kind, receiver.Namespace, receiver.Name)
		slog.Debug("found receiver", "key", receiverKey, "type", receiver.Spec.Type)

		if receiver.Spec.Type == notificationv1.DockerHubReceiver {
			for _, ref := range receiver.Spec.Resources {
				if ref.Namespace == "" {
					ref.Namespace = receiver.Namespace
				}

				ref.APIVersion = "image.toolkit.fluxcd.io/v1beta2"
				key := fmt.Sprintf("%s/%s/%s/%s", ref.APIVersion, ref.Kind, ref.Namespace, ref.Name)
				if receiver.Status.WebhookPath == "" {
					slog.Debug("ignoring receiver with inactive webhook", "receiver", receiverKey, "type", receiver.Spec.Type)
					continue
				}

				if repo, ok := repositories[key]; ok {
					slog.Debug("found receiver for repository", "receiver", receiverKey, "type", receiver.Spec.Type)
					receiverShortKey := fmt.Sprintf("%s/%s", receiver.Namespace, receiver.Name)
					webhookKey := fmt.Sprintf("%s:%s", repo.Spec.Image, receiverShortKey)
					webhooks[webhookKey] = RepositoryReceiver{
						Repository: repo.Spec.Image,
						Name:       receiverShortKey,
						Path:       receiver.Status.WebhookPath,
					}
				}
			}
		}
	}

	return webhooks, nil
}

func init() {
	dockerhubCmd.AddCommand(dockerhubListCmd)
}

type RepositoryReceiver struct {
	Repository string
	Name       string
	Path       string
}
