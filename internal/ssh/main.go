package ssh

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"golang.org/x/crypto/ssh"
)

func GetSSHPrivateKey(cred azcore.TokenCredential, vaultName, keyPrefix string) (*azsecrets.GetSecretResponse, error) {
	if vaultName == "" {
		return nil, fmt.Errorf("vault name must not be empty")
	}
	// Establish a connection to the Key Vault client
	client, _ := azsecrets.NewClient(fmt.Sprintf("https://%s.vault.azure.net/", vaultName), cred, nil)
	pager := client.NewListSecretsPager(nil)
	for pager.More() {
		page, err := pager.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("cannot paginate over secret list: %w", err)
		}
		for _, secret := range page.Value {
			if strings.HasPrefix(secret.ID.Name(), keyPrefix) {
				// Get a secret. An empty string version gets the latest version of the secret.
				version := ""
				resp, err := client.GetSecret(context.TODO(), secret.ID.Name(), version, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to get the secret: %w", err)
				}

				return &resp, nil
			}
		}
	}
	return nil, fmt.Errorf("cannot find any private key")
}

func GetSSHCliConfig(userName string, privateKey []byte) (*ssh.ClientConfig, error) {
	if userName == "" {
		return nil, fmt.Errorf("user name must be set")
	}

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Always accept key.
			return nil
		},
	}

	return config, nil
}
