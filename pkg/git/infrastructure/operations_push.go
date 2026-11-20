package infrastructure

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	log "github.com/sirupsen/logrus"
)

// pushTarget is where a push to origin goes.
type pushTarget struct {
	// url is the URL the push dials: the last of origin's URLs. go-git appends every pushurl
	// to the URLs it read from url and, unless PushOptions.RemoteURL names another, pushes to
	// the last of them, so a pushurl wins over url as it does in git (git pushes to every
	// pushurl, go-git to the last one only). Every push here passes it as
	// PushOptions.RemoteURL, so the transport chosen for it and the URL dialed cannot differ.
	url string
	// firstURL is origin's first URL: the one fetches use, and the one consumers build their
	// token auth methods for (GetRemoteServiceType, GetRemoteRepoURL).
	firstURL string
}

// OriginPushURL returns the URL a push to origin dials: the last of origin's URLs, which is
// its pushurl when one is set, since go-git appends every pushurl to the URLs it reads from
// url and pushes to the last of them, while fetches and remote listings use the first. Every
// push here (PushWithTransportDetection, PushChangesSSH, PushChangesHTTPS, DeleteRemoteBranch)
// reads its URL through the same originPushTarget, so this is the URL it dials. It fails when
// the repository has no origin remote, or origin has no URL.
//
// Exported for use by autobump (github.com/rios0rios0/autobump), whose SSH diagnostics explain
// a failed push by the URL that push dialed rather than by a copy of this rule.
func OriginPushURL(repo *git.Repository) (string, error) {
	target, err := originPushTarget(repo)
	if err != nil {
		return "", err
	}
	return target.url, nil
}

// originPushTarget reads where a push to origin goes. It is the one place the push URL is
// chosen: every push and OriginPushURL read it here.
func originPushTarget(repo *git.Repository) (pushTarget, error) {
	remote, err := repo.Remote(originRemoteName)
	if err != nil {
		return pushTarget{}, fmt.Errorf("failed to get origin remote: %w", err)
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return pushTarget{}, errors.New("origin remote has no URLs configured")
	}
	return pushTarget{url: urls[len(urls)-1], firstURL: urls[0]}, nil
}

// refuseCredentialsForAnotherHost fails a push that would carry HTTP credentials to a host
// other than the host of origin's first URL. Consumers build their token auth methods for the
// forge that first URL names, while the push dials origin's push URL, which a pushurl or a
// second url can point anywhere: unchecked, go-git sends the forge's token, as HTTP Basic auth,
// to whatever host that is. Every auth method that puts credentials on an HTTP request counts,
// and nothing else can authenticate a push over HTTP, so with them dropped the push could not
// authenticate at all: it fails here, naming both hosts, before anything is dialed.
//
// Hosts are compared by name, case-insensitively, without the scheme or the port: an SSH url
// beside an HTTPS pushurl names one forge, while another name may be another party. A push
// URL whose host cannot be read is never the first URL's host.
func refuseCredentialsForAnotherHost(target pushTarget, authMethods []transport.AuthMethod) error {
	if target.url == target.firstURL || !slices.ContainsFunc(authMethods, carriesHTTPCredentials) {
		return nil
	}

	pushHost, credentialHost := remoteHost(target.url), remoteHost(target.firstURL)
	if pushHost != "" && strings.EqualFold(pushHost, credentialHost) {
		return nil
	}
	return fmt.Errorf(
		"refusing to send HTTP credentials to %q, the host of origin's push URL: "+
			"they are for %q, the host of origin's first URL",
		pushHost, credentialHost,
	)
}

// carriesHTTPCredentials reports whether an auth method puts credentials on an HTTP request,
// as the token auth methods the providers build (HTTP Basic) do.
func carriesHTTPCredentials(method transport.AuthMethod) bool {
	_, isHTTP := method.(githttp.AuthMethod)
	return isHTTP
}

