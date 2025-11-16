package userstore

const (
	// Version is the semantic version of the userstore module.
	Version = "0.2.0"
)

// VersionString returns the semantic version for convenient logging/printing.
func VersionString() string {
	return Version
}
