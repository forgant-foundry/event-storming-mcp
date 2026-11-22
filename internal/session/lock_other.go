//go:build !unix

package session

// lockFile is a no-op where flock isn't available. The revision check in Save
// still catches most concurrent writes; only a write racing inside the same
// instant can slip through.
func lockFile(string) (func(), error) {
	return func() {}, nil
}