// remoteHost returns the host name a remote URL names, in any form go-git reads (scp-like,
// ssh://, http(s)://), or "" for a local path or a URL that does not parse.
func remoteHost(remoteURL string) string {
	endpoint, err := transport.NewEndpoint(remoteURL)
	if err != nil {
		return ""
	}
	return endpoint.Host
}

// PushChangesSSH pushes the changes to the remote repository over SSH, to origin's push URL.
// If authMethods is non-empty, each method is tried in order until one succeeds.
// If all explicit methods fail or authMethods is empty, it falls back to
// go-git's default SSH agent behaviour (SSH_AUTH_SOCK), carrying the host key callback
// installed via SetDefaultSSHHostKeyCallback so the fallback verifies hosts against the
// consumer's own known_hosts discovery rather than go-git's.
//
// When both an explicit method and the fallback fail, the returned error names the
// explicit failure too: the fallback's own error (often "unable to find any valid
// known_hosts file", or an agent that answered nothing) otherwise masks the credential
// problem the operator actually has to fix.
func PushChangesSSH(repo *git.Repository, refSpec config.RefSpec, authMethods []transport.AuthMethod) error {
	// Without an origin URL there is nothing to pin, and the push fails with go-git's own
	// error for the missing remote, as it did before the push URL was pinned.
	pushURL := ""
	if target, err := originPushTarget(repo); err == nil {
		pushURL = target.url
	}
	return pushChangesSSH(repo, pushURL, refSpec, authMethods)
}

// pushChangesSSH is PushChangesSSH for the push URL already read.
func pushChangesSSH(
	repo *git.Repository,
	pushURL string,
	refSpec config.RefSpec,
	authMethods []transport.AuthMethod,
) error {
	log.Info("Pushing local changes to remote repository through SSH")

	var lastErr error
	for _, method := range authMethods {
		lastErr = repo.Push(&git.PushOptions{
			RefSpecs:  []config.RefSpec{refSpec},
			RemoteURL: pushURL,
			Auth:      method,
		})
		if lastErr == nil {
			return nil
		}
		log.Debugf("SSH push attempt failed with auth method %T: %v", method, lastErr)
	}
	if len(authMethods) > 0 {
		log.Debug("All explicit SSH auth methods failed, falling back to default SSH agent")
	}

	err := repo.Push(&git.PushOptions{
		RefSpecs:  []config.RefSpec{refSpec},
		RemoteURL: pushURL,
		Auth:      defaultSSHAuth(pushURL),
	})
	if err != nil {
		if lastErr != nil {
			return fmt.Errorf(
				"could not push changes to remote repository: %w (explicit SSH auth also failed: %w)",
				err, lastErr,
			)
		}
		return fmt.Errorf("could not push changes to remote repository: %w", err)
	}
	return nil
}

// PushChangesHTTPS pushes the changes to the remote repository over HTTPS, to origin's push URL.
// It tries each authentication method returned by the adapter until one succeeds, and refuses
// to send them to a push URL whose host is not the host of origin's first URL.
func (o *GitOperations) PushChangesHTTPS(
	repo *git.Repository,
	username string,
	refSpec config.RefSpec,
) error {
	log.Info("Pushing local changes to remote repository through HTTPS")

	service, err := o.GetRemoteServiceType(repo)
	if err != nil {
		return err
	}
	authMethods, err := o.GetAuthMethods(service, username)
	if err != nil {
		return err
	}
	target, err := originPushTarget(repo)
	if err != nil {
		return err
	}
	if err = refuseCredentialsForAnotherHost(target, authMethods); err != nil {
		return err
	}

	pushOptions := &git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: originRemoteName,
		RemoteURL:  target.url,
	}
	var lastErr error
	for _, auth := range authMethods {
		pushOptions.Auth = auth
		pushErr := repo.Push(pushOptions)
		if pushErr == nil {
			return nil
		}
		lastErr = pushErr
	}
	if lastErr != nil {
		return fmt.Errorf("could not push changes to remote repository: %w", lastErr)
	}
	return errors.New("could not push changes to remote repository: no authentication methods attempted")
}

