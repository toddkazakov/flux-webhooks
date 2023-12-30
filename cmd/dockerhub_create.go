package cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/docker/hub-tool/pkg/hub"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type DockerHubConfig struct {
	Username string
	Password string
	BaseUrl  string
	DryRun   bool
}

var hubConfig = &DockerHubConfig{}

var dockerhubCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Docker Hub Webhooks",
	Long:  "The create command is used to create dockerhub webhooks from flux receivers pointing to dockerhub repositories",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 360*time.Second)
		defer cancel()

		baseUrl, err := url.Parse(hubConfig.BaseUrl)
		if err != nil {
			return fmt.Errorf("unable to parse base url: %w", err)
		}

		webhooks, err := ListRepositoryReceivers(ctx)
		if err != nil {
			return err
		}

		repositoryWebhooks := make([]RepositoryReceiver, 0, len(webhooks))
		for _, webhook := range webhooks {
			slog.Debug("webhooks for reconciliation", "repository", webhook.Repository, "name", webhook.Name, "path", webhook.Path)
			repositoryWebhooks = append(repositoryWebhooks, webhook)
		}

		token, err := GetToken(hubConfig.Username, hubConfig.Password)
		if err != nil {
			return fmt.Errorf("unable to obtain dockerhub token: %s", err)
		}

		err = CreateMissingWebhooks(ctx, token, baseUrl, repositoryWebhooks)
		if err != nil {
			return fmt.Errorf("unable to create missing webhooks: %s", err)
		}

		return nil
	},
}

func init() {
	dockerhubCreateCmd.Flags().StringVar(&hubConfig.Username, "hub-username", "", "docker hub username")
	dockerhubCreateCmd.Flags().StringVar(&hubConfig.Password, "hub-password", "", "docker hub password")
	dockerhubCreateCmd.Flags().StringVar(&hubConfig.BaseUrl, "receiver-base-url", "", "flux webhook receiver base url")
	_ = dockerhubCreateCmd.MarkFlagRequired("hub-username")
	_ = dockerhubCreateCmd.MarkFlagRequired("hub-password")
	_ = dockerhubCreateCmd.MarkFlagRequired("receiver-base-url")

	dockerhubCmd.AddCommand(dockerhubCreateCmd)
}

func GetToken(username, password string) (string, error) {
	hubClient, err := hub.NewClient(hub.WithHubAccount(username), hub.WithPassword(password))
	if err != nil {
		return "", fmt.Errorf("can't initiate hubClient: %w", err)
	}

	token, _, err := hubClient.Login(username, password, func() (string, error) {
		return readFromStdin("2FA required, please provide the 6 digit code: ")
	})

	return token, err
}

func CreateMissingWebhooks(ctx context.Context, token string, baseUrl *url.URL, targetWebhooks []RepositoryReceiver) error {
	repositoryWebhooks := make(map[string][]DockerHubWebhookWrapper)
	for _, webhook := range targetWebhooks {
		wrapper, err := NewDockerHubWebhook(baseUrl.String(), webhook)
		if err != nil {
			return err
		}

		repositoryWebhooks[webhook.Repository] = append(repositoryWebhooks[webhook.Repository], wrapper)
	}

	for repository, webhooks := range repositoryWebhooks {
		pageSize := 100
		resp, err := GetWebhooksForRepository(repository, token, 100)
		if err != nil {
			return err
		}
		if resp.Count > pageSize {
			return fmt.Errorf("too many webhooks")
		}

		urls := make(map[string]struct{}, resp.Count)
		for _, webhook := range resp.Results {
			if len(webhook.Webhooks) != 1 {
				return fmt.Errorf("unexpected number of elements for webhook %s - %s", repository, webhook.Name)
			}

			urls[webhook.Webhooks[0].HookUrl] = struct{}{}
		}

		for _, webhook := range webhooks {
			u := webhook.Webhooks[0].HookUrl
			if _, ok := urls[u]; !ok {
				slog.Info("creating webhook", "name", webhook.Name, "repository", repository)
				err := CreateWebhookForRepository(ctx, repository, token, webhook)
				if err != nil {
					slog.Error("couldn't create webhook", "name", webhook.Name, "repository", repository, "error", err.Error())
				}
			} else {
				slog.Info("ignoring webhook because it already exists", "name", webhook.Name, "repository", repository)
			}
		}
	}

	return nil
}

func GetWebhooksForRepository(repository, token string, pageSize int) (GetDockerHubWebhooksResponse, error) {
	u := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/webhook_pipeline/", repository)
	client := resty.New().SetHeader("Content-Type", "application/json")

	result := GetDockerHubWebhooksResponse{}
	_, err := client.R().
		SetQueryParam("page_size", strconv.Itoa(pageSize)).
		SetResult(&result).
		SetAuthToken(token).
		Get(u)

	return result, err
}

func CreateWebhookForRepository(ctx context.Context, repository, token string, webhook DockerHubWebhookWrapper) error {
	u := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/webhook_pipeline/", repository)
	client := resty.New().SetHeader("Content-Type", "application/json")

	resp, err := client.R().
		SetContext(ctx).
		SetBody(webhook).
		SetAuthToken(token).
		Post(u)

	if err != nil {
		return err
	}
	if resp.StatusCode() != http.StatusCreated {
		return fmt.Errorf("unexpected response from docker hub: %s", string(resp.Body()))
	}

	return nil
}

func NewDockerHubWebhook(baseUrl string, webhook RepositoryReceiver) (w DockerHubWebhookWrapper, err error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return
	}

	u.Path = webhook.Path
	w = DockerHubWebhookWrapper{
		Name:                webhook.Name,
		ExpectFinalCallback: false,
		Webhooks: []DockerHubWebhookModel{{
			Name:    webhook.Name,
			HookUrl: u.String(),
		}},
	}

	return
}

func readFromStdin(prompt string) (string, error) {
	var out string
	var err error
	fmt.Fprint(os.Stdout, prompt)
	out, err = bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("could not read from stdin: %w", err)
	}
	fmt.Println()
	return strings.TrimRight(out, "\r\n"), nil
}

type GetDockerHubWebhooksResponse struct {
	Count   int                       `json:"count"`
	Next    string                    `json:"next"`
	Results []DockerHubWebhookWrapper `json:"results"`
}

type DockerHubWebhookWrapper struct {
	Name                string                  `json:"name"`
	ExpectFinalCallback bool                    `json:"expect_final_callback"`
	Webhooks            []DockerHubWebhookModel `json:"webhooks"`
}

type DockerHubWebhookModel struct {
	Name    string `json:"name"`
	HookUrl string `json:"hook_url"`
}
