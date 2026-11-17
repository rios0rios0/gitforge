package infrastructure

import (
	"net"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/skeema/knownhosts"
	"golang.org/x/crypto/ssh"
)

// BuildKnownHostsDB exports buildKnownHostsDB for testing.
var BuildKnownHostsDB = buildKnownHostsDB //nolint:gochecknoglobals // test export

// DiscoverKnownHostsFiles exports discoverKnownHostsFiles for testing.
var DiscoverKnownHostsFiles = discoverKnownHostsFiles //nolint:gochecknoglobals // test export

// WindowsKnownHostsFilesUnder exports windowsKnownHostsFilesUnder for testing.
var WindowsKnownHostsFilesUnder = windowsKnownHostsFilesUnder //nolint:gochecknoglobals // test export

// ParseWSLAutomountRoot exports parseWSLAutomountRoot for testing.
var ParseWSLAutomountRoot = parseWSLAutomountRoot //nolint:gochecknoglobals // test export

// KernelIsWSL exports kernelIsWSL for testing.
var KernelIsWSL = kernelIsWSL //nolint:gochecknoglobals // test export

// SSHConfigSource exports sshConfigSource for testing. An alias rather than a definition, so
// a test can name the files a resolution reads directly instead of arranging a home for them.
type SSHConfigSource = sshConfigSource

// SSHHostSettings exports sshHostSettings for testing.
type SSHHostSettings = sshHostSettings

// ResolveSSHHost loads the given ssh_config files, as the SSH transport loads the ones it
// discovers, and resolves alias against them.
func ResolveSSHHost(sources []SSHConfigSource, automountRoot, alias, remoteUser string) SSHHostSettings {
	return loadSSHConfigFiles(sources, automountRoot).resolve(alias, remoteUser)
}

// SSHHostTarget exports sshHostSettings.target for testing.
func SSHHostTarget(settings SSHHostSettings, endpoint *transport.Endpoint) *transport.Endpoint {
	return settings.target(endpoint)
}

// LoadSSHConfigFiles loads the given ssh_config files and reports how many were usable, so a
// test can tell a skipped file from a loaded one.
func LoadSSHConfigFiles(sources []SSHConfigSource) int {
	return len(loadSSHConfigFiles(sources, "").files)
}

// DescribeSSHConfigSearch exports describeSSHConfigSearch for testing.
var DescribeSSHConfigSearch = describeSSHConfigSearch //nolint:gochecknoglobals // test export

// DiscoverSSHConfigSources exports discoverSSHConfigSources for testing.
var DiscoverSSHConfigSources = discoverSSHConfigSources //nolint:gochecknoglobals // test export

// WindowsSSHConfigSourcesUnder exports windowsSSHConfigSourcesUnder for testing.
var WindowsSSHConfigSourcesUnder = windowsSSHConfigSourcesUnder //nolint:gochecknoglobals // test export

// GitSSHCommandInEffect exports gitSSHCommandInEffect for testing.
var GitSSHCommandInEffect = gitSSHCommandInEffect //nolint:gochecknoglobals // test export

// RunsWindowsSSH exports runsWindowsSSH for testing.
var RunsWindowsSSH = runsWindowsSSH //nolint:gochecknoglobals // test export

// WindowsProfilesOnPath exports windowsProfilesOnPath for testing.
var WindowsProfilesOnPath = windowsProfilesOnPath //nolint:gochecknoglobals // test export

// CurrentWindowsProfileSources exports currentWindowsProfileSources for testing.
var CurrentWindowsProfileSources = currentWindowsProfileSources //nolint:gochecknoglobals // test export

// LoadSSHIdentity exports loadSSHIdentity for testing, flattening the unexported identity
// into its public key and whether the file yielded a key usable without the agent.
func LoadSSHIdentity(path string) (ssh.PublicKey, bool, error) {
	identity, err := loadSSHIdentity(path)
	return identity.publicKey, identity.signer != nil, err
}

// SelectSigners loads the identities the IdentityFile paths name and selects the keys to
// offer for them, as the SSH transport does for an alias.
func SelectSigners(agentSigners []ssh.Signer, identityFiles []string, identitiesOnly bool) []ssh.Signer {
	return selectSigners(agentSigners, loadSSHIdentities(identityFiles), identitiesOnly)
}

// NewSSHTransportWith builds the SSH transport over next with fixed ssh_config and
// known_hosts files in place of the ones the host offers.
func NewSSHTransportWith(
	next transport.Transport,
	sources []SSHConfigSource,
	automountRoot string,
	knownHostsFiles []string,
) transport.Transport {
	return NewSSHTransportWithOptions(next, SSHTransportOptions{}, sources, automountRoot, knownHostsFiles)
}

// NewSSHTransportWithOptions builds the transport NewSSHTransportWith builds, with the
// options a consumer adds through InstallSSHTransportWith.
func NewSSHTransportWithOptions(
	next transport.Transport,
	options SSHTransportOptions,
	sources []SSHConfigSource,
	automountRoot string,
	knownHostsFiles []string,
) transport.Transport {
	config := loadSSHConfigFiles(sources, automountRoot)
	hostKeys := buildKnownHostsDB(knownHostsFiles)
	return newSSHTransport(
		next,
		func() *sshConfigFiles { return config },
		func() *knownhosts.HostKeyDB { return hostKeys },
		options,
	)
}

// SSHTransportOptionsOf returns the options of an SSH transport gitforge built, and false
// when candidate is not one.
func SSHTransportOptionsOf(candidate transport.Transport) (SSHTransportOptions, bool) {
	built, isGitforge := candidate.(*sshTransport)
	if !isGitforge {
		return SSHTransportOptions{}, false
	}
	return built.options, true
}

// ListSSHAgentSigners exports listSSHAgentSigners for testing, taking the dialer as a plain
// function so a test can hand it connections that fail on cue.
func ListSSHAgentSigners(dial func() (net.Conn, error)) ([]ssh.Signer, error) {
	return listSSHAgentSigners(dial)
}

// DefaultSSHHostKeyCallback exports getDefaultSSHHostKeyCallback for testing.
var DefaultSSHHostKeyCallback = getDefaultSSHHostKeyCallback //nolint:gochecknoglobals // test export
