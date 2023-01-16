# dragonfly-operator

Dragonfly Operator creates and manages instances of DragonflyDB

![Version: 0.0.5](https://img.shields.io/badge/Version-0.0.5-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1-alpha.5](https://img.shields.io/badge/AppVersion-0.0.1--alpha.5-informational?style=flat-square)

## Requirements

Kubernetes: `>=1.23.0-0`

## TL;DR

### Installing the Chart

```bash
$ helm upgrade \
    --install \
    --namespace dragonfly-operator \
    --create-namespace \
    dragonfly-operator \
    oci://ghcr.io/tamcore/dragonfly-operator/chart/dragonfly-operator \
    --version 0.0.5
```

The command will install the chart in version `0.0.5` with dragonfly-operator version `0.0.1-alpha.5` (if not overriden) into the namespace `dragonfly-operator`.

> **Tip**: List all releases using `helm list`

### Uninstalling the Chart

To uninstall/delete the `dragonfly-operator` deployment:

```bash
$ helm -n dragonfly-operator uninstall dragonfly-operator
```
The command removes all the components associated with the chart and deletes the release. It's recommended to delete deployed `Dragonfly` resources, **before** removing the operator.

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| dragonflyOperator.kubeRbacProxy.image.repository | string | `"gcr.io/kubebuilder/kube-rbac-proxy"` |  |
| dragonflyOperator.kubeRbacProxy.image.tag | string | `"v0.13.1"` |  |
| dragonflyOperator.kubeRbacProxy.resources.limits.cpu | string | `"500m"` |  |
| dragonflyOperator.kubeRbacProxy.resources.limits.memory | string | `"128Mi"` |  |
| dragonflyOperator.kubeRbacProxy.resources.requests.cpu | string | `"5m"` |  |
| dragonflyOperator.kubeRbacProxy.resources.requests.memory | string | `"64Mi"` |  |
| dragonflyOperator.manager.image.repository | string | `"ghcr.io/tamcore/dragonfly-operator"` |  |
| dragonflyOperator.manager.image.tag | string | `"0.0.1-alpha.5"` |  |
| dragonflyOperator.manager.resources.limits.cpu | string | `"500m"` |  |
| dragonflyOperator.manager.resources.limits.memory | string | `"128Mi"` |  |
| dragonflyOperator.manager.resources.requests.cpu | string | `"10m"` |  |
| dragonflyOperator.manager.resources.requests.memory | string | `"64Mi"` |  |
| dragonflyOperator.replicas | int | `1` |  |
| imagePullSecrets | list | `[]` |  |
| kubernetesClusterDomain | string | `"cluster.local"` |  |
| metricsService.ports[0].name | string | `"https"` |  |
| metricsService.ports[0].port | int | `8443` |  |
| metricsService.ports[0].protocol | string | `"TCP"` |  |
| metricsService.ports[0].targetPort | string | `"https"` |  |
| metricsService.type | string | `"ClusterIP"` |  |

