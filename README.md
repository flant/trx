# TRX

## Getting Started

"TRX" helps you release versions approved by team members in restricted environments if a quorum is reached.  
A quorum is considered reached if the required number of GPG signatures is present on the tag.

## Admin Guide

### Clone the Repository  
To begin, clone the repository to your server:
```sh
git clone https://fox.flant.com/deckhouse/delivery/yet-another-trdl.git
cd yet-another-trdl
```

### Build the Binary  
Ensure you have Go installed (version 1.23 or later). Then, build the binary:
```sh
cd cmd/trx
go build -o trx main.go
```

This will generate an executable file named `trx`.  
Now set up the configuration and use it based on your scenario (run manually, schedule in cron, integrate into CI, etc.).

If you are familliar with `go-task` look into `Taskfile.dist.yaml` for most common actions.
See more about `task`: https://taskfile.dev/

## Configure Git Signatures

### Setting Up a GPG Signature in Git
Git provides a mechanism for signing new tags (releases) and individual commits. The GPG signature becomes an integral part of the Git tag or commit. However, this approach supports only one signature.

The [signatures](https://github.com/werf/third-party-git-signatures) plugin allows signing Git tags and commits after they are created. In this case, GPG signatures are stored in Git notes. Multiple signatures can be used without affecting the linked Git tag or commit.

To create GPG signatures, set up GPG and Git correctly using this [guide](https://git-scm.com/book/en/v2/Git-Tools-Signing-Your-Work#_gpg_introduction).

> **IMPORTANT NOTE:** Only RSA keys currently supported.

### Installing the Signatures Plugin
Install the plugin in a directory included in `PATH` (e.g., `~/bin`):
```sh
git clone https://github.com/werf/third-party-git-signatures.git
cd third-party-git-signatures
install bin/git-signatures ~/bin
```

### Adding a Signature to a Tag
Once a Git tag is published, it must be signed by a sufficient number of trusted GPG keys. Each quorum member specified in the plugin configuration must sign the Git tag and publish their GPG signature:
```sh
git fetch --tags
git signatures pull
git signatures add --push v0.0.1
```

> **IMPORTANT NOTE:** Tags should satisfy semver notation. Read more about semver [here](https://semver.org/)

## Configure Quorum Runner

The config file can be specified using the `--config` flag or the default path `./trx.yaml`.

### Configure the Git Repository

You can specify a Git repository to be cloned via HTTP/HTTPS:
```yaml
repo:
  url: "https://github.com/werf/werf.git"
```

or via SSH:
```yaml
repo:
  url: "git@github.com:werf/werf.git"
```

If authentication is required:
- **Cloning via HTTP:**
```yaml
repo:
  url: "https://github.com/werf/werf.git"
  auth:
    basic:
      username: "username"
      password: "password"
```

- **Cloning via SSH:**
```yaml
repo:
  url: "git@github.com:werf/werf.git"
  auth:
    sshKeyPath: "/path/to/key"
    sshKeyPassword: <optional>
```

### Configure Last Published Tag
You can track the last published commit to prevent redundant executions:
```yaml
repo:
  url: "https://github.com/werf/werf.git"
  auth:
    basic:
      username: "username"
      password: "password"
  initialLastprocessedTag: 'v0.0.0'
```

From this point, the task runner will track the last published commit in local storage and skip execution if the version is less or equal than `initialLastprocessedTag` or last successed tag. If you change the commit under the tag trx will NOT perform the task. Tags considered as immutable.
You can configure corresponding hooks for this event.

### Configure Quorums
Specify quorum requirements and member keys:
```yaml
quorums:
  - name: developers
    minNumberOfKeys: 3
    gpgKeyPaths:
      - "public_key.asc"
    gpgKeys:
      - |
        -----BEGIN PGP PUBLIC KEY BLOCK-----
        ...
        -----END PGP PUBLIC KEY BLOCK-----
      - |
        -----BEGIN PGP PUBLIC KEY BLOCK-----
        ...
        -----END PGP PUBLIC KEY BLOCK-----
  - name: managers
    minNumberOfKeys: 3
    gpgKeyPaths:
      - "public_key.asc"
```

To export a GPG key, use:
```sh
gpg --armor --export E222D5A4896356FA5ABC8FA8E675FCC70C91EC4B
```
Or save it to a file:
```sh
gpg --armor --export E222D5A4896356FA5ABC8FA8E675FCC70C91EC4B > public_key.asc
```

### Configure Commands to Run

By default commands should be specified in `trx-cfg.yaml` file located in your git repository root.
If you want to rewrite default location use `commandsFilePath` directive in config file (default: `trx.yaml`).
Ensure that you specify the path relative to your repository location.

> **NOTE:** `trx.yaml` usually manged by dev team to describe overall process while `trx-cfg.yaml` is supposed to be managed by operations team since they know how exactly to deploy(deliver)

```yaml
#trx-cfg.yaml
commands:
  - echo "$TEST" | base64
  - echo "{{ .RepoTag }} {{ .RepoCommit }} {{ .RepoUrl }}"
  - /Users/flant/test.sh
```

> **NOTE:** You can use template variables `{{ .RepoTag }}`, `{{ .RepoCommit }}`, `{{ .RepoUrl }}`.

Optionally `commands` could be specified in `trx.yaml`. Configuration is the same as for `trx-cfg.yaml`

```yaml
#trx.yaml
repo:
  url: "https://github.com/werf/werf.git"
commands:
  - echo "$TEST" | base64
  - echo "{{ .RepoTag }} {{ .RepoCommit }} {{ .RepoUrl }}"
  - /Users/flant/test.sh
```
We recommend to use it only for debugging or initial setup

### Configure Environment Variables
Set environment variables for command execution:
```yaml
env:
  TEST: "Test"
  TEST2: "tset"
```
> **NOTE:** Variables are forced to uppercase. Existing OS variables are appended.

### Configure Hooks
Define hooks for specific events:
```yaml
hooks:
  onCommandSkipped:
    - "echo SKIPPED"
```
Available hooks:
- `onCommandSuccess` - When a command runs successfully.
- `onCommandFailure` - When a command fails.
- `onCommandSkipped` - When a command is skipped (e.g., tag is already released).
- `onQuorumFailure` - When quorum requirements are not met. `{{ .FailedQuorumName }}` is available

> **NOTE:** Hooks support template variables like `{{ .RepoTag }}`, `{{ .RepoCommit }}`, `{{ .RepoUrl }}`.


## Configuration Example
See `trx.yaml` for reference.

```yaml
repo:
  url: "https://github.com/werf/werf.git"
  auth: #optional, if repo requires no auth
    sshKeyPath: "/home/user/.ssh/id_rsa" 
    sshKeyPassword: "supersecret" #optional
    basic: 
      username: "gituser" #any string
      password: "gitpassword" #optional
  initialLastprocessedTag: 'v0.0.0' #optional

quorums:
  - name: main #optional
    minNumberOfKeys: 1
    gpgKeyPaths:
      - "public_key.asc"
  - name: backup #optional
    minNumberOfKeys: 1
    gpgKeys:
      - |
        -----BEGIN PGP PUBLIC KEY BLOCK-----
        ...
        -----END PGP PUBLIC KEY BLOCK-----

commandsFilePath: #optional, default is runner-cmd.yaml in repo

# optional parameters
env:
  TEST: "True"
hooks:
  onCommandSkiped:
    - "echo skipped"
  onCommandSuccess:
    - "echo success"