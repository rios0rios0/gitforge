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
)
