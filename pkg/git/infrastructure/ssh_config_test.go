package infrastructure_test

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitops "github.com/rios0rios0/gitforge/v4/pkg/git/infrastructure"
)

func TestResolveSSHHost(t *testing.T) {
	t.Parallel()

	t.Run("should resolve an alias from a Windows profile's file the way ssh.exe reads it", func(t *testing.T) {
		t.Parallel()

		// given -- the alias layout that keeps one key per account for the same forge
		root := t.TempDir()
		profile := filepath.Join(root, "c", "Users", "Someone")
		config := writeSSHFixture(t, filepath.Join(profile, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n"+
			"  Port 443\n"+
			"  IdentityFile ~/.ssh/laptop@mine.ssh.pub\n"+
			"  IdentitiesOnly yes\n")
		sources := []gitops.SSHConfigSource{{Path: config, Home: profile, Windows: true}}

		// when
		settings := gitops.ResolveSSHHost(sources, root, "github.com-mine", "git")

		// then -- ~ is the Windows profile, not the WSL home
		assert.Equal(t, gitops.SSHHostSettings{
			HostName:       "ssh.github.com",
			Port:           443,
			IdentityFiles:  []string{filepath.Join(profile, ".ssh", "laptop@mine.ssh.pub")},
			IdentitiesOnly: true,
		}, settings)
	})

	t.Run("should take each keyword from the first file that sets it and accumulate IdentityFile", func(t *testing.T) {
		t.Parallel()

		// given -- the WSL home's file is read before the Windows profile's
		root := t.TempDir()
		linuxHome := filepath.Join(root, "home")
		profile := filepath.Join(root, "c", "Users", "Someone")
		linuxConfig := writeSSHFixture(t, filepath.Join(linuxHome, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  Port 2222\n"+
			"  IdentityFile ~/.ssh/first.pub\n")
		windowsConfig := writeSSHFixture(t, filepath.Join(profile, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n"+
			"  Port 443\n"+
			"  IdentityFile ~/.ssh/second.pub\n")
		sources := []gitops.SSHConfigSource{
			{Path: linuxConfig, Home: linuxHome},
			{Path: windowsConfig, Home: profile, Windows: true},
		}

		// when
		settings := gitops.ResolveSSHHost(sources, root, "github.com-mine", "git")

		// then
		assert.Equal(t, "ssh.github.com", settings.HostName, "only the Windows file sets HostName")
		assert.Equal(t, 2222, settings.Port, "the first file that sets Port wins")
		assert.Equal(t, []string{
			filepath.Join(linuxHome, ".ssh", "first.pub"),
			filepath.Join(profile, ".ssh", "second.pub"),
		}, settings.IdentityFiles, "IdentityFile accumulates, each path against its own file's home")
		assert.False(t, settings.IdentitiesOnly)
	})

	t.Run("should map a Windows drive path into the WSL automount root", func(t *testing.T) {
		t.Parallel()

		// given
		root := t.TempDir()
		profile := filepath.Join(root, "c", "Users", "Someone")
		config := writeSSHFixture(t, filepath.Join(profile, ".ssh", "config"), ""+
			"Host dev.azure.com-work\n"+
			"  IdentityFile C:\\Users\\Someone\\.ssh\\work.pub\n"+
			"  IdentityFile ~\\.ssh\\backup.pub\n")
		sources := []gitops.SSHConfigSource{{Path: config, Home: profile, Windows: true}}

		// when
		settings := gitops.ResolveSSHHost(sources, root, "dev.azure.com-work", "git")

		// then
		assert.Equal(t, []string{
			filepath.Join(root, "c", "Users", "Someone", ".ssh", "work.pub"),
			filepath.Join(profile, ".ssh", "backup.pub"),
		}, settings.IdentityFiles)
	})

	t.Run("should expand the tokens OpenSSH documents for HostName and IdentityFile", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host *.internal\n"+
			"  HostName %h.example.com\n"+
			"  IdentityFile %d/.ssh/%h-%r-100%%.pub\n"+
			"  IdentityFile ~/.ssh/%k.pub\n")
		sources := []gitops.SSHConfigSource{{Path: config, Home: home}}

		// when
		settings := gitops.ResolveSSHHost(sources, "", "git.internal", "git")

		// then -- %h in IdentityFile is the HostName, and an unknown token is left as written
		assert.Equal(t, "git.internal.example.com", settings.HostName)
		assert.Equal(t, []string{
			filepath.Join(home, ".ssh", "git.internal.example.com-git-100%.pub"),
			filepath.Join(home, ".ssh", "%k.pub"),
		}, settings.IdentityFiles)
	})

	t.Run("should say nothing about a host no file names", func(t *testing.T) {
		t.Parallel()

		// given
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n")
		sources := []gitops.SSHConfigSource{{Path: config, Home: home}}

		// when
		settings := gitops.ResolveSSHHost(sources, "", "gitlab.com", "git")

		// then
		assert.Equal(t, gitops.SSHHostSettings{}, settings)
	})

	t.Run("should skip a file it cannot parse and keep the others", func(t *testing.T) {
		t.Parallel()

		// given -- kevinburke/ssh_config refuses Match exec, so the whole first file is lost
		home := t.TempDir()
		unparseable := writeSSHFixture(t, filepath.Join(home, "unparseable"), ""+
			"Match exec \"true\"\n"+
			"  HostName wrong.example.com\n")
		valid := writeSSHFixture(t, filepath.Join(home, "valid"), ""+
			"Host github.com-mine\n"+
			"  HostName ssh.github.com\n")
		sources := []gitops.SSHConfigSource{{Path: unparseable, Home: home}, {Path: valid, Home: home}}

		// when
		settings := gitops.ResolveSSHHost(sources, "", "github.com-mine", "git")

		// then
		assert.Equal(t, "ssh.github.com", settings.HostName)
	})

	t.Run("should read a CRLF file and ignore a file that does not exist", func(t *testing.T) {
		t.Parallel()

		// given -- a Windows editor writes CRLF line endings
		home := t.TempDir()
		config := writeSSHFixture(t, filepath.Join(home, ".ssh", "config"),
			"Host github.com-mine\r\n  HostName ssh.github.com\r\n  Port 443\r\n")
		sources := []gitops.SSHConfigSource{
			{Path: filepath.Join(home, "absent"), Home: home},
			{Path: config, Home: home},
		}

		// when
		settings := gitops.ResolveSSHHost(sources, "", "github.com-mine", "git")

		// then
		assert.Equal(t, "ssh.github.com", settings.HostName)
		assert.Equal(t, 443, settings.Port)
	})
}

// TestLoadSSHConfigFilesReporting is deliberately not parallel: it captures the entries of
// the global logger, which every other test writes to as well.
//
//nolint:paralleltest // test captures the global logger, which parallel tests write to
func TestLoadSSHConfigFilesReporting(t *testing.T) {
	//nolint:paralleltest // test captures the global logger, which parallel tests write to
	t.Run(
		"should report an unparseable system file at debug level and the operator's own as a warning",
		func(t *testing.T) {
			// given -- the parser accepts no Match criterion but host and all
			captured := captureLogs(t)
			dir := t.TempDir()
			unparseable := "Match final all\n  HostName wrong.example.com\n"
			system := writeSSHFixture(t, filepath.Join(dir, "system", "ssh_config"), unparseable)
			own := writeSSHFixture(t, filepath.Join(dir, "home", ".ssh", "config"), unparseable)

			// when
			loaded := gitops.LoadSSHConfigFiles([]gitops.SSHConfigSource{
				{Path: system, Home: dir, System: true},
				{Path: own, Home: dir},
			})

			// then -- both are skipped; only the operator's own is worth a warning on every run
			assert.Zero(t, loaded)
			assert.Equal(t, []log.Level{log.DebugLevel}, captured.levelsMentioning(system))
			assert.Equal(t, []log.Level{log.WarnLevel}, captured.levelsMentioning(own))
		},
	)
}

// logCapture records the level and message of every entry the global logger writes. Entries
// are copied, not kept, because logrus reuses them.
type logCapture struct {
	mu      sync.Mutex
	records []logRecord
}

// logRecord is one captured log entry.
type logRecord struct {
	level   log.Level
	message string
}

// captureLogs captures the global logger's entries, at every level, until the test ends.
func captureLogs(t *testing.T) *logCapture {
	t.Helper()

	capture := &logCapture{}
	standard := log.StandardLogger()
	previousHooks := standard.ReplaceHooks(make(log.LevelHooks))
	previousLevel := standard.GetLevel()
	standard.AddHook(capture)
	standard.SetLevel(log.DebugLevel)
	t.Cleanup(func() {
		standard.ReplaceHooks(previousHooks)
		standard.SetLevel(previousLevel)
	})
	return capture
}

// Levels captures every level.
func (c *logCapture) Levels() []log.Level {
	return log.AllLevels
}

// Fire records an entry.
func (c *logCapture) Fire(entry *log.Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.records = append(c.records, logRecord{level: entry.Level, message: entry.Message})
	return nil
}

// recordsMentioning returns the captured entries whose message mentions text, in the order
// they were written.
func (c *logCapture) recordsMentioning(text string) []logRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	var records []logRecord
	for _, record := range c.records {
		if strings.Contains(record.message, text) {
			records = append(records, record)
		}
	}
	return records
}

