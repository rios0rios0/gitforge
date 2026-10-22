package github

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/google/go-github/v93/github"
	globalEntities "github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
	globalHelpers "github.com/rios0rios0/gitforge/v4/pkg/global/domain/helpers"
	log "github.com/sirupsen/logrus"
)

func (p *Provider) GetFileContent(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) (string, error) {
	fileContent, _, _, err := p.client.Repositories.GetContents(
		ctx, repo.Organization, repo.Name, path,
		&gh.RepositoryContentGetOptions{},
	)
	if err != nil {
		return "", fmt.Errorf("failed to get file %q: %w", path, err)
	}
	if fileContent == nil {
		return "", fmt.Errorf("path %q is a directory, not a file", path)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return content, nil
}

func (p *Provider) ListFiles(
	ctx context.Context,
	repo globalEntities.Repository,
	pattern string,
) ([]globalEntities.File, error) {
	entries, err := p.listTreeEntries(ctx, repo)
	if err != nil {
		return nil, err
	}

	var files []globalEntities.File
	for _, entry := range entries {
		if pattern != "" && !strings.HasSuffix(entry.GetPath(), pattern) {
			continue
		}
		files = append(files, globalEntities.File{
			Path:     entry.GetPath(),
			ObjectID: entry.GetSHA(),
			IsDir:    entry.GetType() == treeType,
		})
	}

	return files, nil
}

// listTreeEntries returns every entry of the tree on the repository's default
// branch, each with its path from the root.
//
// A recursive request lists the whole tree in one response unless it is too
// large: GitHub caps that listing at 100,000 entries and 7 MB and marks a
// response it cut short as `truncated` — see
// https://docs.github.com/en/rest/git/trees#get-a-tree — so a large repository
// came back with entries silently missing. A truncated listing is replaced by a
// walk of the tree one level at a time, the method the documentation gives for
// this case, starting from the root tree the truncated response reported so a
// push to the branch in the middle cannot mix two trees.
func (p *Provider) listTreeEntries(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]*gh.TreeEntry, error) {
	tree, _, err := p.client.Git.GetTree(
		ctx, repo.Organization, repo.Name,
		strings.TrimPrefix(repo.DefaultBranch, "refs/heads/"),
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo tree: %w", err)
	}
	if !tree.GetTruncated() {
		return tree.Entries, nil
	}

	log.WithField("repository", repo.Organization+"/"+repo.Name).
		Info("GitHub truncated the recursive tree listing; listing the tree one level at a time")
	return p.walkTree(ctx, repo, tree.GetSHA(), "", make(map[string]struct{}))
}

// walkTree lists the tree sha one level at a time and returns its entries, and
// those of every tree below it, with prefix, the tree's own path from the root,
// in front of each path. onPath holds the trees from the root down to this one.
// A tree that names one of them cannot occur in a real repository and would make
// the walk endless, and a level that GitHub truncates even without recursion
// cannot be listed completely at all; both are errors rather than a partial list.
func (p *Provider) walkTree(
	ctx context.Context,
	repo globalEntities.Repository,
	sha, prefix string,
	onPath map[string]struct{},
) ([]*gh.TreeEntry, error) {
	if _, cycle := onPath[sha]; cycle {
		return nil, fmt.Errorf("%w: tree %s at /%s contains itself", errIncompleteListing, sha, prefix)
	}

	tree, _, err := p.client.Git.GetTree(ctx, repo.Organization, repo.Name, sha, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo tree %s at /%s: %w", sha, prefix, err)
	}
	if tree.GetTruncated() {
		return nil, fmt.Errorf(
			"%w: tree %s at /%s is truncated even without recursion", errIncompleteListing, sha, prefix,
		)
	}

	onPath[sha] = struct{}{}
	defer delete(onPath, sha)

	var entries []*gh.TreeEntry
	for _, entry := range tree.Entries {
		path := entry.GetPath()
		if prefix != "" {
			path = prefix + "/" + path
		}
		entry.Path = &path
		entries = append(entries, entry)

		if entry.GetType() != treeType {
			continue
		}
		subtree, walkErr := p.walkTree(ctx, repo, entry.GetSHA(), path, onPath)
		if walkErr != nil {
			return nil, walkErr
		}
		entries = append(entries, subtree...)
	}

	return entries, nil
}

func (p *Provider) GetTags(
	ctx context.Context,
	repo globalEntities.Repository,
) ([]string, error) {
	var allTags []string
	opts := &gh.ListOptions{PerPage: perPage}

	for {
		tags, resp, err := p.client.Repositories.ListTags(
			ctx, repo.Organization, repo.Name, opts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list tags: %w", err)
		}

		for _, tag := range tags {
			allTags = append(allTags, tag.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	globalHelpers.SortVersionsDescending(allTags)
	return allTags, nil
}

func (p *Provider) HasFile(
	ctx context.Context,
	repo globalEntities.Repository,
	path string,
) bool {
	_, err := p.GetFileContent(ctx, repo, path)
	return err == nil
}

func (p *Provider) CreateBranchWithChanges(
	ctx context.Context,
	repo globalEntities.Repository,
	input globalEntities.BranchInput,
) error {
	owner := repo.Organization
	repoName := repo.Name

	baseBranch := strings.TrimPrefix(input.BaseBranch, "refs/heads/")
	baseRef, _, err := p.client.Git.GetRef(
		ctx, owner, repoName, "refs/heads/"+baseBranch,
	)
	if err != nil {
		return fmt.Errorf("failed to get base branch ref: %w", err)
	}
	baseSHA := baseRef.Object.GetSHA()

	baseCommit, _, err := p.client.Git.GetCommit(
		ctx, owner, repoName, baseSHA,
	)
	if err != nil {
		return fmt.Errorf("failed to get base commit: %w", err)
	}

	var treeEntries []*gh.TreeEntry
	for _, change := range input.Changes {
		content := change.Content
		path := strings.TrimPrefix(change.Path, "/")
		mode := blobMode
		entryType := blobType
		treeEntries = append(treeEntries, &gh.TreeEntry{
			Path:    &path,
			Mode:    &mode,
			Type:    &entryType,
			Content: &content,
		})
	}

	newTree, _, err := p.client.Git.CreateTree(
		ctx, owner, repoName, baseCommit.Tree.GetSHA(), treeEntries,
	)
	if err != nil {
		return fmt.Errorf("failed to create tree: %w", err)
	}

	newCommit, _, err := p.client.Git.CreateCommit(
		ctx, owner, repoName,
		gh.Commit{
			Message: &input.CommitMessage,
			Tree:    newTree,
			Parents: []*gh.Commit{{SHA: &baseSHA}},
		},
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}

	branchRef := "refs/heads/" + input.BranchName
	_, _, err = p.client.Git.CreateRef(
		ctx, owner, repoName,
		gh.CreateRef{
			Ref: branchRef,
			SHA: newCommit.GetSHA(),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}
