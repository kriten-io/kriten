[![Version Release](https://github.com/Kriten-io/kriten/actions/workflows/version-release.yml/badge.svg)](https://github.com/Kriten-io/kriten/actions/workflows/version-release.yml)

# Kriten

## Quick Install

### Add helm repo
```helm repo add kriten https://kriten-io.github.io/kriten-charts/```

### Copy values.yaml
```helm show values kriten/kriten > myvalues.yaml```

### Edit myvalues.yaml

### Install

```helm install -f myvalues.yaml kriten kriten/kriten```
