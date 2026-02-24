package infrastructure

import (
	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// ProviderFactory is a constructor function that creates a ForgeProvider given an auth token.
type ProviderFactory func(token string) globalEntities.ForgeProvider
