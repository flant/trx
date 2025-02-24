# trx  

"trx" helps you release versions approved by team members in restricted environments if a quorum is reached.  
A quorum is considered reached if the required number of GPG signatures is present on the tag.  

## Table of Contents  
- [For Administrators](#for-administrators)  
  - [Project Configuration](#project-configuration)  
  - [Creating a Configuration File](#creating-a-configuration-file)  
  - [Configuring Commands](#configuring-commands)  
  - [Running the Application](#running-the-application)  
- [For Developers](#for-developers)  
  - [GPG Key Requirements](#gpg-key-requirements)  
  - [Generating a GPG Key](#generating-a-gpg-key)  
  - [Installing the Signatures Plugin](#installing-the-signatures-plugin)  
  - [Adding a Signature to a Tag](#adding-a-signature-to-a-tag)  
- [Configuration Example](#configuration-example)  

---

## For Administrators  

### Project Configuration  

To begin, clone the repository to your server:  
```sh
git clone https://fox.flant.com/deckhouse/delivery/yet-another-trdl.git
cd yet-another-trdl
```

Ensure you have Go installed (version 1.23 or later). Then, build the binary:  
```sh
cd cmd/trx
go build -o bin/trx ./cmd/trx
```

This will generate an executable file named `trx`.  
Now set up the configuration and use it based on your scenario (run manually, schedule in cron, integrate into CI, etc.).  

If you are familiar with `go-task`, check `Taskfile.dist.yaml` for common actions.  
Learn more about `task`: [taskfile.dev](https://taskfile.dev/).  

### Creating a Configuration File  

- Configure the target repository:  
```yaml
repo:
  url: "https://github.com/werf/werf.git"
  
  # Optional, required if the repository needs authentication.
  # auth:
```

- Configure quorums:  
```yaml
quorums:
  - name: main
    minNumberOfKeys: 1  
    gpgKeyPaths:
      - public.asc
```

### Configuring Commands  

Commands should be stored in `trx.yaml` inside your project's Git repository.  
There is also an option to define `commands` in the main `trx.yaml`, but this is recommended only for debugging purposes.  
If `commands` are specified in the main `trx.yaml`, commands from the repository will be ignored and vice versa.  

Example:  
```yaml
commands:
  - echo "$TEST" | base64
  - echo "{{ .RepoTag }} {{ .RepoCommit }} {{ .RepoUrl }}"
env:
  TEST: "Test"
  COMMIT: "{{ .RepoCommit }}"
```
Available template variables:  
- `{{ .RepoTag }}` – current tag  
- `{{ .RepoCommit }}` – current commit  
- `{{ .RepoUrl }}` – repository URL  

- Optionally configure `hooks`.  
See the [configuration example](#configuration-example) for details.  

### Running the Application  

The config file can be specified using the `--config` flag or the default path `./trx.yaml`.  
See the [configuration example](#configuration-example) to create a config file.  
```sh
./trx --config trx.yaml
```
To force execution even if no new version is found:  
```sh
./trx --force
```

---

## For Developers  

### GPG Key Requirements  

- **Only RSA encryption is supported for now.**  
- **Store keys in a secure location** (e.g., `~/.gnupg`).  
- **Private keys must be encrypted with a password.**  
- **Public keys must be provided to the administrator.**  

### Generating a GPG Key  

Generate a new key:  
```sh
gpg --default-new-key-algo rsa4096 --gen-key
```
List existing keys:  
```sh
gpg --list-secret-keys --keyid-format=long
```
Export a public key:  
```sh
gpg --armor --export KEY_ID > public_key.asc
```

### Installing the Signatures Plugin  

```sh
git clone https://github.com/werf/third-party-git-signatures.git
cd third-party-git-signatures
make install
```
Refer to the [official repository](https://github.com/werf/3p-git-signatures) for additional details.  

### Adding a Signature to a Tag  

After a tag is published:  

- For the initial configuration, sign the first tag:  
```sh
git signatures add --push v0.0.1
```
- For all other cases:  
```sh
git fetch --tags
git signatures pull
git signatures add --push v0.0.1
```

---

## Configuration Example  

**trx.yaml:** 

```yaml
repo:
  url: "https://github.com/werf/werf.git"
  
  # Optional, required if the repository needs authentication.
  auth:
    sshKeyPath: "/home/user/.ssh/id_rsa" 
    sshKeyPassword: "supersecret"
    basic:
      username: "gituser" 
      password: "gitpass"

  # Optional, default is `trx.yaml` in the repository.
  configFile: "trx.yaml"
  
  # Optional. Ensures processing starts from a specific tag and prevents processing older tags (safeguard against freeze attacks).
  initialLastProcessedTag: "v0.10.1"

quorums:
  - name: main
    minNumberOfKeys: 1  
    gpgKeyPaths:
      - "public_key.asc"
  - name: admin
    minNumberOfKeys: 1
    gpgKeys:
      - |
        -----BEGIN PGP PUBLIC KEY BLOCK-----
        ...
        -----END PGP PUBLIC KEY BLOCK-----

env:
  TEST: "True"

hooks:
  onCommandStarted: # Async invocation
    - "echo 'Command started: {{ .RepoTag }} at {{ .RepoCommit }}'"
  onCommandSuccess:
    - "echo 'Success: {{ .RepoTag }}'"
  onCommandFailure:
    - "echo 'Failure: {{ .RepoTag }}'"
  onCommandSkipped:
    - "echo 'Skipped: {{ .RepoTag }}'"
  onQuorumFailure:
    - "echo 'Quorum {{ .FailedQuorumName }} failed'"
```

**Commands file:**  
```yaml 
#trx.yaml
commands:
  - echo "$TEST" | base64
  - echo "{{ .RepoTag }} {{ .RepoCommit }} {{ .RepoUrl }}"
env:
  TEST: "True"
  COMMIT: "{{ .RepoCommit }}"
```