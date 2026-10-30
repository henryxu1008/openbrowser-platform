package browser

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

type publishedZipEntry struct {
	name string
	mode os.FileMode
}

func publishedZipFixture(t *testing.T, entries []publishedZipEntry) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "artifact.zip")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		h.SetMode(entry.mode)
		out, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write([]byte("offline fixture, never execute")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestPublishedCoreArchiveExtractsVerifiedSelfContainedShape(t *testing.T) {
	archive := publishedZipFixture(t, []publishedZipEntry{{"bundle/" + CoreExecutableCandidates()[0], 0755}})
	destination := t.TempDir()
	if err := ExtractPublishedCoreArchive(context.Background(), archive, "zip", destination, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := FindCoreExecutable(destination); !ok {
		t.Fatal("published executable was not made available in the staging directory")
	}
}

func TestPublishedCoreArchiveSupportsDeclaredTarFormats(t *testing.T) {
	for _, format := range []string{"tar.gz", "tar.xz"} {
		t.Run(format, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "artifact."+format)
			f, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			var compressor io.WriteCloser
			if format == "tar.gz" {
				compressor = gzip.NewWriter(f)
			} else {
				compressor, err = xz.NewWriter(f)
				if err != nil {
					t.Fatal(err)
				}
			}
			w := tar.NewWriter(compressor)
			payload := []byte("offline non-executable fixture")
			if err := w.WriteHeader(&tar.Header{Name: "bundle/" + CoreExecutableCandidates()[0], Mode: 0755, Size: int64(len(payload)), Typeflag: tar.TypeReg}); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(payload); err != nil {
				t.Fatal(err)
			}
			for _, closer := range []io.Closer{w, compressor, f} {
				if err := closer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			destination := t.TempDir()
			if err := ExtractPublishedCoreArchive(context.Background(), file, format, destination, nil); err != nil {
				t.Fatal(err)
			}
			if _, _, found := FindCoreExecutable(destination); !found {
				t.Fatal("declared tar format did not produce its executable")
			}
		})
	}
}

func TestPublishedCoreArchiveRejectsEscapesLinksAndDuplicatePathsBeforeWriting(t *testing.T) {
	for _, entries := range [][]publishedZipEntry{
		{{"../escape.exe", 0755}},
		{{"bundle/../../escape.exe", 0755}},
		{{"/absolute.exe", 0755}},
		{{"C:\\foreign\\escape.exe", 0755}},
		{{"bundle/core.exe:stream", 0755}},
		{{"bundle/chrome.exe.", 0755}},
		{{"bundle/CON.txt", 0755}},
		{{"bundle/link", os.ModeSymlink | 0755}},
		{{"bundle/chrome.exe", 0755}, {"bundle/CHROME.EXE", 0755}},
	} {
		archive := publishedZipFixture(t, entries)
		destination := t.TempDir()
		if !errors.Is(ExtractPublishedCoreArchive(context.Background(), archive, "zip", destination, nil), ErrPublishedCoreArchive) {
			t.Fatal("unsafe archive accepted")
		}
		contents, err := os.ReadDir(destination)
		if err != nil || len(contents) != 0 {
			t.Fatal("invalid archive wrote output before validation")
		}
	}
}

func TestPublishedCoreArchiveRejectsTarLinksAndExpansionMetadata(t *testing.T) {
	for _, header := range []*tar.Header{
		{Name: "bundle/link", Linkname: "../../outside", Typeflag: tar.TypeSymlink},
		{Name: "bundle/hardlink", Linkname: "/outside", Typeflag: tar.TypeLink},
		{Name: "bundle/huge.exe", Size: maxPublishedExpandedBytes + 1, Typeflag: tar.TypeReg},
	} {
		file := filepath.Join(t.TempDir(), "artifact.tar.gz")
		f, err := os.Create(file)
		if err != nil {
			t.Fatal(err)
		}
		gz := gzip.NewWriter(f)
		tarWriter := tar.NewWriter(gz)
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		// The oversized entry deliberately has no body; the header must be
		// rejected before seeking/allocating its advertised expansion size.
		_ = tarWriter.Close()
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		destination := t.TempDir()
		if !errors.Is(ExtractPublishedCoreArchive(context.Background(), file, "tar.gz", destination, nil), ErrPublishedCoreArchive) {
			t.Fatal("tar link or oversized entry accepted")
		}
		contents, _ := os.ReadDir(destination)
		if len(contents) != 0 {
			t.Fatal("rejected tar wrote output")
		}
	}
}

func TestPublishedCoreArchiveDoesNotOverwriteOrIgnoreCancellation(t *testing.T) {
	archive := publishedZipFixture(t, []publishedZipEntry{{"bundle/chrome.exe", 0755}})
	destination := t.TempDir()
	existing := filepath.Join(destination, "keep")
	if err := os.WriteFile(existing, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ExtractPublishedCoreArchive(context.Background(), archive, "zip", destination, nil), ErrPublishedCoreArchive) {
		t.Fatal("nonempty destination accepted")
	}
	if data, err := os.ReadFile(existing); err != nil || string(data) != "retained" {
		t.Fatal("existing staging data changed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	empty := t.TempDir()
	if !errors.Is(ExtractPublishedCoreArchive(ctx, archive, "zip", empty, nil), ErrPublishedCoreArchive) {
		t.Fatal("canceled extraction accepted")
	}
}
