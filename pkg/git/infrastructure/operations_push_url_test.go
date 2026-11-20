package infrastructure_test

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5"
	gitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	gitops "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	"github.com/rios0rios0/gitforge/v4/test/builders"
	"github.com/rios0rios0/gitforge/v4/test/doubles"
)

// The tests in this file give origin a pushurl, or a second url, so the URL a push dials --
// the last of origin's URLs, which is where go-git pushes -- differs from origin's first URL.
// An http:// push URL stands in for an https:// one where a test needs the request to arrive:
// both take the same path, and go-git cannot be handed a certificate for a loopback TLS
// server through PushWithTransportDetection. The refusals need no handshake, so they are
// checked over TLS too.

func TestPushWithTransportDetectionDialsThePushURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// origin returns origin's url and pushurl entries for the bare repository at bare.
		origin func(bare string) (urls, pushURLs []string)
	}{
		{
			name: "should push to a local pushurl rather than the HTTPS url beside it",
			origin: func(bare string) ([]string, []string) {
				return []string{"https://github.com/org/repo.git"}, []string{bare}
			},
		},
		{
			name: "should push to a file:// pushurl rather than the HTTPS url beside it",
			origin: func(bare string) ([]string, []string) {
				return []string{"https://github.com/org/repo.git"}, []string{"file://" + bare}
			},
		},
		{
			name: "should push to the last of two url entries",
			origin: func(bare string) ([]string, []string) {
				return []string{"https://github.com/org/repo.git", bare}, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given
			bare := initBareRepository(t)
			urls, pushURLs := test.origin(bare)
			repo := newRepoWithOriginURLs(t, urls, pushURLs)

			// when
			err := gitops.PushWithTransportDetection(repo, headRefSpec(t, repo), nil)

			// then
			require.NoError(t, err)
			assertHeadPushedTo(t, repo, bare)
		})
	}
}

func TestPushWithTransportDetectionSendsCredentialsToTheirHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// origin returns origin's url and pushurl entries for the forge served at serverURL.
		origin   func(serverURL string) (urls, pushURLs []string)
		wantPath string
	}{
		{
			name: "should send the credentials to origin's only URL",
			origin: func(serverURL string) ([]string, []string) {
				return []string{serverURL + "/org/repo.git"}, nil
			},
			wantPath: "/org/repo.git/info/refs",
		},
		{
			name: "should send the credentials to a pushurl on the host of origin's first URL",
			origin: func(serverURL string) ([]string, []string) {
				return []string{serverURL + "/org/fetched.git"}, []string{serverURL + "/org/pushed.git"}
			},
			wantPath: "/org/pushed.git/info/refs",
		},
		{
			name: "should push over HTTP to an HTTP pushurl beside an SSH url on the same host",
			origin: func(serverURL string) ([]string, []string) {
				// The url is a closed port: nothing may be dialed there.
				return []string{"ssh://git@127.0.0.1:1/org/repo.git"}, []string{serverURL + "/org/repo.git"}
			},
			wantPath: "/org/repo.git/info/refs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given
			forge := startCredentialRecorder(t, false)
			urls, pushURLs := test.origin(forge.url())
			repo := newRepoWithOriginURLs(t, urls, pushURLs)
			token := newTokenAuth()

			// when
			err := gitops.PushWithTransportDetection(repo, headRefSpec(t, repo), []transport.AuthMethod{token})

			// then -- the forge answers 401, so every request it got is one HTTPS attempt
			require.ErrorIs(t, err, transport.ErrAuthenticationRequired)
			assert.Contains(t, err.Error(), "all push attempts failed")
			requests := forge.recordedRequests()
			require.NotEmpty(t, requests, "the push never reached the push URL")
			for _, request := range requests {
				assert.Equal(t, test.wantPath, request.path)
				assert.Equal(t, token.Password, request.password, "a request arrived without the credentials")
			}
		})
	}
}

