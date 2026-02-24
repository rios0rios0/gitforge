package entities

// ProviderConfig describes a single Git hosting provider instance.
type ProviderConfig struct {
	Type          string   `yaml:"type"`          // "github", "gitlab", "azuredevops"
	Token         string   `yaml:"token"`         // inline, ${ENV_VAR}, or file path
	Organizations []string `yaml:"organizations"` // org names or URLs to scan
}
