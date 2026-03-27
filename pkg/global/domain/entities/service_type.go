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
