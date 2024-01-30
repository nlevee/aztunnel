package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/go-playground/validator"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type Config struct {
	SubscriptionID string `yaml:"subscription" validate:"required"`
	ResourceGroup  string `yaml:"resource-group" validate:"required"`
	Vault          struct {
		Name      string `yaml:"name" validate:"required"`
		KeyPrefix string `yaml:"key-prefix" validate:"required"`
	} `yaml:"vault"`
	Bastion struct {
		Name   string `yaml:"name" validate:"required"`
		Server string `yaml:"server" validate:"required"`
	} `yaml:"bastion"`
	SSH struct {
		User string `yaml:"user" validate:"required"`
		Port int    `yaml:"port" validate:"min=0"`
		Dest string `yaml:"dest" validate:"required"`
	} `yaml:"ssh"`
	Cluster string `yaml:"cluster,omitempty"`
}

func init() {
	pflag.StringP("config", "c", "", "Load config file")
	pflag.Parse()

	viper.BindPFlags(pflag.CommandLine)
}

func main() {
	// read config file from disk
	configFile := viper.GetString("config")
	if configFile == "" {
		log.Fatalf("-config flag is required")
	}

	f, err := os.Open(configFile)
	if err != nil {
		log.Fatalf("cannot open config file: %v", err)
	}
	defer f.Close()

	// load configuration
	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		log.Fatalf("cannot decode config file: %v", err)
	}
	err = validator.New().Struct(cfg)
	if err != nil {
		log.Fatalf("validation failed due to %v", err)
	}

	// Create a credential using the NewDefaultAzureCredential type.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to obtain a credential: %v", err)
	}

	// Establish a connection to the Key Vault client
	privateKey, err := getSSHPrivateKey(cred, cfg.Vault.Name, cfg.Vault.KeyPrefix)
	if err != nil {
		log.Fatalf("failed to obtain private Key: %v", err)
	}

	sshTunnelPort, _ := getFreePort()

	// open tunnel to bastion
	go func(port int, subID, rgName, bastion, bastionVM string) {
		if subID == "" {
			log.Fatalf("subscription id is required to start tunnel")
		}
		if rgName == "" {
			log.Fatalf("resource-group is required to start tunnel")
		}

		cmd := cli.GetAzureCLICommand()
		cmd.Args = append(cmd.Args, "network", "bastion", "tunnel",
			fmt.Sprintf("--subscription=%s", subID),
			fmt.Sprintf("--target-resource-id=/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", subID, rgName, bastionVM),
			fmt.Sprintf("--port=%d", port),
			fmt.Sprintf("--name=%s", bastion),
			fmt.Sprintf("--resource-group=%s", rgName),
			"--resource-port=22",
		)
		defer cmd.Process.Kill()

		std, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("failed open tunnel: %v -- %s", err, std)
		}
		log.Printf("%s: %s", cmd.String(), std)
	}(
		sshTunnelPort,
		cfg.SubscriptionID,
		cfg.ResourceGroup,
		cfg.Bastion.Name,
		cfg.Bastion.Server,
	)

	signer, err := ssh.ParsePrivateKey([]byte(*privateKey.Value))
	if err != nil {
		log.Fatalf("Unable to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: cfg.SSH.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Always accept key.
			return nil
		},
	}

	dest := cfg.SSH.Dest
	if dest == "" {
		log.Fatalf("destination server and port must be set")
	}

	kubeClusterName := cfg.Cluster
	var kubeHandler TunnelHandler
	if kubeClusterName != "" {
		kubeHandler = &TunnelKubernetesHandler{
			KubeClusterName: kubeClusterName,
		}
	}
	if err := runTunnel(config, cfg.SSH.Port, sshTunnelPort, dest, kubeHandler); err != nil {
		log.Fatalf("Unable to run tunnel: %v", err)
	}
}

func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

type TunnelKubernetesHandler struct {
	KubeClusterName string
}

func (t *TunnelKubernetesHandler) Handle(l net.Listener) error {
	server := fmt.Sprintf("https://localhost:%d", l.Addr().(*net.TCPAddr).Port)
	KubeConfigClusterSet(t.KubeClusterName, false, server)
	return nil
}

type TunnelHandler interface {
	Handle(l net.Listener) error
}

func runTunnel(config *ssh.ClientConfig, lport, sshPort int, dest string, tunnelHandler TunnelHandler) error {
	maxAttenmpts := 10
	attemptsLeft := maxAttenmpts
	var (
		client *ssh.Client
		err    error
	)
	for {
		client, err = ssh.Dial("tcp", fmt.Sprintf("localhost:%d", sshPort), config)
		if err != nil {
			attemptsLeft--
			if attemptsLeft <= 0 {
				return fmt.Errorf("failed to dial: %w", err)
			}
			time.Sleep(1 * time.Second)
			log.Printf("server dial error: %v: attempt %d/%d", err, maxAttenmpts-attemptsLeft, maxAttenmpts)
		} else {
			break
		}
	}
	defer client.Close()

	log.Printf("opening SSH connection to 'localhost:%d' succeed", sshPort)

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", lport))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	defer listener.Close()

	forwardedPort := listener.Addr().(*net.TCPAddr).Port
	log.Printf("waiting on 'localhost:%v' succeed", forwardedPort)

	// try to handle connection enabled
	if tunnelHandler != nil {
		if err := tunnelHandler.Handle(listener); err != nil {
			return fmt.Errorf("cannot handle after connection enabled: %w", err)
		}
	}

	for {
		// Like ssh -L by default, local connections are handled one at a time.
		// While one local connection is active in forward, others will be stuck
		// dialing, waiting for this Accept.
		local, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		// Issue a dial to the remote server on our SSH client
		// refers to the remote server.
		remote, err := client.Dial("tcp", dest)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("tunnel established with: %s", local.LocalAddr())
		go forward(local, remote)
	}
}

func forward(local, remote net.Conn) {
	defer local.Close()
	defer remote.Close()
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()

	<-done
}

func getSSHPrivateKey(cred azcore.TokenCredential, vaultName, keyPrefix string) (*azsecrets.GetSecretResponse, error) {
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