// PushWithTransportDetection pushes the given refSpec to the origin remote,
// auto-detecting the transport (SSH or HTTPS) from the URL the push dials:
// origin's push URL, the last of its URLs, which is its pushurl when one is set.
// That URL is passed to go-git explicitly, so the transport detected is always
// the transport of the URL dialed.
//
// For SSH remotes (git@ or ssh://), authMethods are tried in order; if all
// fail or none are provided, it falls back to go-git's default SSH agent.
//
// For HTTPS/HTTP remotes, authMethods are tried in order until one succeeds.
// An empty authMethods slice returns an error for HTTPS remotes, and so do HTTP
// credentials (the providers' token auth methods) for a push URL whose host is
// not the host of origin's first URL, the one consumers build them for.
//
// Exported for use by autobump (github.com/rios0rios0/autobump) and
// autoupdate (github.com/rios0rios0/autoupdate).
func PushWithTransportDetection(
	repo *git.Repository,
	refSpec config.RefSpec,
	authMethods []transport.AuthMethod,
) error {
	target, err := originPushTarget(repo)
	if err != nil {
		return err
	}

	switch {
	case strings.HasPrefix(target.url, "git@") || strings.HasPrefix(target.url, "ssh://"):
		log.Info("Pushing to remote via SSH")
		return pushChangesSSH(repo, target.url, refSpec, authMethods)

	case strings.HasPrefix(target.url, "https://"):
		log.Info("Pushing to remote via HTTPS")
		return pushWithAuthRetry(repo, target, refSpec, authMethods)

	case strings.HasPrefix(target.url, "http://"):
		log.Warn("Pushing over plaintext HTTP — credentials may be exposed; consider switching to HTTPS")
		return pushWithAuthRetry(repo, target, refSpec, authMethods)

	case strings.HasPrefix(target.url, "file://") || filepath.IsAbs(target.url):
		log.Info("Pushing to a remote on the local filesystem")
		return pushLocal(repo, target.url, refSpec)

	default:
		return fmt.Errorf("unsupported remote URL scheme: %s", target.url)
	}
}

// pushLocal pushes to a remote living on the local filesystem, where there is no
// transport to authenticate against. Keeping this case explicit (rather than letting
// it fall through to the default) preserves the unsupported-scheme error for genuinely
// malformed remote URLs.
func pushLocal(repo *git.Repository, pushURL string, refSpec config.RefSpec) error {
	err := repo.Push(&git.PushOptions{
		RefSpecs:   []config.RefSpec{refSpec},
		RemoteName: originRemoteName,
		RemoteURL:  pushURL,
	})
	if err != nil {
		return fmt.Errorf("could not push changes to the local remote repository: %w", err)
	}
	return nil
}

// pushWithAuthRetry tries each auth method in sequence until one succeeds.
func pushWithAuthRetry(
	repo *git.Repository,
	target pushTarget,
	refSpec config.RefSpec,
	authMethods []transport.AuthMethod,
) error {
	if len(authMethods) == 0 {
		return errors.New("no authentication methods provided for HTTPS push")
	}
	if err := refuseCredentialsForAnotherHost(target, authMethods); err != nil {
		return err
	}

	var lastErr error
	for _, method := range authMethods {
		lastErr = repo.Push(&git.PushOptions{
			RefSpecs:   []config.RefSpec{refSpec},
			RemoteName: originRemoteName,
			RemoteURL:  target.url,
			Auth:       method,
		})
		if lastErr == nil {
			return nil
		}
		log.Debugf("Push attempt failed with auth method %T: %v", method, lastErr)
	}

	return fmt.Errorf("all push attempts failed, last error: %w", lastErr)
}
