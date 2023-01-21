# dragonfly-operator

Dragonfly Operator creates and manages instances of DragonflyDB

![Version: 0.0.6](https://img.shields.io/badge/Version-0.0.6-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1-alpha.6](https://img.shields.io/badge/AppVersion-0.0.1--alpha.6-informational?style=flat-square)

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
    --version 0.0.6
```

The command will install the chart in version `0.0.6` with dragonfly-operator version `0.0.1-alpha.6` (if not overriden) into the namespace `dragonfly-operator`.

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
| affinity | object | `{}` |  |
| cleanup.enabled | bool | `true` |  |
| cleanup.image.pullPolicy | string | `"IfNotPresent"` |  |
| cleanup.image.repository | string | `"gcr.io/google_containers/hyperkube"` |  |
| cleanup.image.tag | string | `"v1.18.0"` |  |
| env[0].name | string | `"DRAGONFLY_IMAGE_REPOSITORY"` |  |
| env[0].value | string | `"ghcr.io/dragonflydb/dragonfly"` |  |
| env[1].name | string | `"DRAGONFLY_IMAGE_TAG"` |  |
| env[1].value | string | `"v0.14.0"` |  |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/tamcore/dragonfly-operator"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| podAnnotations | object | `{}` |  |
| podMonitor.annotations | object | `{}` |  |
| podMonitor.enable | bool | `true` |  |
| podSecurityContext.runAsNonRoot | bool | `true` |  |
| replicas | int | `1` |  |
| resources.limits.cpu | string | `"500m"` |  |
| resources.limits.memory | string | `"128Mi"` |  |
| resources.requests.cpu | string | `"10m"` |  |
| resources.requests.memory | string | `"64Mi"` |  |
| securityContext.allowPrivilegeEscalation | bool | `false` |  |
| securityContext.capabilities.drop[0] | string | `"ALL"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| tolerations | list | `[]` |  |

