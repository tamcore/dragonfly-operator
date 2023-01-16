# dragonfly-operator

Dragonfly Operator creates and manages instances of [DragonflyDB](https://github.com/dragonflydb/dragonfly). Beware, that this is not an officially supported project.

## Installation

### Helm Chart

Kindly have a look in [chart/dragonfly-operator](https://github.com/tamcore/dragonfly-operator/tree/master/chart/dragonfly-operator). The README there will provide you with instructions on how to install the latest version of this operator.

### Kustomize

1. Fetch a current copy of this repository
2. Build & Apply through Kustomize+kubectl
```sh
kustomize build config/default | kubectl apply -f-
```

### Deploying a Dragonfly instance

> **Tip**: In [config/samples](https://github.com/tamcore/dragonfly-operator/tree/master/config/samples) you'll find a few more samples. To deploy Dragonfly in a stateful way, or with TLS enabled for example.

To deploy a single, stateless Dragonfly instance, as much as the snipped bellow is enough.

```sh
cat <<EOC | kubectl apply -f -
---
apiVersion: dragonfly.pborn.eu/v1alpha1
kind: Dragonfly
metadata:
  name: dragonfly
  namespace: dragonfly
spec:
  replicaCount: 1
EOC
```

## License

Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

