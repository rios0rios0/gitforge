package doubles

import (
	"context"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
)

// MirrorProviderStub implements MirrorProvider for testing.
type MirrorProviderStub struct {
	ForgeProviderStub
	MigrateErr error
	Migrated   []globalEntities.MirrorInput
}

func (s *MirrorProviderStub) MigrateRepository(
	_ context.Context,
	input globalEntities.MirrorInput,
) error {
	s.Migrated = append(s.Migrated, input)
	return s.MigrateErr
}
