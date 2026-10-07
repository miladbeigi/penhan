# Commands

All commands except `add`, `access`, `unlock`, `update`, and `version` run inside a safe directory, the one containing `penhan.yaml`.

| Command | Purpose |
|---|---|
| [`add`](#penhan-add) | Create a safe |
| [`encrypt`](#penhan-encrypt) | Encrypt secret files for committing to Git |
| [`decrypt`](#penhan-decrypt) | Decrypt `.enc` files for editing |
| [`check`](#penhan-check) | Compare local secrets with the backend |
| [`push`](#penhan-push) | Write new and changed secrets to the backend |
| [`import`](#penhan-import) | Bring existing Kubernetes Secrets into a safe |
| [`access`](#penhan-access) | Share the master key through GitHub SSH keys |
| [`unlock`](#penhan-unlock) | Decrypt the master key with your SSH key |
| [`wrap`](#penhan-wrap) | Encrypt safe keys with the master key so they can be committed |
| [`update`](#penhan-update) | Update Penhan to the latest release |
| [`version`](#penhan-version) | Print version information |

## `penhan add`

```
penhan add [name] [flags]
```

Creates a safe named `name` in the current directory: its `penhan.yaml`, `secrets/` directory, encryption key, and backend settings, and adds the matching `.gitignore` entries. Without flags it prompts for each setting. When stdin isn't a terminal, it requires every flag and fails with the list of missing ones.

| Flag | Description |
|---|---|
| `--encryption` | `aes` or `gpg`. See [Encryption](encryption.md). |
| `--backend` | `vault`, `kubernetes`, or `file` |
| `--vault-addr` | Vault address, including the scheme, e.g. `https://vault.example.com` |
| `--vault-token-file` | File containing the Vault token, copied into the safe |
| `--vault-token` | Vault token. Visible in shell history; prefer `--vault-token-file`. |
| `--kube-namespace` | Namespace to write Secrets to (required for `kubernetes`) |
| `--kube-context` | Kubeconfig context to pin. Defaults to the current context. |
| `--kubeconfig` | Kubeconfig path. Defaults to `$KUBECONFIG` or `~/.kube/config`. |
| `--remote-dir` | Directory for the `file` backend. Defaults to `.penhan/remote`. |

Safe names start with a letter or digit and contain only letters, digits, `-`, and `_`.

## `penhan encrypt`

```
penhan encrypt [file|dir]...
```

Encrypts each plaintext secret file (`.yaml`, `.yml`, `.json`) to a `.enc` file next to it and removes the plaintext. With no arguments, it encrypts the whole `secrets/` directory. Other files, such as `.gitkeep`, are left alone.

Encryption is randomized, so the same plaintext produces different bytes each time. If a `.enc` file already decrypts to exactly the plaintext, it's kept as is, so unchanged secrets don't show up in `git diff`.

## `penhan decrypt`

```
penhan decrypt [file|dir]...
```

Writes the plaintext of each `.enc` file next to it, readable only by you, for editing. The `.enc` file is kept. With no arguments, it decrypts the whole `secrets/` directory.

If a plaintext file already exists with different content, `decrypt` stops rather than overwrite your edits. Encrypt or remove it first.

## `penhan check`

```
penhan check
```

Compares every local secret, plaintext or encrypted, with the backend and reports each one as `new`, `changed`, or `unchanged`. It writes nothing, locally or remotely.

## `penhan push`

```
penhan push
```

Runs the same comparison as `check`, then writes every `new` and `changed` secret to the backend, reporting each one. Unchanged secrets are skipped. The local file is the source of truth: `push` overwrites edits made directly in the backend.

## `penhan import`

```
penhan import [secret]... [--all]
```

Kubernetes backend only. With no arguments, lists every Secret in the safe's namespace as `ready`, `present` (already in the safe), or `skip` with the reason, and changes nothing. With names, or `--all` for everything that's ready, it writes each Secret to an encrypted `secrets/<name>.yaml.enc` and marks the Secret as managed by the safe. The Secret's data is not changed. See [Importing existing Secrets](backends/kubernetes.md#importing-existing-secrets).

## `penhan access`

```
penhan access grant <github-user> [--key SHA256:...] [--yes]
penhan access revoke <github-user>
penhan access list
```

Manages who can unlock the project's master key. Run it at the project root. See [Sharing keys with a master key](master-key.md).

- `grant` fetches the user's SSH keys from GitHub, pins them in `.penhan/access.yaml`, and re-encrypts `.penhan/master.age`. The first grant creates the master key. `--key` limits the grant to the given fingerprints. If the user's keys changed since they were pinned, `grant` shows the change and asks first; `--yes` accepts it without asking.
- `revoke` removes the user, rotates the master key, and re-wraps every safe key with the new one.
- `list` shows each user's pinned keys, marking the ones on this machine.

## `penhan unlock`

```
penhan unlock [--identity <private key>]...
```

Decrypts `.penhan/master.age` with your SSH private key and saves the master key to `.penhan/master.key` (gitignored). Works anywhere in the project. By default it uses the key pairs in `~/.ssh`.

## `penhan wrap`

```
penhan wrap
```

Encrypts a safe's key with the master key into `.penhan/<method>.key.enc`, to be committed. Inside a safe it wraps that safe; at the project root it wraps every safe. Safes that are already wrapped are left alone. `penhan add` does this automatically once a master key exists.

## `penhan update`

```
penhan update [--check] [--force]
```

Downloads the latest release for your OS and architecture and verifies it against the release's `checksums.txt`. It runs the new binary once to confirm it works, then replaces the installed binary atomically. If Penhan is installed in a directory you can't write to, run it with `sudo`.

| Flag | Description |
|---|---|
| `--check` | Only report whether a newer release exists |
| `--force` | Replace a development build (e.g. one built from source) with the latest release. `update` never installs a release older than the running build, even with `--force`. |

In an interactive terminal, Penhan checks for a new release at most once a day and prints a one-line notice. It never installs anything on its own. The check is skipped when output isn't a terminal or `CI` is set. Set `PENHAN_NO_UPDATE_NOTIFIER=1` to disable it.

## `penhan version`

```
penhan version
```

Prints the version, commit, build date, Go version, and platform.