// levelsMentioning returns the levels of the captured entries whose message mentions text.
func (c *logCapture) levelsMentioning(text string) []log.Level {
	records := c.recordsMentioning(text)
	levels := make([]log.Level, 0, len(records))
	for _, record := range records {
		levels = append(levels, record.level)
	}
	return levels
}

func TestSSHHostTarget(t *testing.T) {
	t.Parallel()

	t.Run("should dial the HostName and Port the alias names", func(t *testing.T) {
		t.Parallel()

		// given -- go-git parses an scp-like remote to port 22, which the URL cannot carry
		endpoint, err := transport.NewEndpoint("git@github.com-mine:owner/repo.git")
		require.NoError(t, err)
		settings := gitops.SSHHostSettings{HostName: "ssh.github.com", Port: 443}

		// when
		target := gitops.SSHHostTarget(settings, endpoint)

		// then
		assert.Equal(t, "ssh.github.com", target.Host)
		assert.Equal(t, 443, target.Port)
		assert.Equal(t, endpoint.Path, target.Path)
		assert.Equal(t, "github.com-mine", endpoint.Host, "the remote's own endpoint is left alone")
	})

	t.Run("should keep a port the URL names over the configured Port", func(t *testing.T) {
		t.Parallel()

		// given -- as `ssh -p` wins over the file
		endpoint, err := transport.NewEndpoint("ssh://git@forge-alias:2222/owner/repo.git")
		require.NoError(t, err)
		settings := gitops.SSHHostSettings{HostName: "forge.example.com", Port: 443}

		// when
		target := gitops.SSHHostTarget(settings, endpoint)

		// then
		assert.Equal(t, "forge.example.com", target.Host)
		assert.Equal(t, 2222, target.Port)
	})

	t.Run("should return the endpoint itself when the alias names neither", func(t *testing.T) {
		t.Parallel()

		// given
		endpoint, err := transport.NewEndpoint("git@github.com:owner/repo.git")
		require.NoError(t, err)

		// when
		target := gitops.SSHHostTarget(gitops.SSHHostSettings{}, endpoint)

		// then
		assert.Same(t, endpoint, target)
	})
}

