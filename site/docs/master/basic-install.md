# Basic Install

- [Basic Install](#basic-install)
  - [Prerequisites](#prerequisites)
  - [Install the CLI](#install-the-cli)
    - [Option 1: macOS - Homebrew](#option-1-macos---homebrew)
    - [Option 2: GitHub release](#option-2-github-release)
  - [Install and configure the server components](#install-and-configure-the-server-components)

Use this doc to get a basic installation of Velero.
Refer [this document](customize-installation.md) to customize your installation.

## Prerequisites

- Access to a Kubernetes cluster, v1.10 or later, with DNS and container networking enabled.
- `kubectl` installed locally

Velero uses object storage to store backups and associated artifacts. It also optionally integrates with supported block storage systems to snapshot your persistent volumes. Before beginning the installation process, you should identify the object storage provider and optional block storage provider(s) you'll be using from the list of [compatible providers][0].

There are supported storage providers for both cloud-provider environments and on-premises environments. For more details on on-premises scenarios, see the [on-premises documentation][2].

## Install the CLI

### Option 1: macOS - Homebrew

On macOS, you can use [Homebrew](https://brew.sh) to install the `velero` client:

```bash
brew install velero
```

### Option 2: GitHub release

1. Download the [latest release][1]'s tarball for your client platform.
1. Extract the tarball:

   ```bash
   tar -xvf <RELEASE-TARBALL-NAME>.tar.gz
   ```

1. Move the extracted `velero` binary to somewhere in your `$PATH` (e.g. `/usr/local/bin` for most users).

## Install and configure the server components

There are two supported methods for installing the Velero server components:

- the `velero install` CLI command
- the [Helm chart](https://github.com/helm/charts/tree/master/stable/velero)

Velero uses storage provider plugins to integrate with a variety of storage systems to support backup and snapshot operations. The steps to install and configure the Velero server components along with the appropriate plugins are specific to your chosen storage provider. To find installation instructions for your chosen storage provider, follow the documentation link for your provider at our [supported storage providers][0] page

_Note: if your object storage provider is different than your volume snapshot provider, follow the installation instructions for your object storage provider first, then return here and follow the instructions to [add your volume snapshot provider][4]._

## Optional Velero CLI configurations

### Enabling shell autocompletion

**Velero CLI** provides autocompletion support for `Bash` and `Zsh`, which can save you a lot of typing.

Below are the procedures to set up autocompletion for `Bash` (including the difference between `Linux` and `macOS`) and `Zsh`.

## Bash on Linux

The **Velero CLI** completion script for `Bash` can be generated with the command `velero completion bash`. Sourcing the completion script in your shell enables velero autocompletion.

However, the completion script depends on [**bash-completion**](https://github.com/scop/bash-completion), which means that you have to install this software first (you can test if you have bash-completion already installed by running `type _init_completion`).

### Install bash-completion

`bash-completion` is provided by many package managers (see [here](https://github.com/scop/bash-completion#installation)). You can install it with `apt-get install bash-completion` or `yum install bash-completion`, etc.

The above commands create `/usr/share/bash-completion/bash_completion`, which is the main script of bash-completion. Depending on your package manager, you have to manually source this file in your `~/.bashrc` file.

To find out, reload your shell and run `type _init_completion`. If the command succeeds, you're already set, otherwise add the following to your `~/.bashrc` file:

```shell
source /usr/share/bash-completion/bash_completion
```

Reload your shell and verify that bash-completion is correctly installed by typing `type _init_completion`.

### Enable Velero CLI autocompletion for Bash on Linux

You now need to ensure that the **Velero CLI** completion script gets sourced in all your shell sessions. There are two ways in which you can do this:

- Source the completion script in your `~/.bashrc` file:

    ```shell
    echo 'source <(velero completion bash)' >>~/.bashrc
    ```

- Add the completion script to the `/etc/bash_completion.d` directory:

    ```shell
    velero completion bash >/etc/bash_completion.d/velero
    ```

- If you have an alias for velero, you can extend shell completion to work with that alias:

    ```shell
    echo 'alias v=velero' >>~/.bashrc
    echo 'complete -F __start_velero v' >>~/.bashrc
    ```

> `bash-completion` sources all completion scripts in `/etc/bash_completion.d`.

Both approaches are equivalent. After reloading your shell, velero autocompletion should be working.

## Bash on macOS

The **Velero CLI** completion script for Bash can be generated with `velero completion bash`. Sourcing this script in your shell enables velero completion.

However, the velero completion script depends on [**bash-completion**](https://github.com/scop/bash-completion) which you thus have to previously install.


> There are two versions of bash-completion, v1 and v2. V1 is for Bash 3.2 (which is the default on macOS), and v2 is for Bash 4.1+. The velero completion script **doesn't work** correctly with bash-completion v1 and Bash 3.2. It requires **bash-completion v2** and **Bash 4.1+**. Thus, to be able to correctly use velero completion on macOS, you have to install and use Bash 4.1+ ([*instructions*](https://itnext.io/upgrading-bash-on-macos-7138bd1066ba)). The following instructions assume that you use Bash 4.1+ (that is, any Bash version of 4.1 or newer).


### Install bash-completion

> As mentioned, these instructions assume you use Bash 4.1+, which means you will install bash-completion v2 (in contrast to Bash 3.2 and bash-completion v1, in which case kubectl completion won't work).

You can test if you have bash-completion v2 already installed with `type _init_completion`. If not, you can install it with Homebrew:

  ```shell
  brew install bash-completion@2
  ```

As stated in the output of this command, add the following to your `~/.bashrc` file:

  ```shell
  export BASH_COMPLETION_COMPAT_DIR="/usr/local/etc/bash_completion.d"
  [[ -r "/usr/local/etc/profile.d/bash_completion.sh" ]] && . "/usr/local/etc/profile.d/bash_completion.sh"
  ```

Reload your shell and verify that bash-completion v2 is correctly installed with `type _init_completion`.

### Enable Velero CLI autocompletion for Bash on macOS

You now have to ensure that the velero completion script gets sourced in all your shell sessions. There are multiple ways to achieve this:

- Source the completion script in your `~/.bashrc` file:

    ```shell
    echo 'source <(velero completion bash)' >>~/.bashrc

    ```

- Add the completion script to the `/usr/local/etc/bash_completion.d` directory:

    ```shell
    velero completion bash >/usr/local/etc/bash_completion.d/velero
    ```

- If you have an alias for velero, you can extend shell completion to work with that alias:

    ```shell
    echo 'alias v=velero' >>~/.bashrc
    echo 'complete -F __start_velero v' >>~/.bashrc
    ```

- If you installed velero with Homebrew (as explained [above](#install-with-homebrew-on-macos)), then the velero completion script should already be in `/usr/local/etc/bash_completion.d/velero`. In that case, you don't need to do anything.

> The Homebrew installation of bash-completion v2 sources all the files in the `BASH_COMPLETION_COMPAT_DIR` directory, that's why the latter two methods work.

In any case, after reloading your shell, velero completion should be working.


## Autocompletion on Zsh

The velero completion script for Zsh can be generated with the command `velero completion zsh`. Sourcing the completion script in your shell enables velero autocompletion.

To do so in all your shell sessions, add the following to your `~/.zshrc` file:

  ```shell
  source <(velero completion zsh)
  ```

If you have an alias for kubectl, you can extend shell completion to work with that alias:

  ```shell
  echo 'alias v=velero' >>~/.zshrc
  echo 'complete -F __start_velero v' >>~/.zshrc
  ```

After reloading your shell, kubectl autocompletion should be working.

If you get an error like `complete:13: command not found: compdef`, then add the following to the beginning of your `~/.zshrc` file:

  ```shell
  autoload -Uz compinit
  compinit
  ```

[0]: supported-providers.md
[1]: https://github.com/vmware-tanzu/velero/releases/latest
[2]: on-premises.md
[3]: overview-plugins.md
[4]: customize-installation.md#install-an-additional-volume-snapshot-provider
