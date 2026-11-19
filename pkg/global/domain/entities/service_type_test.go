package entities_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rios0rios0/gitforge/v4/pkg/global/domain/entities"
)

func TestServiceTypeDisplayName(t *testing.T) {
	t.Parallel()

	cases := map[entities.ServiceType]string{
		entities.UNKNOWN:     "unknown forge",
		entities.GITHUB:      "GitHub",
		entities.GITLAB:      "GitLab",
		entities.AZUREDEVOPS: "Azure DevOps",
		entities.BITBUCKET:   "Bitbucket",
		entities.CODECOMMIT:  "AWS CodeCommit",
		entities.CODEBERG:    "Codeberg",
	}
	for serviceType, expected := range cases {
		t.Run(fmt.Sprintf("should return %q when the service type is %d", expected, serviceType), func(t *testing.T) {
			t.Parallel()

			// given
			subject := serviceType

			// when
			name := subject.DisplayName()

			// then
			assert.Equal(t, expected, name)
		})
	}

	t.Run("should return the unknown forge name when the service type is out of range", func(t *testing.T) {
		t.Parallel()

		// given
		subject := entities.ServiceType(99)

		// when
		name := subject.DisplayName()

		// then
		assert.Equal(t, "unknown forge", name)
	})

	t.Run("should keep printing the number when the service type is formatted with %v", func(t *testing.T) {
		t.Parallel()

		// given
		subject := entities.GITHUB

		// when
		formatted := fmt.Sprintf("%v", subject)

		// then
		assert.Equal(t, "1", formatted)
	})
}
