package browser

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
)

const MaxPublishedCoreArchiveBytes int64 = 4 << 30

var ErrPublishedCoreIntegrity = errors.New("kernel_artifact_integrity_failed")

// VerifyPublishedCoreArchive binds an already downloaded, private staging file
// to platform-issued metadata before any extraction or core registration.
// It never executes the artifact, writes its contents, or accepts a directory.
func VerifyPublishedCoreArchive(filePath string, expectedSize int64, expectedSHA256 string) error {
	if expectedSize <= 0 || expectedSize > MaxPublishedCoreArchiveBytes || len(expectedSHA256) != sha256.Size*2 || expectedSHA256 != strings.ToLower(expectedSHA256) {
		return ErrPublishedCoreIntegrity
	}
	if _, err := hex.DecodeString(expectedSHA256); err != nil {
		return ErrPublishedCoreIntegrity
	}
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != expectedSize {
		return ErrPublishedCoreIntegrity
	}
	file, err := os.Open(filePath)
	if err != nil {
		return ErrPublishedCoreIntegrity
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil || !os.SameFile(info, openedInfo) || openedInfo.Size() != expectedSize {
		return ErrPublishedCoreIntegrity
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, expectedSize+1))
	if err != nil || n != expectedSize || hex.EncodeToString(hash.Sum(nil)) != expectedSHA256 {
		return ErrPublishedCoreIntegrity
	}
	return nil
}
