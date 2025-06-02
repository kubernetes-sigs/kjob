# kjob

Run AI/ML jobs based on the pre-defined templates

## Description
This kubectl plugin allows ML researchers to run templated Jobs with different values
for the application-specific parameters, without the need to edit and submit entire
YAML files. The tool comes with built-in support for running slurm scripts inside
a Kubernetes cluster.

Read the [overview](docs/_index.md) to learn more.

## Getting Started

### Prerequisites
- go version v1.24+
- kubectl version v1.29+.
- Access to a Kubernetes v1.29+ cluster.

### To Install

**Install the CRDs into the cluster:**

```sh
make install
```

**Install `kubectl kjob` plugin:**

```sh
make kubectl-kjob
sudo cp ./bin/kubectl-kjob /usr/local/bin/kubectl-kjob
```

**Additionally, you can create an alias `kjobctl` to allow shorter syntax:**

```sh
echo 'alias kjobctl="kubectl kjob"' >> ~/.bashrc
# Or if you are using ZSH
echo 'alias kjobctl="kubectl kjob"' >> ~/.zshrc
```

**Autocompletion:**

```bash
echo '[[ $commands[kubectl-kjob] ]] && source <(kubectl-kjob completion bash)' >> ~/.bashrc
# Or if you are using ZSH
echo '[[ $commands[kubectl-kjob] ]] && source <(kubectl-kjob completion zsh)' >> ~/.zshrc

cat <<EOF >kubectl_complete-kjob
#!/usr/bin/env sh

# Call the __complete command passing it all arguments
kubectl kjob __complete "\$@"
EOF

chmod u+x kubectl_complete-kjob
sudo mv kubectl_complete-kjob /usr/local/bin/kubectl_complete-kjob
```

### To Uninstall

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**Delete `kubectl kjob` plugin:**

```sh
sudo rm /usr/local/bin/kubectl-kjob
```

**NOTE:** Run `make help` for more information on all potential `make` targets

## Production Readiness status

- ✔️ Test Coverage:
  - ✔️ Unit Test [testgrid](https://testgrid.k8s.io/sig-apps#periodic-kjob-test-unit-main).
  - ✔️ Integration Test [testgrid](https://testgrid.k8s.io/sig-apps#periodic-kjob-test-integration-main)
  - ✔️ E2E Tests for Kubernetes
    [1.30](https://testgrid.k8s.io/sig-apps#periodic-kjob-test-e2e-main-1-30),
    [1.31](https://testgrid.k8s.io/sig-apps#periodic-kjob-test-e2e-main-1-31),
    [1.32](https://testgrid.k8s.io/sig-apps#periodic-kjob-test-e2e-main-1-32),
    [1.33](https://testgrid.k8s.io/sig-apps#periodic-kjob-test-e2e-main-1-33),
    on Kind.

## License

Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- [Slack](https://kubernetes.slack.com/messages/sig-apps)
- [Mailing List](https://groups.google.com/a/kubernetes.io/g/sig-apps)


### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
