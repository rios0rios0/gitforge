package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

func TestResolveMergeOptions(t *testing.T) {
	t.Parallel()

	// Pins the `MergeOption` resolution contract that providers consume
	// when deciding whether to set `bypassPolicy=true` on the wire. A
	// regression here would silently flip auto-merge from
	// "bypass branch policies" back to "respect branch policies", which
	// is the same failure mode that surfaced live on `code-guru` smoke
	// PRs (an internal repo's required-reviewers policy rejecting
	// the merge with `GitPullRequestUpdateRejectedByPolicyException`).

	t.Run("should default to bypass disabled when no options are passed", func(t *testing.T) {
		t.Parallel()

		// when
		got := entities.ResolveMergeOptions()

		// then
		assert.False(
			t,
			got.Enabled,
			"the zero value MUST be 'do not bypass'; otherwise every call to MergePullRequest would silently bypass branch policy",
		)
		assert.Empty(t, got.Reason)
	})

	t.Run("should enable bypass and propagate the audit reason", func(t *testing.T) {
		t.Parallel()

		// when
		got := entities.ResolveMergeOptions(
			entities.WithBypassPolicy("auto-merged by code-guru trivial PR policy"),
		)

		// then
		assert.True(t, got.Enabled)
		assert.Equal(t, "auto-merged by code-guru trivial PR policy", got.Reason,
			"the reason MUST reach the provider unchanged so it lands in ADO's audit trail")
	})

	t.Run("should fall back to a non-empty audit reason when the caller passes an empty string", func(t *testing.T) {
		t.Parallel()

		// given: ADO rejects `bypassPolicy=true` with an empty `bypassReason`,
		// so the helper substitutes a literal "bypass" rather than passing
		// the empty string through. Callers that care about the audit text
		// should always pass a meaningful reason.
		got := entities.ResolveMergeOptions(entities.WithBypassPolicy(""))

		// then
		assert.True(t, got.Enabled)
		assert.Equal(t, "bypass", got.Reason)
	})

	t.Run("should ignore nil entries in the option slice", func(t *testing.T) {
		t.Parallel()

		// given: defensive — callers building option slices dynamically may
		// emit a nil for an unselected option. The resolver MUST not panic.
		var opts []entities.MergeOption
		opts = append(opts, nil, entities.WithBypassPolicy("force"))

		// when
		got := entities.ResolveMergeOptions(opts...)

		// then
		assert.True(t, got.Enabled)
		assert.Equal(t, "force", got.Reason)
	})
}
