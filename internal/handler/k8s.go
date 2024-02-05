package handler

import (
	"fmt"
	"log"
	"net"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

type TunnelKubernetesHandler struct {
	KubeClusterName string
}

func (t *TunnelKubernetesHandler) Handle(l net.Listener) error {
	server := fmt.Sprintf("https://localhost:%d", l.Addr().(*net.TCPAddr).Port)
	KubeConfigClusterSet(t.KubeClusterName, false, server)
	return nil
}

func KubeConfigClusterListDisplay() {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	config, err := loadingRules.Load()
	if err != nil {
		log.Fatal(err)
	}

	for k := range config.Clusters {
		fmt.Println(k)
	}
}

func KubeConfigClusterSet(
	clusterName string,
	insecureSkipTLSVerify bool,
	server string,
) {
	pathOptions := clientcmd.NewDefaultPathOptions()
	config, err := pathOptions.LoadingRules.Load()
	if err != nil {
		log.Fatal(err)
	}

	cluster, exists := config.Clusters[clusterName]
	if !exists {
		cluster = api.NewCluster()
	}

	cluster.InsecureSkipTLSVerify = insecureSkipTLSVerify
	cluster.Server = server

	config.Clusters[clusterName] = cluster

	if err := clientcmd.ModifyConfig(pathOptions, *config, true); err != nil {
		log.Fatalf("error saving kubeconfig: %v", err)
	}
}
