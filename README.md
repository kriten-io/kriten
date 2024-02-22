[![Version Release](https://github.com/Kriten-io/kriten/actions/workflows/version-release.yml/badge.svg)](https://github.com/Kriten-io/kriten/actions/workflows/version-release.yml)

# Kriten

## Quick Install

### Add helm repo
```helm repo add kriten https://kriten-io.github.io/kriten-charts/```

```helm repo update```

### Copy values.yaml (if necessary)
```helm show values kriten/kriten > myvalues.yaml```

### Edit myvalues.yaml

### Create namespace
```kubectl create namespace kriten```

### Install
```helm install -f myvalues.yaml kriten kriten/kriten -n kriten```

or

```helm install kriten kriten/kriten -n kriten```