func TestWindowsSSHConfigSourcesUnder(t *testing.T) {
	t.Parallel()

	t.Run("should find every mounted profile's ssh_config with the profile as its home", func(t *testing.T) {
		t.Parallel()

		// given
		root := t.TempDir()
		first := writeSSHFixture(t, filepath.Join(root, "c", "Users", "First", ".ssh", "config"), "")
		second := writeSSHFixture(t, filepath.Join(root, "d", "Users", "Second", ".ssh", "config"), "")
		writeSSHFixture(t, filepath.Join(root, "wsl", "Users", "NotADrive", ".ssh", "config"), "")

		// when
		sources := gitops.WindowsSSHConfigSourcesUnder(root)

		// then
		assert.ElementsMatch(t, []gitops.SSHConfigSource{
			{Path: first, Home: filepath.Join(root, "c", "Users", "First"), Windows: true},
			{Path: second, Home: filepath.Join(root, "d", "Users", "Second"), Windows: true},
		}, sources)
	})
}

func TestDescribeSSHConfigSearch(t *testing.T) {
	t.Parallel()

	t.Run("should name the Windows profile's file when it is read", func(t *testing.T) {
		t.Parallel()

		// given / when
		description := gitops.DescribeSSHConfigSearch(true, true)

		// then
		assert.Contains(t, description, "~/.ssh/config")
		assert.Contains(t, description, "the Windows profile's .ssh/config")
		assert.NotContains(t, description, "read only when")
	})

	t.Run(
		"should say when the Windows profile's file is read on a WSL machine that does not read it",
		func(t *testing.T) {
			t.Parallel()

			// given -- an alias defined only for ssh.exe, on a machine whose git runs the Linux ssh

			// when
			description := gitops.DescribeSSHConfigSearch(true, false)

			// then
			assert.Contains(t, description, "/etc/ssh/ssh_config")
			assert.Contains(t, description, "read only when git runs Windows' ssh.exe")
			assert.Contains(t, description, "core.sshCommand")
		},
	)

	t.Run("should name only the Linux files elsewhere", func(t *testing.T) {
		t.Parallel()

		// given / when
		description := gitops.DescribeSSHConfigSearch(false, false)

		// then
		assert.Equal(t, "~/.ssh/config and /etc/ssh/ssh_config", description)
	})
}
