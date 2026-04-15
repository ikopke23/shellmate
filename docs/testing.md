# Local Testing with Wish SSH

## Automated Tests: `testsession` package

The `github.com/charmbracelet/wish/testsession` package provides programmatic SSH testing without needing a real SSH daemon.

`testsession.New` starts the wish server on a random port, connects a client, and registers cleanup via `t.Cleanup()` — no manual teardown needed.

```go
import (
    "testing"

    gossh "golang.org/x/crypto/ssh"
    "github.com/charmbracelet/ssh"
    "github.com/charmbracelet/wish"
    "github.com/charmbracelet/wish/testsession"
    bm "github.com/charmbracelet/wish/bubbletea"
)

func TestMultiUser(t *testing.T) {
    hub := server.NewHub(db)
    s, err := wish.NewServer(
        wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool { return true }),
        wish.WithMiddleware(bm.Middleware(newTeaHandler(hub))),
    )
    if err != nil {
        t.Fatal(err)
    }

    // Each call connects a separate session to the same server
    sess1 := testsession.New(t, s, &gossh.ClientConfig{
        User:            "alice",
        Auth:            []gossh.AuthMethod{gossh.Password("test")},
        HostKeyCallback: gossh.InsecureIgnoreHostKey(),
    })

    sess2 := testsession.New(t, s, &gossh.ClientConfig{
        User:            "bob",
        Auth:            []gossh.AuthMethod{gossh.Password("test")},
        HostKeyCallback: gossh.InsecureIgnoreHostKey(),
    })

    _ = sess1
    _ = sess2
}
```

If you need the server address separately (e.g. to manage the lifecycle yourself):

```go
addr := testsession.Listen(t, s)                   // starts server, returns "host:port"
sess, err := testsession.NewClientSession(t, addr, cfg) // connect manually
```

## Manual Local Testing: Multiple SSH Key Pairs

Each SSH key has a unique fingerprint. Shellmate uses the fingerprint as the user identity, so generating a separate key per test user gives you independent sessions.

```bash
# Generate keys for two test users (no passphrase)
ssh-keygen -t ed25519 -f ~/.ssh/shellmate_user1 -N "" -C "shellmate-user1"
ssh-keygen -t ed25519 -f ~/.ssh/shellmate_user2 -N "" -C "shellmate-user2"
```

Run the server locally (default port 2222), then connect from separate terminals:

```bash
# Terminal 1
ssh -i ~/.ssh/shellmate_user1 -p 2222 localhost

# Terminal 2
ssh -i ~/.ssh/shellmate_user2 -p 2222 localhost
```

Each connection gets a different fingerprint → `GetUserByKeyFingerprint` returns a different user (or triggers the registration flow for new keys).

### Suppress host key warnings

Add this to `~/.ssh/config` to avoid `REMOTE HOST IDENTIFICATION HAS CHANGED` prompts during development:

```
Host localhost
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
```
