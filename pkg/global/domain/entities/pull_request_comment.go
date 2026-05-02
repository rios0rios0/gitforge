package entities

// PullRequestComment is a single comment on a pull request — either a
// PR-wide "issue comment" (no file/line context) or an inline review
// comment anchored to a specific file and line. Providers populate every
// field they can map from the underlying API; consumers filter by
// Author / FilePath / Line as needed.
//
// Defined as one shape (rather than a separate struct per surface) so
// consumers like a comment-dedup pass or a "has the bot already
// reviewed this PR?" check can iterate the full list with a single
// loop. Providers are responsible for mapping their wire shape into
// this struct (GitHub's `issue_comments` + `pulls/comments` lists land
// here as one list; Azure DevOps' `threads` API expands into one entry
// per inner comment).
type PullRequestComment struct {
	// ID is the provider-specific identifier for the comment itself.
	// Useful for posting a reply or for stable dedup keys.
	ID int64

	// ThreadID is the provider-specific identifier for the conversation
	// the comment belongs to. Zero for PR-wide comments on platforms
	// that do not group them (GitHub `issue_comments`); set on every
	// inline comment and on every Azure DevOps comment (which always
	// belongs to a thread).
	ThreadID int64

	// Body is the rendered Markdown body the user / bot posted.
	Body string

	// Author is the login (GitHub) or display name / unique name
	// (Azure DevOps) of whoever posted the comment. Used to filter
	// the bot's own comments out of (or into) a result set.
	Author string

	// FilePath is the path the inline comment is anchored to. Empty
	// for PR-wide comments.
	FilePath string

	// Line is the line number the inline comment is anchored to.
	// Zero for PR-wide comments.
	Line int

	// InReplyToID is the comment ID this comment is a reply to. Zero
	// for top-level comments. Lets a re-review pass walk a thread
	// without a separate "list replies" call.
	InReplyToID int64
}
