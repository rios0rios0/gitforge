//go:build unit

package infrastructure_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	globalEntities "github.com/rios0rios0/gitforge/pkg/global/domain/entities"
	registryInfra "github.com/rios0rios0/gitforge/pkg/registry/infrastructure"
)

func TestServiceTypeToProviderName(t *testing.T) {
	t.Parallel()

	t.Run("should return github for GITHUB service type", func(t *testing.T) {
		t.Parallel()

		// given
		serviceType := globalEntities.GITHUB

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Equal(t, "github", name)
	})

	t.Run("should return gitlab for GITLAB service type", func(t *testing.T) {
		t.Parallel()

		// given
		serviceType := globalEntities.GITLAB

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Equal(t, "gitlab", name)
	})

	t.Run("should return azuredevops for AZUREDEVOPS service type", func(t *testing.T) {
		t.Parallel()

		// given
		serviceType := globalEntities.AZUREDEVOPS

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Equal(t, "azuredevops", name)
	})

	t.Run("should return empty string for UNKNOWN service type", func(t *testing.T) {
		t.Parallel()

		// given
		serviceType := globalEntities.UNKNOWN

		// when
		name := registryInfra.ServiceTypeToProviderName(serviceType)

		// then
		assert.Empty(t, name)
	})
}
