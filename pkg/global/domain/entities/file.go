package entities

// File represents a file entry within a repository.
type File struct {
	Path     string
	ObjectID string
	IsDir    bool
}
