# Renders docs/demo/demo.gif. Regenerate with `make demo` (needs vhs, Docker,
# ssh-keygen, and python3).
#
# Setup runs hidden: a throwaway Vault dev server in Docker, a scratch
# project directory, and a `vault` shell function that runs the Vault CLI
# inside that container. HOME points at the scratch directory, with a freshly
# generated SSH key for the demo user "alice", and PENHAN_GITHUB_URL points
# `penhan access grant` at a local server that plays github.com/alice.keys.
# Nothing outside the scratch directory is touched.

Output docs/demo/demo.gif

Require penhan
Require docker
Require ssh-keygen
Require python3

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
Type "docker rm -f penhan-demo-vault >/dev/null 2>&1; docker run -d --name penhan-demo-vault -p 127.0.0.1:8201:8200 -e VAULT_DEV_ROOT_TOKEN_ID=demo-token hashicorp/vault:1.21 >/dev/null" Enter
Type "until curl -sf http://127.0.0.1:8201/v1/sys/health >/dev/null; do sleep 0.5; done" Enter
Type `D=$(mktemp -d /tmp/penhan.XXXX) && mkdir -p $D/home/.ssh $D/github && export DOCKER_CONFIG=$HOME/.docker HOME=$D/home PENHAN_GITHUB_URL=http://127.0.0.1:8399 GIT_AUTHOR_NAME=alice GIT_AUTHOR_EMAIL=alice@example.com GIT_COMMITTER_NAME=alice GIT_COMMITTER_EMAIL=alice@example.com` Enter
Type `ssh-keygen -q -t ed25519 -N '' -C alice -f ~/.ssh/id_ed25519 && cp ~/.ssh/id_ed25519.pub $D/github/alice.keys` Enter
Type `python3 -m http.server -d $D/github -b 127.0.0.1 8399 >/dev/null 2>&1 & GH=$!` Enter
Type "mkdir -p $D/project && cd $_ && git init -q && echo demo-token > token && echo token > .git/info/exclude" Enter
Type `vault() { docker exec -e VAULT_TOKEN=demo-token -e VAULT_ADDR=http://127.0.0.1:8200 penhan-demo-vault vault "$@"; echo; }` Enter
Sleep 4s
Type "clear" Enter
Show

Type "# Create a safe: its own key, config, and backend (here, Vault)"
Enter
Sleep 400ms
Type "penhan add myapp --encryption=aes --backend=vault \"
Enter
Type "--vault-addr=http://127.0.0.1:8201 --vault-token-file=token"
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
Sleep 3s

Type "clear"
Enter
Type "# Share safe keys with a master key, unlocked by GitHub SSH keys"
Enter
Sleep 400ms
Type "cd .. && penhan access grant alice"
Enter
Sleep 2.5s
Type "penhan wrap && git add -A && git commit -qm 'Add myapp'"
Enter
Sleep 2s

Type "# A fresh clone has no keys until you unlock it with your SSH key"
Enter
Sleep 400ms
Type "git clone -q . ../laptop && cd ../laptop && penhan unlock"
Enter
Sleep 2.5s
Type "cd myapp && penhan decrypt && cat secrets/db.yaml"
Enter
Sleep 5s

Hide
Type "kill $GH; cd /; rm -rf $D; docker rm -f penhan-demo-vault >/dev/null" Enter
Sleep 1s
