package git

import (
	nethttp "net/http"
	"sync"
	"time"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitclient "github.com/go-git/go-git/v5/plumbing/transport/client"

	"github.com/c-premus/documcp/internal/security"
)

// gitDialTimeout bounds the TCP dial for git HTTP(S) transport. Separate
// from go-git's CloneContext context.Context — this is the per-connection
// ceiling, not the overall clone budget.
const gitDialTimeout = 30 * time.Second

// installOnce ensures the safe HTTP transport is installed into go-git
// exactly once per process, regardless of how many *Client instances are
// constructed. InstallProtocol mutates package-level state in the go-git
// plumbing/transport/client package; the sync.Once keeps the mutation
// race-free across concurrent NewClient calls.
var installOnce sync.Once

// installSafeHTTPTransport swaps go-git's default HTTP(S) transport for one
// that re-validates every resolved IP at DialContext time against the SSRF
// block-list in internal/security. Without this, gogit.PlainCloneContext
// uses http.DefaultTransport and trusts a hostname's first DNS resolution —
// a TOCTOU gap that DNS rebinding can exploit to aim clone requests at
// 127.0.0.1 / 169.254.169.254 / RFC 1918 addresses between preflight and
// dial.
//
// Redirects are disabled (NoFollowRedirects). go-git's default
// FollowInitialRedirects policy would still route redirect targets through
// our DialContext (so they'd fail the block-list check), but disabling
// redirects outright surfaces misconfigured URLs as errors rather than
// silently chasing them. Admins must register the canonical URL.
//
// Private RFC 1918 / CGN addresses are permitted (SafeTransportAllowPrivate)
// because operators legitimately run internal git servers (self-hosted
// GitLab, Gitea, Forgejo) on private networks. Loopback, link-local, IETF
// documentation, benchmark, multicast, and reserved ranges remain blocked.
func installSafeHTTPTransport() {
	installOnce.Do(func() {
		httpClient := &nethttp.Client{
			Transport: security.SafeTransportAllowPrivate(gitDialTimeout),
		}
		opts := &githttp.ClientOptions{
			RedirectPolicy: githttp.NoFollowRedirects,
		}
		transport := githttp.NewClientWithOptions(httpClient, opts)
		gitclient.InstallProtocol("http", transport)
		gitclient.InstallProtocol("https", transport)
	})
}
