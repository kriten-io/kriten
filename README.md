<h1 align="center">
  <a href="https://kriten.io" target="_blank"><img src="./assets/kriten.png" alt="Kriten" width="200"></a>
  <br>
  Kriten
</h1>

<h4 align="center">Visit <a href="https://kriten.io" target="_blank">kriten.io</a> for the full documentation, examples and guides.</h4>


<div align="center">

[![Version Release](https://github.com/Kriten-io/kriten/actions/workflows/version-release.yml/badge.svg)](https://github.com/Kriten-io/kriten/actions/workflows/version-release.yml)

</div>

<p align="center">
  <a href="#quickstart">Quickstart</a> •
  <a href="#contributing">Contributing</a> •
  <a href="#credits">Credits</a> •
  <a href="#license">License</a>
</p>

* Code execution platform
* Written for and runs on [Kubernetes](https://kubernetes.io/)
* Automated REST API exposure
  - Your custom code will be made available through dynamically created endpoints
* Granular RBAC control
* Local and Active Directory user authentication

## Quickstart

Kriten is avaible to be installed with [Helm](https://helm.sh/). From your command line:

```bash
# Add kriten Repo to Helm and update
$ helm repo add kriten https://kriten-io.github.io/kriten-charts/
$ helm repo update

# Create a namespace in your cluster
$ kubectl create ns kriten

# Install Kriten
$ helm install kriten kriten/kriten -n kriten
```

> **Note**
> You may want to modify the default values before install, more info on the installation can be found [here](https://kriten.io/#installation).


## Contributing

Kriten welcomes users feedback and ideas, feel free to raise an issue on GitHub if you need any help.

## Credits

This software uses the following open source packages:

- [Gin](https://gin-gonic.com/)
- [K8s client-go](https://github.com/kubernetes/client-go/)
- [Swaggo](https://github.com/swaggo/swag)
- [GORM](https://gorm.io/)

## License

GNU General Public License v3.0, see [LICENSE](https://github.com/kriten-io/kriten/blob/main/LICENSE).

