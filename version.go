package userstore

const (
	// Version is the semantic version of the userstore module.
	Version = "0.4.1"
)

// VersionString returns the semantic version for convenient logging/printing.
func VersionString() string {
	return Version
}
