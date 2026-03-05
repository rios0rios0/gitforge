package builders

import (
	entities "github.com/rios0rios0/gitforge/pkg/config/domain/entities"
	testkit "github.com/rios0rios0/testkit/pkg/test"
)

type ProviderConfigBuilder struct {
	*testkit.BaseBuilder

	configType    string
	token         string
	organizations []string
}

func NewProviderConfigBuilder() *ProviderConfigBuilder {
	return &ProviderConfigBuilder{
		BaseBuilder:   testkit.NewBaseBuilder(),
		configType:    "github",
		token:         "test-token",
		organizations: []string{"test-org"},
	}
}

func (b *ProviderConfigBuilder) WithType(t string) *ProviderConfigBuilder {
	b.configType = t
	return b
}

func (b *ProviderConfigBuilder) WithToken(token string) *ProviderConfigBuilder {
	b.token = token
	return b
}

func (b *ProviderConfigBuilder) WithOrganizations(orgs []string) *ProviderConfigBuilder {
	b.organizations = orgs
	return b
}

func (b *ProviderConfigBuilder) Build() any {
	return entities.ProviderConfig{
		Type:          b.configType,
		Token:         b.token,
		Organizations: b.organizations,
	}
}
