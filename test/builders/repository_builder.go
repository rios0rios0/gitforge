package builders

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

type RepositoryBuilder struct {
	*testkit.BaseBuilder

	id            string
	name          string
	organization  string
	project       string
	defaultBranch string
	remoteURL     string
	sshURL        string
	providerName  string
}

func NewRepositoryBuilder() *RepositoryBuilder {
	return &RepositoryBuilder{
		BaseBuilder:   testkit.NewBaseBuilder(),
		id:            "test-repo-id",
		name:          "test-repo",
		organization:  "test-org",
		project:       "",
		defaultBranch: "main",
		remoteURL:     "https://github.com/test-org/test-repo.git",
		sshURL:        "",
		providerName:  "",
	}
}

func (b *RepositoryBuilder) WithID(id string) *RepositoryBuilder {
	b.id = id
	return b
}

func (b *RepositoryBuilder) WithName(name string) *RepositoryBuilder {
	b.name = name
	return b
}

func (b *RepositoryBuilder) WithOrganization(organization string) *RepositoryBuilder {
	b.organization = organization
	return b
}

func (b *RepositoryBuilder) WithProject(project string) *RepositoryBuilder {
	b.project = project
	return b
}

func (b *RepositoryBuilder) WithDefaultBranch(defaultBranch string) *RepositoryBuilder {
	b.defaultBranch = defaultBranch
	return b
}

func (b *RepositoryBuilder) WithRemoteURL(remoteURL string) *RepositoryBuilder {
	b.remoteURL = remoteURL
	return b
}

func (b *RepositoryBuilder) WithSSHURL(sshURL string) *RepositoryBuilder {
	b.sshURL = sshURL
	return b
}

func (b *RepositoryBuilder) WithProviderName(providerName string) *RepositoryBuilder {
	b.providerName = providerName
	return b
}

func (b *RepositoryBuilder) Build() any {
	return globalEntities.Repository{
		ID:            b.id,
		Name:          b.name,
		Organization:  b.organization,
		Project:       b.project,
		DefaultBranch: b.defaultBranch,
		RemoteURL:     b.remoteURL,
		SSHURL:        b.sshURL,
		ProviderName:  b.providerName,
	}
}
