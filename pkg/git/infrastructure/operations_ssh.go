package infrastructure

import (
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// sshProtocol is the endpoint protocol string go-git's transport parser assigns to SSH
// remotes, both scp-like (git@host:path) and ssh:// ones.
const sshProtocol = "ssh"

// defaultSSHHostKeyCallback is the host key callback installed by a consumer for the
// default SSH agent fallback. Guarded by a mutex because SetDefaultSSHHostKeyCallback
// can be called from the consumer's wiring while pushes run elsewhere; in practice it
// is set once at startup and only read afterwards.
var (
	defaultSSHHostKeyCallbackMu sync.RWMutex        //nolint:gochecknoglobals // process-wide hook guard, set once at startup
	defaultSSHHostKeyCallback   ssh.HostKeyCallback //nolint:gochecknoglobals // process-wide hook, set once at startup
)

// SetDefaultSSHHostKeyCallback installs the host key callback used by the default SSH
// agent fallback in PushChangesSSH and remote reference listing.
//
// Without it the fallback pushes with a nil auth method, and go-git then builds its own
// host key verification from SSH_KNOWN_HOSTS or ~/.ssh/known_hosts alone. On a host that
// keeps its known_hosts somewhere else — a WSL machine whose file belongs to the Windows
// profile is the common case — every fallback fails with "unable to find any valid
// known_hosts file" even though the consumer already discovered the file and verified
// against it for the explicit auth methods. A consumer that does its own discovery hands
// the resulting callback here so the fallback verifies hosts against the same data.
//
// Passing nil restores go-git's own default behaviour.
func SetDefaultSSHHostKeyCallback(callback ssh.HostKeyCallback) {
	defaultSSHHostKeyCallbackMu.Lock()
	defer defaultSSHHostKeyCallbackMu.Unlock()

	defaultSSHHostKeyCallback = callback
}

// getDefaultSSHHostKeyCallback returns the installed fallback host key callback, or nil
// when none is set.
func getDefaultSSHHostKeyCallback() ssh.HostKeyCallback {
	defaultSSHHostKeyCallbackMu.RLock()
	defer defaultSSHHostKeyCallbackMu.RUnlock()

	return defaultSSHHostKeyCallback
}

// defaultSSHAuth builds the fallback auth method for a remote URL: the default SSH agent
// go-git would have consulted anyway (SSH_AUTH_SOCK), carrying the installed host key
// callback. It returns nil — leaving the caller to pass a nil auth method, exactly the
// previous behaviour — when no callback is installed, the remote is not an SSH one, the
// URL does not parse, or no agent is reachable.
func defaultSSHAuth(remoteURL string) transport.AuthMethod {
	callback := getDefaultSSHHostKeyCallback()
	if callback == nil {
		return nil
	}

	endpoint, err := transport.NewEndpoint(remoteURL)
	if err != nil {
		log.Debugf("Cannot parse remote URL for the default SSH auth: %v", err)
		return nil
	}
	if endpoint.Protocol != sshProtocol {
		return nil
	}

	auth, err := gitssh.NewSSHAgentAuth(endpoint.User)
	if err != nil {
		log.Debugf("Cannot build the default SSH agent auth: %v", err)
		return nil
	}
	auth.HostKeyCallback = callback

	return auth
}

// defaultSSHAuthForRemote builds the fallback auth method from a remote's first URL.
func defaultSSHAuthForRemote(remote *git.Remote) transport.AuthMethod {
	if remote == nil {
		return nil
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return nil
	}

	return defaultSSHAuth(urls[0])
}

// defaultSSHAuthForRepo builds the fallback auth method from the repository's origin
// remote.
func defaultSSHAuthForRepo(repo *git.Repository) transport.AuthMethod {
	if repo == nil {
		return nil
	}

	remote, err := repo.Remote(originRemoteName)
	if err != nil {
		return nil
	}

	return defaultSSHAuthForRemote(remote)
}
