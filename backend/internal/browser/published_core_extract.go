package browser

import (
	"archive/tar"
	"archive/zip"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"
)

var ErrPublishedCoreArchive = errors.New("kernel_artifact_archive_invalid")

const maxPublishedExpandedBytes int64 = 8 << 30
const maxPublishedArchiveEntries = 100000

type checkedArchiveReader struct {
	ctx context.Context
	r   io.Reader
}

func (r checkedArchiveReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// ExtractPublishedCoreArchive validates a self-contained platform package
// before reusing the existing extract/strip-root implementation. Published
// packages contain regular files/directories; links and special files fail
// closed instead of silently installing an incomplete or escaping tree.
func ExtractPublishedCoreArchive(ctx context.Context, archivePath, format, destination string, progress func(int, string)) error {
	if ctx == nil || ctx.Err() != nil {
		return ErrPublishedCoreArchive
	}
	info, err := os.Lstat(destination)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrPublishedCoreArchive
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		return ErrPublishedCoreArchive
	}
	seen := map[string]bool{}
	var expanded int64
	count := 0
	validate := func(name string, size int64, directory bool) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > maxPublishedArchiveEntries || size < 0 || size > maxPublishedExpandedBytes-expanded || strings.ContainsAny(name, ":\x00") {
			return ErrPublishedCoreArchive
		}
		normalized := strings.ReplaceAll(name, "\\", "/")
		if strings.HasPrefix(normalized, "/") {
			return ErrPublishedCoreArchive
		}
		for _, part := range strings.Split(normalized, "/") {
			// Windows aliases (trailing dots/spaces and device names) must not
			// collapse distinct entries onto the same filesystem target.
			device := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
			reserved := device == "CON" || device == "PRN" || device == "AUX" || device == "NUL" || len(device) == 4 && (strings.HasPrefix(device, "COM") || strings.HasPrefix(device, "LPT")) && device[3] >= '1' && device[3] <= '9'
			if part == ".." || part != "." && (strings.TrimRight(part, ". ") != part || reserved) {
				return ErrPublishedCoreArchive
			}
		}
		key := strings.ToLower(path.Clean(normalized))
		if key == "." && directory {
			return nil
		}
		if key == "." || seen[key] {
			return ErrPublishedCoreArchive
		}
		seen[key] = true
		expanded += size
		return nil
	}
	if format == "zip" {
		reader, err := zip.OpenReader(archivePath)
		if err != nil {
			return ErrPublishedCoreArchive
		}
		defer reader.Close()
		for _, entry := range reader.File {
			mode := entry.Mode()
			if !mode.IsRegular() && !mode.IsDir() || entry.UncompressedSize64 > uint64(maxPublishedExpandedBytes) {
				return ErrPublishedCoreArchive
			}
			if err := validate(entry.Name, int64(entry.UncompressedSize64), mode.IsDir()); err != nil {
				return ErrPublishedCoreArchive
			}
		}
	} else if format == "tar.gz" || format == "tar.xz" {
		// The native installer assigns this filename from the fixed format;
		// signed CDN URLs are never used to infer an archive type.
		if !strings.HasSuffix(archivePath, "."+format) {
			return ErrPublishedCoreArchive
		}
		file, err := os.Open(archivePath)
		if err != nil {
			return ErrPublishedCoreArchive
		}
		defer file.Close()
		stream, closeStream, err := tarStreamReader(archivePath, file)
		if err != nil {
			return ErrPublishedCoreArchive
		}
		defer closeStream()
		reader := tar.NewReader(checkedArchiveReader{ctx: ctx, r: stream})
		for {
			entry, err := reader.Next()
			if err == io.EOF {
				break
			}
			if err != nil || entry.Typeflag != tar.TypeReg && entry.Typeflag != tar.TypeRegA && entry.Typeflag != tar.TypeDir {
				return ErrPublishedCoreArchive
			}
			if err := validate(entry.Name, entry.Size, entry.Typeflag == tar.TypeDir); err != nil {
				return ErrPublishedCoreArchive
			}
		}
	} else {
		return ErrPublishedCoreArchive
	}
	if count == 0 || ctx.Err() != nil {
		return ErrPublishedCoreArchive
	}
	if progress == nil {
		progress = func(int, string) {}
	}
	var extractErr error
	if format == "zip" {
		extractErr = extractZipArchiveAndStripRoot(archivePath, destination, progress)
	} else {
		extractErr = extractTarArchiveAndStripRoot(archivePath, destination, progress)
	}
	if extractErr != nil || ctx.Err() != nil {
		return ErrPublishedCoreArchive
	}
	// The caller validates the executable and writes its own receipt only after
	// extraction. No registration or process launch occurs in this helper.
	return nil
}
