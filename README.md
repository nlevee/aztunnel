# Azure Bastion Tunnel Tool

Ce repository contient un outil permettant d'ouvrir un tunnel via Azure Bastion pour accéder à des ressources privées sur Azure de manière sécurisée.

## Prérequis

Avant d'utiliser cet outil, assurez-vous d'avoir les éléments suivants :

- Un abonnement Azure actif
- Accès en lecture/écriture aux ressources Azure dans votre abonnement
- Un Azure Bastion déployé
- Une clé SSH sur un keyvault azure (pour l'accès à la machine de rebond du bastion)
- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) installé et configuré sur votre machine

## Installation

1. Clonez ce repository sur votre machine locale :

    ```bash
    git clone https://github.com/nlevee/aztunnel.git
    ```

2. Accédez au répertoire du projet :

    ```bash
    cd aztunnel
    ```

3. Construisez le binaire sur **Linux** ou **MacOS** :

    ```bash
    make build-linux
    # or 
    make build-macos
    ```

## Configuration

Connectez-vous à votre compte Azure via Azure CLI :

```bash
az login
```

## Utilisation

1. Créer un fichier de config pour le tunnel (voir [example.yaml](./example.yaml))

2. Exécutez la commande suivante pour ouvrir un tunnel via Azure Bastion vers votre ressource privée :

    ```bash
    ./aztunnel -c ./tunnel.yaml
    ```

3. Une fois le tunnel ouvert, accédez à vos ressources privées via `localhost:<port_local>`.

## Contributions

Les contributions sont les bienvenues ! N'hésitez pas à ouvrir une issue pour signaler des bogues ou à soumettre une demande de fusion (pull request) avec des améliorations.

## Licence

Ce projet est sous licence [MIT](LICENSE).
