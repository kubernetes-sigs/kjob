# Installation

Installing the `kubectl-kjob` plugin, `kjobctl`.

## Installing 

### From Release Binaries

#### 1. Download the latest release:

On Linux (AMD64 / x86_64):
```bash
curl -Lo ./kubectl-kjob https://github.com/kubernetes-sigs/kjob/releases/download/v0.1.0/kubectl-kjob-linux-amd64
```

On Linux (ARM64):
```bash
curl -Lo ./kubectl-kjob https://github.com/kubernetes-sigs/kjob/releases/download/v0.1.0/kubectl-kjob-linux-arm64
```

On Mac (AMD64 / x86_64):
```bash
curl -Lo ./kubectl-kjob https://github.com/kubernetes-sigs/kjob/releases/download/v0.1.0/kubectl-kjob-darwin-amd64
```

On Mac (ARM64):
```bash
curl -Lo ./kubectl-kjob https://github.com/kubernetes-sigs/kjob/releases/download/v0.1.0/kubectl-kjob-darwin-arm64
```

### 2. Make the kubectl-kjob binary executable.

```shell
chmod +x ./kubectl-kjob
```

### 3. Move the kubectl-kjob binary to a file location on your system PATH.

```shell
sudo mv ./kubectl-kjob /usr/local/bin/kubectl-kjob
```

### From source

```bash
make kubectl-kjob
sudo mv ./bin/kubectl-kjob /usr/local/bin/kubectl-kjob
```

## Installing CRDs

### Using printcrds command

```bash
kubectl-kjob printcrds | kubectl apply --server-side -f -
```

### From source

```bash
make install
```

## Kjobctl

Additionally, you can create an alias `kjobctl` to allow shorter syntax.

```bash
echo 'alias kjobctl="kubectl kjob"' >> ~/.bashrc
# Or if you are using ZSH
echo 'alias kjobctl="kubectl kjob"' >> ~/.zshrc
```

## Autocompletion

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

## See Also

* [overview](_index.md)	 - `kubectl-kjob` plugin, `kjobctl` overview.
* [commands](commands/kjobctl.md)	 - Full list of commands, along with all possible flags.