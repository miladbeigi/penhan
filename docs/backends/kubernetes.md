# Kubernetes backend

Pushes each secret as a native `Opaque` Secret in one namespace, with one data key per key in the file. Pods consume them as usual, through volume mounts or `envFrom`.

## Setup

```bash
penhan add myapp --encryption=aes --backend=kubernetes --kube-namespace=myapp
```

penhan uses your kubeconfig: `--kubeconfig`, `$KUBECONFIG`, or `~/.kube/config`. No cluster credentials are stored in the safe. The namespace must already exist.

## Name mapping

The Secret name is the secret path with `/` replaced by `-`:

| Local file | Secret |
|---|---|
| `secrets/db.yaml` | `db` |
| `secrets/db/main.yaml` | `db-main` |
| `secrets/api-token.yaml` | `api-token` |

Names must be valid Kubernetes names: lowercase letters, digits, `-`, and `.`. A file like `secrets/DB_Password.yaml` is rejected with an error rather than renamed, because renaming could map two files to the same Secret.

## Guardrails

- **The context is pinned.** `add` records the kubeconfig context (the current one, or `--kube-context`) in `penhan.yaml`. Every later command uses that context, even after `kubectl config use-context`. If a teammate's kubeconfig doesn't have the context, penhan fails instead of falling back to another cluster.
- **Ownership is enforced.** Every Secret penhan writes has the label `app.kubernetes.io/managed-by: penhan` and the annotations `penhan/safe` and `penhan/path`. `check` and `push` refuse to touch a Secret that something else created, that belongs to another safe, or that holds a different path mapping to the same name (e.g. `db/main.yaml` and `db-main.yaml`).
- **Pushes replace data.** A key removed from the local file is removed from the Secret. Labels and annotations added by other tools are kept.
- **Nothing is deleted.** Removing a local file leaves the Secret in place.

penhan decrypts secrets locally and sends them to the API server over TLS. Encrypting Secrets at rest inside the cluster is the cluster's job; see [Encrypting confidential data at rest](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/).

## Importing existing Secrets

To bring Secrets that were created by hand under a safe, run `penhan import` inside it. Without arguments, it only lists what's in the namespace:

```console
$ penhan import
  ready     api-key (1 key(s))
  skip      ghcr: type kubernetes.io/dockerconfigjson is not supported (penhan manages Opaque Secrets only)
  skip      sh.helm.release.v1.myapp.v1: type helm.sh/release.v1 is not supported (penhan manages Opaque Secrets only)
  ready     db-credentials (2 key(s))

2 secret(s) ready to import: run `penhan import <name>...` or `penhan import --all`
```

Then import by name, or everything that's ready:

```bash
penhan import api-key          # or: penhan import --all
penhan check                   # reports it as unchanged
git add -A && git commit -m "Import api-key"
```

For each imported Secret, penhan:

1. writes it straight to an encrypted `secrets/<name>.yaml.enc`, so the plaintext never touches disk
2. checks the file reads back as exactly the Secret's data
3. adds the penhan label and annotations to the Secret

The Secret's data, type, and other labels aren't changed, so workloads using it aren't restarted. If the data changes while the import runs, penhan stops instead of marking the Secret.

penhan never imports:

- non-`Opaque` Secrets, such as image pull secrets or TLS Secrets
- Secrets managed by Helm, Argo CD, or another tool, or with an owner reference
- Secrets belonging to another safe
- Secrets holding binary values

## Permissions

penhan needs `get`, `list`, `create`, and `update` on Secrets in the namespace (`list` is only used by `import`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: penhan
  namespace: myapp
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "create", "update"]
```
