package entities

// ServiceType represents the type of Git hosting service.
type ServiceType int

const (
	UNKNOWN ServiceType = iota
	GITHUB
	GITLAB
	AZUREDEVOPS
	BITBUCKET
	CODECOMMIT
	// CODEBERG is intentionally excluded from ParseRemoteURL URL parsing.
	// Providers that support Codeberg must select this ServiceType via other means.
	CODEBERG
)

// serviceTypeDisplayNames maps each ServiceType to the name people know the forge by.
//
//nolint:gochecknoglobals // read-only lookup table
var serviceTypeDisplayNames = map[ServiceType]string{
	UNKNOWN:     "unknown forge",
	GITHUB:      "GitHub",
	GITLAB:      "GitLab",
	AZUREDEVOPS: "Azure DevOps",
	BITBUCKET:   "Bitbucket",
	CODECOMMIT:  "AWS CodeCommit",
	CODEBERG:    "Codeberg",
}

// DisplayName returns the forge's name as people write it ("GitHub", "Azure DevOps"),
// for log lines and errors meant to be read. It is deliberately not String: formatting a
// ServiceType with %v or %d keeps printing the number that consumers already compare
// against.
func (s ServiceType) DisplayName() string {
	if name, ok := serviceTypeDisplayNames[s]; ok {
		return name
	}
	return serviceTypeDisplayNames[UNKNOWN]
}
