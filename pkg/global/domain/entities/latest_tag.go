package entities

import (
	"time"

	"github.com/Masterminds/semver/v3"
)

// LatestTag holds information about the latest Git tag.
type LatestTag struct {
	Tag  *semver.Version
	Date time.Time
}