func TestPushWithTransportDetectionRefusesCredentialsForAnotherHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		overTLS bool
		// origin returns origin's url and pushurl entries for the server served at serverURL.
		origin func(serverURL string) (urls, pushURLs []string)
	}{
		{
			name: "should refuse to send the credentials to an HTTP pushurl on another host",
			origin: func(serverURL string) ([]string, []string) {
				return []string{"https://github.com/org/repo.git"}, []string{serverURL + "/org/repo.git"}
			},
		},
		{
			name:    "should refuse to send the credentials to an HTTPS pushurl on another host",
			overTLS: true,
			origin: func(serverURL string) ([]string, []string) {
				return []string{"https://github.com/org/repo.git"}, []string{serverURL + "/org/repo.git"}
			},
		},
		{
			name: "should refuse to send the credentials to the second of two url entries on another host",
			origin: func(serverURL string) ([]string, []string) {
				return []string{"https://github.com/org/repo.git", serverURL + "/org/repo.git"}, nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// given
			attacker := startCredentialRecorder(t, test.overTLS)
			urls, pushURLs := test.origin(attacker.url())
			repo := newRepoWithOriginURLs(t, urls, pushURLs)

			// when
			err := gitops.PushWithTransportDetection(
				repo, headRefSpec(t, repo), []transport.AuthMethod{newTokenAuth()},
			)

			// then
			require.Error(t, err)
			assert.Contains(t, err.Error(), `"127.0.0.1"`, "the error should name the push URL's host")
			assert.Contains(t, err.Error(), `"github.com"`, "the error should name the first URL's host")
			assert.Zero(t, attacker.connectionCount(), "the push dialed the other host")
			assert.Empty(t, attacker.recordedRequests())
		})
	}
}

func TestPushWithTransportDetectionClassifiesThePushURL(t *testing.T) {
	t.Parallel()

	t.Run("should push over SSH to an SSH pushurl beside an HTTPS url", func(t *testing.T) {
		t.Parallel()

		// given -- the pushurl is a closed port, so the SSH key's attempt fails at the dial
		repo := newRepoWithOriginURLs(t,
			[]string{"https://github.com/org/repo.git"},
			[]string{"ssh://git@127.0.0.1:1/org/repo.git"},
		)
		authMethods := []transport.AuthMethod{newTokenAuth(), newUntrustingSSHKeyAuth(t)}

		// when
		err := gitops.PushWithTransportDetection(repo, headRefSpec(t, repo), authMethods)

		// then -- the SSH path, whose fallback names the explicit attempts' failure
		require.Error(t, err)
		assert.Contains(t, err.Error(), "explicit SSH auth also failed")
		assert.Contains(t, err.Error(), "127.0.0.1:1", "the SSH key should have dialed the pushurl")
		assert.NotContains(t, err.Error(), "all push attempts failed")
	})

	t.Run("should not fall back to an anonymous dial of an HTTP pushurl beside an SSH url", func(t *testing.T) {
		t.Parallel()

		// given
		forge := startCredentialRecorder(t, false)
		repo := newRepoWithOriginURLs(t,
			[]string{"git@127.0.0.1:org/repo.git"},
			[]string{forge.url() + "/org/repo.git"},
		)

		// when
		err := gitops.PushWithTransportDetection(
			repo, headRefSpec(t, repo), []transport.AuthMethod{newUntrustingSSHKeyAuth(t)},
		)

		// then -- an SSH key cannot authenticate an HTTP push, and no SSH agent fallback runs
		require.ErrorIs(t, err, transport.ErrInvalidAuthMethod)
		assert.Contains(t, err.Error(), "all push attempts failed")
		assert.Zero(t, forge.connectionCount())
	})
}

