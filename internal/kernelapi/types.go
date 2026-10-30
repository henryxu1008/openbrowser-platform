// Package kernelapi is the Go-only client for the platform kernel release catalog.
package kernelapi

import (
	"errors"
	"net/http"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/mod/semver"
)

var (
	ErrConfiguration = errors.New("kernel_service_configuration_required")
	ErrUnavailable   = errors.New("kernel_catalog_unavailable")
	ErrNotFound      = errors.New("kernel_release_not_found")
	ErrPermission    = errors.New("kernel_release_forbidden")
)

const MaxArtifactBytes int64 = 2 << 30

type Auth interface {
	Check(uint64) error
	AuthorizeRequest(uint64, *http.Request) error
}

type Release struct {
	ReleaseID                 string `json:"releaseId"`
	Version                   string `json:"version"`
	OS                        string `json:"os"`
	Arch                      string `json:"arch"`
	MinClientVersion          string `json:"minClientVersion"`
	MaxClientVersionExclusive string `json:"maxClientVersionExclusive"`
	Recommended               bool   `json:"recommended"`
}

// Artifact remains native-only even if accidentally serialized by a caller.
type Artifact struct {
	Release
	ArchiveFormat string `json:"-"`
	Size          int64  `json:"-"`
	SHA256        string `json:"-"`
	DownloadURL   string `json:"-"`
}

func ValidReleaseID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func validClientVersion(value string) bool {
	base := strings.SplitN(strings.SplitN(value, "+", 2)[0], "-", 2)[0]
	return len(value) <= 64 && len(strings.Split(base, ".")) == 3 && semver.IsValid("v"+value)
}

func (r Release) compatible(osName, arch, clientVersion string) bool {
	if !ValidReleaseID(r.ReleaseID) || r.OS != osName || r.Arch != arch || r.Version == "" || len(r.Version) > 64 || strings.TrimSpace(r.Version) != r.Version {
		return false
	}
	for _, char := range r.Version {
		if unicode.IsControl(char) {
			return false
		}
	}
	if !validClientVersion(r.MinClientVersion) || semver.Compare("v"+clientVersion, "v"+r.MinClientVersion) < 0 {
		return false
	}
	if r.MaxClientVersionExclusive != "" {
		return validClientVersion(r.MaxClientVersionExclusive) && semver.Compare("v"+r.MinClientVersion, "v"+r.MaxClientVersionExclusive) < 0 && semver.Compare("v"+clientVersion, "v"+r.MaxClientVersionExclusive) < 0
	}
	return true
}
