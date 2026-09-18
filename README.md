**trx** allows executing commands using the software’s source code version (Git tag) verified by a quorum of trusted users. This guarantees that only approved versions of the code are used for operations, like deploying an application or running other critical tasks.

## Table of contents

* [Overview](#overview)
* [For a developer](#for-a-developer)
  * [Setting up a GPG signature](#setting-up-a-gpg-signature)
    * [GPG key requirements](#gpg-key-requirements)
    * [Generating a GPG Key](#generating-a-gpg-key)
    * [Installing the signatures plugin](#installing-the-signatures-plugin)
  * [Adding a signature to a tag](#adding-a-signature-to-a-tag)
  * [Configuring commands (optional)](#configuring-commands-optional)
* [For a user](#for-a-user)
  * [Creating a configuration file](#creating-a-configuration-file)
  * [Installing trx](#installing-trx)
  * [Running](#running)

## Overview

The development team releases software versions (Git tags in [SemVer format](https://semver.org/)) and signs them using GPG signatures. Along with the code, developers can provide default commands that users can execute.

End users create a project configuration specifying repository access credentials and trusted GPG key groups. The trx utility then:

1.	Fetches the latest available software version (highest [SemVer](https://semver.org/) release; pre-release tags such as `v2.0.0-rc1` are skipped unless `repo.allowPrerelease` is set).
2.	Verifies the required signatures.
3.	Executes commands in the repository root.

Commands can be provided alongside the source code by developers and/or parameterized and defined by the user.

## For a developer

### Setting up a GPG signature

#### GPG key requirements

- Recommended key algorithms: RSA and Ed25519 (EdDSA). DSA and ElGamal keys are rejected, as are signatures hashed with SHA-1 or MD5.
- Ensure keys are stored securely (e.g., in `~/.gnupg`).
- Private keys must be encrypted with a password.
- Public keys must be provided to the administrator.

#### Generating a GPG Key

Use the following command to generate an RSA4096 GPG key:

```sh
gpg --default-new-key-algo rsa4096 --gen-key
```

Or generate an Ed25519 GPG key:

```sh
gpg --default-new-key-algo ed25519 --gen-key
```

#### Installing the signatures plugin

Install the signatures plugin with:

```sh
git clone https://github.com/werf/third-party-git-signatures.git
cd third-party-git-signatures
make install
```

Refer to the [official repository](https://github.com/werf/3p-git-signatures) for additional details.

### Adding a signature to a tag

After a tag is published, use the following commands:

```sh
git fetch --tags
git signatures pull
git signatures add --push v0.0.1
```

> On first use in the Git repository, run `git signatures add --push v0.0.1`.

### Configuring commands (optional)

The `trx.yaml` file inside the project repository defines commands and environment variables, which users can override.

Example:

```yaml
commands:
  - werf converge
  - echo "{{ .RepoUrl }} / {{ .RepoTag }} / {{ .RepoCommit }}"
env:
  WERF_ENV: "production"
```

Available template variables:
- `{{ .RepoTag }}` – current tag.
- `{{ .RepoCommit }}` – current commit.
- `{{ .RepoUrl }}` – repository URL.
- `{{ .FailedQuorumName }}` – the quorum that failed verification; empty outside the `onQuorumFailure` hook.

A variable that is not on this list is an error, so a typo fails the run instead
of reaching the shell as `<no value>`.

## For a user

### Creating a configuration file

```yaml
# trx.yaml
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

  # Optional, default is false. Deploy pre-release tags (v2.0.0-rc1) as well,
  # for a repository that releases through them on purpose.
  allowPrerelease: false

# Optional. Commands defined here have a higher priority than those specified in `trx.yaml`.
commands:
  - werf converge
  - echo "{{ .RepoUrl }} / {{ .RepoTag }} / {{ .RepoCommit }}"

# Optional. Set environment variables here to be used in the commands.
# Environment variables defined here are merged with those in the configFile,
# but have higher priority (values in this section will override those in the configFile).
env:
  WERF_ENV: "production"

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

# Optional. Define actions to be taken at different stages of command execution.
# onCommandSkipped and onQuorumFailure run before the tag is verified, so their
# working directory is the one trx was started in, not the repository clone.
# The other hooks and the commands run in the clone, checked out at the verified tag.
hooks:
  onCommandStarted:
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

### Installing trx

Follow instructions on [GitHub Releases](https://github.com/flant/trx/releases).

### Running

The config file can be specified using the `--config` flag or the default path `./trx.yaml`.

```sh
trx --config trx.yaml
```

Also you can execute command from the command line:
```sh
trx --config trx.yaml -- ls -la
```

To force the execution even if no new version is detected, use the `--force` flag:

```sh
trx --force
```

A tag whose quorum verification or commands failed is not retried by itself: trx
exits with an error and does not run the hooks again until a newer tag is pushed,
or until the tag is retried explicitly with `--force`.