// TestPushToSSHPushURLThroughTheAgentFallback is deliberately not parallel: it installs the
// process-wide host key callback of the default SSH agent fallback and sets SSH_AUTH_SOCK.
func TestPushToSSHPushURLThroughTheAgentFallback(t *testing.T) {
	t.Cleanup(func() { gitops.SetDefaultSSHHostKeyCallback(nil) })

	tests := []struct {
		name string
		push func(repo *git.Repository, refSpec gitcfg.RefSpec) error
	}{
		{
			name: "should push to an SSH pushurl beside an HTTPS url with the forge token given",
			push: func(repo *git.Repository, refSpec gitcfg.RefSpec) error {
				return gitops.PushWithTransportDetection(repo, refSpec, []transport.AuthMethod{newTokenAuth()})
			},
		},
		{
			name: "should build the agent fallback of PushChangesSSH for the pushurl",
			push: func(repo *git.Repository, refSpec gitcfg.RefSpec) error {
				return gitops.PushChangesSSH(repo, refSpec, nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// given -- a forge with one host key, recorded in known_hosts, and an agent holding
			// the one key it accepts
			hostKey := newTestSigner(t, newEd25519Key(t))
			userKey := newEd25519Key(t)
			forge := startGitSSHServer(t, newTestSigner(t, userKey).PublicKey(), hostKey)
			knownHosts := writeSSHFixture(
				t, filepath.Join(t.TempDir(), "known_hosts"), forge.knownHostsLine(hostKey.PublicKey()),
			)
			gitops.SetDefaultSSHHostKeyCallback(gitops.BuildKnownHostsDB([]string{knownHosts}).HostKeyCallback())
			t.Setenv("SSH_AUTH_SOCK", serveSSHAgent(t, userKey))
			repo := newRepoWithOriginURLs(t,
				[]string{"https://github.com/org/repo.git"},
				[]string{"ssh://git@127.0.0.1:" + forge.port() + "/org/repo.git"},
			)

			// when
			err := test.push(repo, headRefSpec(t, repo))

			// then
			require.NoError(t, err)
			assert.Equal(t,
				[]string{ssh.FingerprintSHA256(newTestSigner(t, userKey).PublicKey())},
				forge.offeredKeys(),
			)
		})
	}
}

func TestPushChangesHTTPSPushURL(t *testing.T) {
	t.Parallel()

	newOperations := func() *gitops.GitOperations {
		adapter := builders.NewForgeProviderStubBuilder().
			WithServiceType(globalEntities.GITHUB).
			WithAuthMethods([]transport.AuthMethod{newTokenAuth()}).
			Build().(*doubles.ForgeProviderStub)
		return gitops.NewGitOperations(builders.NewAdapterFinderStubBuilder().
			WithAdapterByURL(adapter).
			WithAdapterByServiceType(adapter).
			Build().(*doubles.AdapterFinderStub))
	}

	t.Run("should refuse to send the forge token to a pushurl on another host", func(t *testing.T) {
		t.Parallel()

		// given
		attacker := startCredentialRecorder(t, false)
		repo := newRepoWithOriginURLs(t,
			[]string{"https://github.com/org/repo.git"},
			[]string{attacker.url() + "/org/repo.git"},
		)

		// when
		err := newOperations().PushChangesHTTPS(repo, "user", headRefSpec(t, repo))

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"127.0.0.1"`)
		assert.Contains(t, err.Error(), `"github.com"`)
		assert.Zero(t, attacker.connectionCount(), "the push dialed the other host")
	})

	t.Run("should send the forge token to a pushurl on the host of origin's first URL", func(t *testing.T) {
		t.Parallel()

		// given
		forge := startCredentialRecorder(t, false)
		repo := newRepoWithOriginURLs(t,
			[]string{forge.url() + "/org/fetched.git"},
			[]string{forge.url() + "/org/pushed.git"},
		)

		// when
		err := newOperations().PushChangesHTTPS(repo, "user", headRefSpec(t, repo))

		// then
		require.ErrorIs(t, err, transport.ErrAuthenticationRequired)
		requests := forge.recordedRequests()
		require.NotEmpty(t, requests, "the push never reached the push URL")
		for _, request := range requests {
			assert.Equal(t, "/org/pushed.git/info/refs", request.path)
			assert.Equal(t, newTokenAuth().Password, request.password)
		}
	})
}

// newRepoWithOriginURLs builds a repository on disk with one commit whose origin remote has
// the given url entries, then the given pushurl entries. They are written to .git/config the
// way git writes them: go-git's RemoteConfig has no field for a pushurl, and reading one from
// the file is what appends it to RemoteConfig.URLs.
func newRepoWithOriginURLs(t *testing.T, urls, pushURLs []string) *git.Repository {
	t.Helper()

	path := t.TempDir()
	createFilesystemRepoWithCommit(t, path)

	lines := []string{`[remote "origin"]`}
	for _, remoteURL := range urls {
		lines = append(lines, "\turl = "+remoteURL)
	}
	for _, pushURL := range pushURLs {
		lines = append(lines, "\tpushurl = "+pushURL)
	}
	lines = append(lines, "\tfetch = +refs/heads/*:refs/remotes/origin/*", "")

	configPath := filepath.Join(path, ".git", "config")
	existing, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, append(existing, strings.Join(lines, "\n")...), 0o600))

	repo, err := git.PlainOpen(path)
	require.NoError(t, err)
	remote, err := repo.Remote("origin")
	require.NoError(t, err)
	require.Equal(t, slices.Concat(urls, pushURLs), remote.Config().URLs,
		"go-git should read every pushurl after the url entries")
	return repo
}

// headRefSpec returns the refspec that pushes the branch HEAD is on to the same branch.
func headRefSpec(t *testing.T, repo *git.Repository) gitcfg.RefSpec {
	t.Helper()

	head, err := repo.Head()
	require.NoError(t, err)
	return gitcfg.RefSpec(head.Name().String() + ":" + head.Name().String())
}

// initBareRepository creates an empty bare repository and returns its path.
func initBareRepository(t *testing.T) string {
	t.Helper()

	path := t.TempDir()
	_, err := git.PlainInit(path, true)
	require.NoError(t, err)
	return path
}

// assertHeadPushedTo asserts that the bare repository at barePath holds the branch HEAD of
// repo is on, at HEAD's commit.
func assertHeadPushedTo(t *testing.T, repo *git.Repository, barePath string) {
	t.Helper()

	head, err := repo.Head()
	require.NoError(t, err)
	bare, err := git.PlainOpen(barePath)
	require.NoError(t, err)
	pushed, err := bare.Reference(head.Name(), false)
	require.NoError(t, err, "the push did not reach %s", barePath)
	assert.Equal(t, head.Hash(), pushed.Hash())
}

// newTokenAuth returns the HTTP Basic auth method the providers build from a forge token.
func newTokenAuth() *githttp.BasicAuth {
	return &githttp.BasicAuth{Username: "x-access-token", Password: "fixture-token-placeholder"}
}

// newUntrustingSSHKeyAuth returns an SSH key auth method whose host key callback trusts no
// host. The tests that use it never reach a host key: they dial a closed port, or a URL an
// SSH auth method cannot authenticate.
func newUntrustingSSHKeyAuth(t *testing.T) *gitssh.PublicKeys {
	t.Helper()

	return &gitssh.PublicKeys{
		User:   "git",
		Signer: newTestSigner(t, newEd25519Key(t)),
		HostKeyCallback: func(string, net.Addr, ssh.PublicKey) error {
			return errors.New("no host is trusted in this test")
		},
	}
}

// credentialRecorder is a loopback HTTP server standing in for a forge's smart HTTP endpoint.
// It answers every request 401 Unauthorized after recording it, and counts the connections
// opened to it, so a test can tell whether a push dialed it at all and which credentials
// reached it.
type credentialRecorder struct {
	server *httptest.Server

	mu          sync.Mutex
	connections int
	requests    []recordedRequest
}

// recordedRequest is a request a credentialRecorder answered.
type recordedRequest struct {
	path string
	// password is the HTTP Basic password the request carried, empty when it carried none.
	password string
}

// startCredentialRecorder starts a credentialRecorder for the lifetime of the test, over TLS
// when overTLS is set.
func startCredentialRecorder(t *testing.T, overTLS bool) *credentialRecorder {
	t.Helper()

	recorder := &credentialRecorder{}
	recorder.server = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, password, _ := r.BasicAuth()
		recorder.record(recordedRequest{path: r.URL.Path, password: password})
		w.WriteHeader(http.StatusUnauthorized)
	}))
	recorder.server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			recorder.countConnection()
		}
	}
	if overTLS {
		recorder.server.StartTLS()
	} else {
		recorder.server.Start()
	}
	t.Cleanup(recorder.server.Close)
	return recorder
}

// url returns the server's base URL, http://127.0.0.1:<port> or https://127.0.0.1:<port>.
func (r *credentialRecorder) url() string {
	return r.server.URL
}

// record records a request.
func (r *credentialRecorder) record(request recordedRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.requests = append(r.requests, request)
}

// countConnection counts a connection opened to the server.
func (r *credentialRecorder) countConnection() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.connections++
}

// recordedRequests returns the requests the server answered, in order.
func (r *credentialRecorder) recordedRequests() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.requests)
}

// connectionCount returns how many connections were opened to the server.
func (r *credentialRecorder) connectionCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.connections
}
