package codeberg

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

// errIncompleteListing reports a listing that could not be read to its end: a
// walk that stopped making progress while the server still announced more.
// Returning what was read so far would hand callers a partial listing they could
// not tell apart from a complete one.
var errIncompleteListing = errors.New("listing could not be read completely")

// walkPages reads a Forgejo listing that pages with `page` and `limit`, page 1
// first, fetching each page with fetch and handing visit, in order, every item
// whose key it has not seen before. listing names the listing in the log.
//
// It returns once a page comes back shorter than perPage, which is the page size
// every caller asks for and no more than Forgejo serves by default
// (MAX_RESPONSE_ITEMS, 50 — see
// https://forgejo.org/docs/latest/admin/config-cheat-sheet/), so a short page
// means the listing is exhausted. It also returns as soon as visit reports it
// has what it was looking for. A full page that brings no new key ends the walk
// too, the same guard the Azure DevOps listings carry: a server that ignores
// `page` would otherwise serve page 1 forever. That stop is logged, because what
// was read before it may be incomplete.
func walkPages[T any, K comparable](
	listing string,
	fetch func(page int) ([]T, error),
	key func(T) K,
	visit func(T) bool,
) error {
	seen := make(map[K]struct{})
	for page := 1; ; page++ {
		items, err := fetch(page)
		if err != nil {
			return err
		}

		added := 0
		for _, item := range items {
			itemKey := key(item)
			if _, dup := seen[itemKey]; dup {
				continue
			}
			seen[itemKey] = struct{}{}
			added++
			if visit(item) {
				return nil
			}
		}

		if len(items) < perPage {
			return nil
		}
		if added == 0 {
			log.WithFields(log.Fields{"listing": listing, "page": page}).Warn(
				"Forgejo served a full page whose items were all listed already; " +
					"stopped paging, so the listing may be incomplete",
			)
			return nil
		}
	}
}
