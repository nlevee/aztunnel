package main

import (
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/nlevee/aztunnel/internal/config"
	"github.com/nlevee/aztunnel/internal/handler"
	"github.com/nlevee/aztunnel/internal/ssh"
	"github.com/nlevee/aztunnel/internal/tunnel"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

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

	cfg, err := config.LoadFromFile(configFile)
	if err != nil {
		log.Fatalf("cannot open config file: %v", err)
	}

	// Create a credential using the NewDefaultAzureCredential type.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		log.Fatalf("failed to obtain a credential: %v", err)
	}

	// Establish a connection to the Key Vault client
	privateKey, err := ssh.GetSSHPrivateKey(cred, cfg.Vault.Name, cfg.Vault.KeyPrefix)
	if err != nil {
		log.Fatalf("failed to obtain private Key: %v", err)
	}

	sshTunnelPort, _ := tunnel.GetFreePort()

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

	config, err := ssh.GetSSHCliConfig(cfg.SSH.User, []byte(*privateKey.Value))
	if err != nil {
		log.Fatalf("cannot get ssh config for client: %v", err)
	}

	dest := cfg.SSH.Dest
	if dest == "" {
		log.Fatalf("destination server and port must be set")
	}

	kubeClusterName := cfg.Cluster
	var kubeHandler handler.TunnelHandler
	if kubeClusterName != "" {
		kubeHandler = &handler.TunnelKubernetesHandler{
			KubeClusterName: kubeClusterName,
		}
	}
	if err := tunnel.RunTunnel(config, cfg.SSH.Port, sshTunnelPort, dest, kubeHandler); err != nil {
		log.Fatalf("unable to run tunnel: %v", err)
	}
}
