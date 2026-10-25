//go:build !windows

package apppath

// Only Windows adopts installed legacy databases. This stub also permits
// platform-independent path fixtures to exercise the Windows layout mapping.
func holdLegacyDatabaseFiles(paths []string) (func(), error) { return func() {}, nil }
