package vault

import (
	"context"
	"errors"
	"fmt"
	"link-validator/pkg/config"
	"link-validator/pkg/errs"
	"link-validator/pkg/regex"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/vault-client-go"
)

type LinkProcessor struct {
	clients map[string]vaultClient
}

func New(vaults []config.Vault, timeout time.Duration) (*LinkProcessor, error) {
	processor := LinkProcessor{clients: make(map[string]vaultClient)}
	for _, v := range vaults {
		for _, vaultUrl := range v.Urls {
			vaultClient, err := vault.New(
				vault.WithAddress(vaultUrl),
				vault.WithRequestTimeout(timeout),
			)
			if err != nil {
				return nil, err
			}
			err = vaultClient.SetToken(v.Token)
			if err != nil {
				return nil, err
			}
			processor.clients[vaultUrl] = &wrapper{vaultClient}
		}
	}
	return &processor, nil
}

func (proc *LinkProcessor) Process(ctx context.Context, link string, _ string) error {
	slog.Debug("vault: starting validation", slog.String("url", link))
	u, err := url.Parse(link)
	if err != nil {
		return err
	}
	vaultClient, err := proc.getClient(fmt.Sprintf("%s://%s", u.Scheme, u.Hostname()))
	if err != nil {
		return err
	}

	secretPath := transformPath(u.Path)
	err = validateSecret(ctx, vaultClient, secretPath)
	return err
}

func validateSecret(ctx context.Context, client vaultClient, secretPath string) error {
	var vaultError *vault.ResponseError
	_, err := client.List(ctx, secretPath)
	if err == nil {
		// secret folder found
		return nil
	}
	if errors.As(err, &vaultError) {
		if vaultError.StatusCode != http.StatusNotFound {
			return err
		}
	}

	// due to limitation of KVv1 I need to read the secret,
	// I can't just read secret metadata as it would be possible in KVv2 only
	_, err = client.Read(ctx, secretPath)
	if err != nil {
		if errors.As(err, &vaultError) {
			if vaultError.StatusCode != http.StatusNotFound {
				return errs.NewNotFound(secretPath)
			}
			return err
		}
	}
	return nil
}

// transformPath strips the UI path '/ui/vault/secrets/' and removes '/show/' if present in the UI path
func transformPath(path string) string {
	secretPath := strings.TrimPrefix(path, "/ui/vault/secrets/")
	parts := strings.Split(secretPath, "/")
	if parts[1] == "show" {
		parts = append(parts[:1], parts[2:]...)
	}
	secretPath = strings.Join(parts, "/")
	return fmt.Sprintf("/%s", secretPath)
}

func (proc *LinkProcessor) getClient(hostname string) (vaultClient, error) {
	for host := range proc.clients {
		if strings.HasPrefix(hostname, host) {
			return proc.clients[host], nil
		}
	}
	return nil, fmt.Errorf("no vaultClient found for '%s'", hostname)
}

func (proc *LinkProcessor) ExtractLinks(line string) []string {
	result := make([]string, 0)

	urls := regex.Url.FindAllString(line, -1)

	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil || u.Hostname() == "" {
			continue // skip malformed
		}

		for k := range proc.clients {
			if strings.HasPrefix(raw, k) {
				result = append(result, raw)
			}
		}
	}
	return result
}
