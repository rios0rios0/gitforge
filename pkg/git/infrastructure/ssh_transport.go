package infrastructure

import (
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/client"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	log "github.com/sirupsen/logrus"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

// sshTransport is go-git's SSH transport with the operator's SSH client configuration
// applied to every connection. go-git never runs the ssh binary, so nothing else applies
// it; and because the transport sits under every SSH connection go-git opens, it covers the
// pushes, clones and remote listings a consumer asks for as much as the SSH agent fallback
// PushChangesSSH opens on its own after the consumer's methods fail. For each connection it:
//
//   - dials the HostName and Port the remote's host alias names in ssh_config -- under WSL,
//     in the Windows profile's file too when git runs ssh.exe, since that is where such a
//     setup defines its aliases;
//   - offers keys the way OpenSSH would for that alias: the agent keys its IdentityFile
//     entries name first, with IdentitiesOnly no other agent key, and the unencrypted
//     IdentityFile keys the agent does not hold last;
//   - negotiates only the host key types known_hosts records for the host, so the key the
//     server presents is one the host key callback can verify.
type sshTransport struct {
	next     transport.Transport
	config   func() *sshConfigFiles
	hostKeys func() *knownhosts.HostKeyDB

	// options is what the consumer added to the transport through InstallSSHTransportWith.
	options SSHTransportOptions

	// warned holds the aliases a connection has already warned about offering no key, so
	// a run that connects several times says so once.
	warned sync.Map
}

// SSHTransportOptions is what a consumer adds to the SSH transport InstallSSHTransportWith
// installs: what only the consumer knows, such as settings of its own. The zero value adds
// nothing, and is what InstallSSHTransport installs the transport with.
type SSHTransportOptions struct {
	// MissingKeyRemedy is a way the consumer itself offers to provide a key, such as a
	// setting that names a key file. When IdentitiesOnly leaves a host alias no key to
	// offer, the transport warns once for that alias, and the warning ends "add that key to
	// the SSH agent". A non-empty MissingKeyRemedy is appended to it after ", or ", so it
	// should be an instruction about that key: "configure it as ssh_key_path", for example.
	// Empty leaves the warning as it is.
	MissingKeyRemedy string
}

// InstallSSHTransport makes every SSH connection go-git opens in this process honour the
// operator's SSH client configuration, the way the ssh binary does for git: a consumer
// calls it once, at startup, before any push, clone or listing. It
//
//   - installs NewSSHTransport(gitssh.DefaultClient) for the "ssh" scheme;
//   - switches off go-git's own ssh_config reader (gitssh.DefaultSSHConfig), which applies
//     only HostName and Port, since the transport has already replaced an alias with the
//     HostName it names, which must not be resolved a second time;
//   - hands SSHHostKeyCallback to SetDefaultSSHHostKeyCallback, so the default SSH agent
//     fallback verifies hosts against the same known_hosts discovery.
//
// The transport asks the server only for the host key types known_hosts records, and reads
// them from the files KnownHostsCallback verifies against: an auth method a consumer builds
// should carry KnownHostsCallback, as a callback verifying against other files may be asked
// for a key type it does not hold.
//
// A consumer with SSHTransportOptions of its own calls InstallSSHTransportWith instead.
func InstallSSHTransport() {
	InstallSSHTransportWith(SSHTransportOptions{})
}

// InstallSSHTransportWith does what InstallSSHTransport does, but installs the transport
// with the options a consumer adds to it (see SSHTransportOptions). With the zero value it
// is InstallSSHTransport.
func InstallSSHTransportWith(options SSHTransportOptions) {
	client.InstallProtocol(sshProtocol, newSSHTransport(gitssh.DefaultClient, sshConfig, knownHostsDB, options))
	gitssh.DefaultSSHConfig = nil //nolint:reassign // the SSH transport resolves ssh_config instead
	SetDefaultSSHHostKeyCallback(SSHHostKeyCallback())
}

// NewSSHTransport wraps go-git's SSH transport (gitssh.DefaultClient) so every connection
// it opens honours the operator's ssh_config and known_hosts; InstallSSHTransport installs
// the result for the "ssh" scheme, and InstallSSHTransportWith the same transport with a
// consumer's SSHTransportOptions. The files are discovered on the first connection, not
// here, so a run that never touches SSH neither pays for the discovery nor sees its warnings.
func NewSSHTransport(next transport.Transport) transport.Transport {
	return newSSHTransport(next, sshConfig, knownHostsDB, SSHTransportOptions{})
}

// newSSHTransport builds the transport from where it reads ssh_config and known_hosts,
// which is what a test replaces, and the options the consumer added to it.
func newSSHTransport(
	next transport.Transport,
	config func() *sshConfigFiles,
	hostKeys func() *knownhosts.HostKeyDB,
	options SSHTransportOptions,
) *sshTransport {
	return &sshTransport{next: next, config: config, hostKeys: hostKeys, options: options}
}

// NewUploadPackSession opens a fetch, clone or remote listing session to the endpoint, with
// the operator's SSH client configuration applied.
func (t *sshTransport) NewUploadPackSession(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (transport.UploadPackSession, error) {
	target, prepared := t.prepare(endpoint, auth)
	return t.next.NewUploadPackSession(target, prepared)
}

// NewReceivePackSession opens a push session to the endpoint, with the operator's SSH
// client configuration applied.
func (t *sshTransport) NewReceivePackSession(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (transport.ReceivePackSession, error) {
	target, prepared := t.prepare(endpoint, auth)
	return t.next.NewReceivePackSession(target, prepared)
}

// prepare resolves the endpoint's host alias and fits auth to the host it resolves to.
func (t *sshTransport) prepare(
	endpoint *transport.Endpoint,
	auth transport.AuthMethod,
) (*transport.Endpoint, transport.AuthMethod) {
	settings := t.config().resolve(endpoint.Host, endpoint.User)
	target := settings.target(endpoint)
	if target != endpoint {
		log.Debugf("Resolved SSH host %q to %s through ssh_config", endpoint.Host, sshHostWithPort(target))
	}

	auth = t.withIdentities(auth, endpoint.Host, settings)
	return target, withHostKeyAlgorithms(auth, t.hostKeys(), sshHostWithPort(target))
}

// withIdentities makes an SSH agent auth method offer keys the way OpenSSH would for the
// alias (see selectSigners): the agent keys its IdentityFile entries name, the agent's
// others unless IdentitiesOnly, and the unencrypted IdentityFile keys the agent does not
// hold. A key file the consumer configured (a *gitssh.PublicKeys) is offered as configured,
// and an alias with
// no IdentityFile leaves the agent's keys in the agent's order.
func (t *sshTransport) withIdentities(
	auth transport.AuthMethod,
	alias string,
	settings sshHostSettings,
) transport.AuthMethod {
	agentAuth, isAgentAuth := auth.(*gitssh.PublicKeysCallback)
	if !isAgentAuth || agentAuth == nil || agentAuth.Callback == nil || len(settings.IdentityFiles) == 0 {
		return auth
	}

	identities := loadSSHIdentities(settings.IdentityFiles)
	listSigners := agentAuth.Callback
	narrowed := *agentAuth
	narrowed.Callback = func() ([]ssh.Signer, error) {
		signers, err := listSigners()
		if err != nil {
			return nil, err
		}

		selected := selectSigners(signers, identities, settings.IdentitiesOnly)
		if len(selected) == 0 && len(signers) > 0 {
			t.warnNoKeyOffered(alias, settings)
		}
		return selected, nil
	}
	return &narrowed
}

// warnNoKeyOffered explains, once per alias, a connection that offers no key at all:
// IdentitiesOnly narrowed the keys to the ones IdentityFile names, the agent holds none of
// them, and none is a key file usable without a passphrase. The server's refusal
// that follows says nothing about why. The warning asks for the key in the SSH agent, or
// through the consumer's MissingKeyRemedy when it set one.
func (t *sshTransport) warnNoKeyOffered(alias string, settings sshHostSettings) {
	if _, alreadyWarned := t.warned.LoadOrStore(alias, struct{}{}); alreadyWarned {
		return
	}

	remedy := "add that key to the SSH agent"
	if t.options.MissingKeyRemedy != "" {
		remedy += ", or " + t.options.MissingKeyRemedy
	}
	log.Warnf(
		"The ssh_config entry for %q sets IdentitiesOnly, and none of the keys its IdentityFile names "+
			"(%s) is held by the SSH agent or readable without a passphrase, so no key is offered: %s",
		alias, strings.Join(settings.IdentityFiles, ", "), remedy,
	)
}

// withHostKeyAlgorithms returns a copy of an SSH auth method that negotiates only the host
// key types known_hosts records for hostWithPort, when the method verifies host keys with a
// callback it was given.
//
// A server presents one host key, of the type the handshake settles on, and x/crypto
// settles on the first type in its own preference order that the server offers -- ECDSA
// ahead of Ed25519. known_hosts commonly records only the type OpenSSH settled on, which is
// Ed25519 for GitHub, so the key presented is not the one recorded and the callback fails
// with "knownhosts: key mismatch" against a host it actually trusts. go-git asks for the
// recorded types only when it reads known_hosts itself. The types come from knownHostsDB,
// which every callback gitforge builds verifies against; InstallSSHTransport asks consumers
// to give their own auth methods KnownHostsCallback for the same reason.
func withHostKeyAlgorithms(
	auth transport.AuthMethod,
	hostKeys *knownhosts.HostKeyDB,
	hostWithPort string,
) transport.AuthMethod {
	if hostKeys == nil {
		return auth
	}

	switch method := auth.(type) {
	case *gitssh.PublicKeysCallback:
		if method != nil && needsHostKeyAlgorithms(method.HostKeyCallbackHelper) {
			pinned := *method
			pinned.HostKeyAlgorithms = hostKeys.HostKeyAlgorithms(hostWithPort)
			return &pinned
		}
	case *gitssh.PublicKeys:
		if method != nil && needsHostKeyAlgorithms(method.HostKeyCallbackHelper) {
			pinned := *method
			pinned.HostKeyAlgorithms = hostKeys.HostKeyAlgorithms(hostWithPort)
			return &pinned
		}
	}
	return auth
}

// needsHostKeyAlgorithms reports whether an auth method verifies host keys with a callback
// it was given and has chosen no host key types. A method without a callback is left alone:
// go-git gives it a default callback reading its own known_hosts, which the types read from
// gitforge's discovery would not match.
func needsHostKeyAlgorithms(helper gitssh.HostKeyCallbackHelper) bool {
	return helper.HostKeyCallback != nil && len(helper.HostKeyAlgorithms) == 0
}
