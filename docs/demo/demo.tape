# Renders docs/demo/demo.gif. Regenerate with `make demo` (needs vhs and Docker).
#
# Setup runs hidden: a throwaway Vault dev server in Docker, a scratch
# project directory, and a `vault` shell function that runs the Vault CLI
# inside that container. Nothing outside the scratch directory is touched.

Output docs/demo/demo.gif

Require penhan
Require docker

Set Shell "bash"
Set FontSize 15
Set Width 1200
Set Height 720
Set Padding 24
Set Theme "Catppuccin Mocha"
Set TypingSpeed 35ms
Set WindowBar Colorful
Set CursorBlink false

Hide
Type `export PS1='\[\e[38;5;111m\]\W\[\e[0m\] $ ' PS2='    ' HISTFILE=/dev/null PENHAN_NO_UPDATE_NOTIFIER=1` Enter
Type "docker rm -f penhan-demo-vault >/dev/null 2>&1; docker run -d --name penhan-demo-vault -p 127.0.0.1:8200:8200 -e VAULT_DEV_ROOT_TOKEN_ID=demo-token hashicorp/vault:1.21 >/dev/null" Enter
Type "until curl -sf http://127.0.0.1:8200/v1/sys/health >/dev/null; do sleep 0.5; done" Enter
Type "mkdir -p $(mktemp -d)/project && cd $_ && git init -q && echo demo-token > token && echo token > .git/info/exclude" Enter
Type `vault() { docker exec -e VAULT_TOKEN=demo-token -e VAULT_ADDR=http://127.0.0.1:8200 penhan-demo-vault vault "$@"; echo; }` Enter
Sleep 4s
Type "clear" Enter
Show

Type "# Create a safe: its own key, config, and backend (here, Vault)"
Enter
Sleep 400ms
Type "penhan add myapp --encryption=aes --backend=vault \"
Enter
Type "--vault-addr=http://127.0.0.1:8200 --vault-token-file=token"
Enter
Sleep 2s

Type "cd myapp && echo 'password: s3cr3t-0012' > secrets/db.yaml"
Enter
Sleep 800ms

Type "# Encrypt it; only ciphertext is ever committed to git"
Enter
Sleep 400ms
Type "penhan encrypt"
Enter
Sleep 1s
Type "git status --short -uall"
Enter
Sleep 2.5s

Type "# Compare with Vault, then push what changed"
Enter
Sleep 400ms
Type "penhan check"
Enter
Sleep 1.5s
Type "penhan push"
Enter
Sleep 2s

Type "vault kv get -mount=secret -field=password myapp/db"
Enter
Sleep 2s
Type "penhan check"
Enter
Sleep 4s

Hide
Type "docker rm -f penhan-demo-vault >/dev/null" Enter
Sleep 1s
