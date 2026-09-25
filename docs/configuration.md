# Configuration

Each safe is configured by the `penhan.yaml` in its directory. `penhan add` writes it, and it's committed alongside the encrypted secrets. Paths are relative to the safe directory.

## Example

```yaml
encryption:
  method: aes
  aes:
    key_path: .penhan/keys/aes.key

backend:
  type: vault
  vault:
    addr: https://vault.example.com
    token_path: .penhan/vault-token
    mount_path: secret
    base_path: myapp

secrets:
  path: secrets/
  format: yaml
```

## `encryption`

| Field | Description |
|---|---|
| `method` | `aes` or `gpg` |
| `aes.key_path` | Path to the AES key, when `method: aes` |
| `gpg.key_path` | Path to the GPG private key, when `method: gpg` |

See [Encryption and keys](encryption.md).

## `backend`

| Field | Description |
|---|---|
| `type` | `vault`, `kubernetes`, or `file` |

### `backend.vault`

| Field | Description |
|---|---|
| `addr` | Vault address, including the scheme |
| `token_path` | File holding the Vault token (gitignored) |
| `mount_path` | KV v2 mount, e.g. `secret` |
| `base_path` | Prefix under the mount. `add` sets it to the safe name. |

### `backend.kubernetes`

| Field | Description |
|---|---|
| `context` | Kubeconfig context, pinned by `add`. Every command uses this context, whatever the current context is. |
| `namespace` | Namespace the Secrets live in |
| `safe` | Safe name, recorded on each Secret to mark ownership |
| `kubeconfig` | Optional kubeconfig path. Defaults to `$KUBECONFIG` or `~/.kube/config`. |

### `backend.file`

| Field | Description |
|---|---|
| `path` | Directory for the encrypted copies. Defaults to `.penhan/remote`. |

## `secrets`

| Field | Description |
|---|---|
| `path` | Directory holding the secret files |
| `format` | Written by `add`; not currently used. Files may be `.yaml`, `.yml`, or `.json`. |
