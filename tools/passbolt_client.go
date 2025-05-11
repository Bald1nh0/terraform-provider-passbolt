// Package tools provides shared utilities for the Passbolt Terraform provider.
package tools

import (
	"context"
	"fmt"

	"github.com/passbolt/go-passbolt/api"
)

// PassboltClient wraps the low-level API client and configuration.
type PassboltClient struct {
	Client     *api.Client
	URL        string
	PrivateKey string
	Password   string
}

// Login authenticates the Passbolt client using its internal credentials.
func Login(ctx context.Context, client *PassboltClient) error {
	if err := client.Client.Login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}
