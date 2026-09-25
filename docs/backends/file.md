# File backend

Writes an encrypted copy of each secret to a directory instead of a remote service. It's useful for trying Penhan without any infrastructure, or when another tool picks up the files.

## Setup

```bash
penhan add myapp --encryption=aes --backend=file
```

Copies are written under `.penhan/remote` in the safe, or the directory given with `--remote-dir`. Each secret is stored at `{path}/{secret path}.enc`, encrypted with the safe's key, so `secrets/db/main.yaml` becomes `.penhan/remote/db/main.enc`.

The directory isn't gitignored, so you can commit it or point it somewhere else.
