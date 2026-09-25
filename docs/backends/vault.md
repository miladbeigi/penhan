# Vault backend

Pushes each secret to a [HashiCorp Vault](https://www.vaultproject.io/) KV version 2 mount. Applications read secrets from Vault as usual.

## Setup

```bash
penhan add myapp --encryption=aes --backend=vault \
  --vault-addr=https://vault.example.com --vault-token-file=./vault-token
```

The token is copied to `myapp/.penhan/vault-token`, which is gitignored. Each teammate needs their own token file there.

## Path mapping

Secrets are written to `{mount_path}/data/{base_path}/{secret path}`. `add` sets `base_path` to the safe name, so several safes can share one Vault without colliding:

| Local file | Vault path |
|---|---|
| `secrets/db.yaml` | `secret/data/myapp/db` |
| `secrets/db/main.yaml` | `secret/data/myapp/db/main` |
| `secrets/api/key.yaml` | `secret/data/myapp/api/key` |

Each key in the file becomes a key in the Vault secret. A push replaces the whole secret, so a key removed from the file is removed in Vault too.

## Permissions

The token needs `read`, `create`, and `update` on the safe's paths. A minimal policy for a safe named `myapp` on the `secret` mount:

```hcl
path "secret/data/myapp/*" {
  capabilities = ["create", "read", "update"]
}
```

## Notes

- Only KV version 2 mounts are supported.
- A soft-deleted secret is reported as `new`, and `push` writes a new version.
- Penhan never deletes secrets from Vault. Removing a local file leaves the Vault secret in place.
