// Package kernelcache prepares immutable local installations of platform releases.
package kernelcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/internal/apiconfig"
	"ant-chrome/internal/desktopauth"
	"ant-chrome/internal/kernelapi"
	"github.com/google/uuid"
)

var (
	ErrPreparation = errors.New("kernel_preparation_failed")
	ErrCache       = errors.New("kernel_cache_unavailable")
	ErrBusy        = errors.New("kernel_preparation_busy")
	ErrIdentity    = errors.New("kernel_release_identity_changed")
)

const receiptName = ".platform-kernel-installation.json"

type Source interface {
	Origin() string
	Catalog(context.Context, uint64) ([]kernelapi.Release, error)
	Artifact(context.Context, uint64, string) (kernelapi.Artifact, error)
}

type Authority interface {
	Check(uint64) error
	WithIdentity(uint64, func(desktopauth.Identity) error) error
}

type Progress func(phase string, percent int)

type Version struct {
	kernelapi.Release
	Installed bool `json:"installed"`
}

type installation struct {
	Schema          int               `json:"schema"`
	PublisherOrigin string            `json:"publisherOrigin"`
	Release         kernelapi.Release `json:"release"`
	Format          string            `json:"archiveFormat"`
	Size            int64             `json:"size"`
	SHA256          string            `json:"sha256"`
	Executable      string            `json:"executable"`
	ExecutableSize  int64             `json:"executableSize"`
	ExecutableSHA   string            `json:"executableSha256"`
}

func (i installation) artifact() kernelapi.Artifact {
	return kernelapi.Artifact{Release: i.Release, ArchiveFormat: i.Format, Size: i.Size, SHA256: i.SHA256}
}

type flight struct {
	generation uint64
	done       chan struct{}
	err        error
}

type Manager struct {
	root, origin, version string
	source                Source
	authority             Authority
	httpClient            func() (*http.Client, error)
	mu                    sync.Mutex
	flights               map[string]*flight
}

// Construction never creates directories or adopts a caller-selected core.
// A nil source keeps the capability explicitly unavailable, without a legacy fallback.
func New(root, version string, source Source, authority Authority, httpClient func() (*http.Client, error)) *Manager {
	m := &Manager{root: filepath.Clean(root), version: version, source: source, authority: authority, httpClient: httpClient, flights: map[string]*flight{}}
	if source != nil {
		m.origin = source.Origin()
	}
	return m
}

func (m *Manager) enabled() bool {
	return m != nil && m.source != nil && m.authority != nil && filepath.IsAbs(m.root) && (apiconfig.Config{APIOrigin: m.origin}).Valid()
}

func samePayload(a, b kernelapi.Artifact) bool {
	a.Release.Recommended, b.Release.Recommended = false, false
	return a.Release == b.Release && a.ArchiveFormat == b.ArchiveFormat && a.Size == b.Size && a.SHA256 == b.SHA256
}

func (m *Manager) Catalog(ctx context.Context, generation uint64) ([]Version, error) {
	if !m.enabled() {
		return nil, kernelapi.ErrConfiguration
	}
	if err := m.authority.Check(generation); err != nil {
		return nil, err
	}
	releases, err := m.source.Catalog(ctx, generation)
	if err != nil {
		return nil, publicError(err)
	}
	result := make([]Version, 0, len(releases))
	for _, release := range releases {
		_, installed := m.readyCore(release.ReleaseID, &release)
		result = append(result, Version{Release: release, Installed: installed})
	}
	if err := m.authority.Check(generation); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *Manager) ReadyCores() []browser.Core {
	result := []browser.Core{}
	if !m.enabled() || !m.safeDirectory(m.root) {
		return result
	}
	entries, err := os.ReadDir(m.root)
	if err != nil || len(entries) > 4096 {
		return result
	}
	for _, entry := range entries {
		if entry.IsDir() && kernelapi.ValidReleaseID(entry.Name()) {
			if core, ok := m.readyCore(entry.Name(), nil); ok {
				result = append(result, core)
			}
		}
	}
	return result
}

func (m *Manager) readInstall(dir string) (installation, bool) {
	var record installation
	if !m.safeDirectory(dir) {
		return record, false
	}
	file, err := os.Open(filepath.Join(dir, receiptName))
	if err != nil {
		return record, false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(data) > 16384 {
		return record, false
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if d.Decode(&record) != nil || d.Decode(new(any)) != io.EOF || record.Schema != 1 || record.PublisherOrigin != m.origin || !record.artifact().ValidPayload() || !record.Release.Compatible(runtime.GOOS, runtime.GOARCH, m.version) {
		return installation{}, false
	}
	return record, true
}

func (m *Manager) installations(releaseID string) []string {
	if !kernelapi.ValidReleaseID(releaseID) || !m.safeDirectory(m.root) {
		return nil
	}
	base := filepath.Join(m.root, releaseID)
	if !m.safeDirectory(base) {
		return nil
	}
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) > 1024 {
		return nil
	}
	result := []string{}
	for _, entry := range entries {
		if entry.IsDir() && kernelapi.ValidReleaseID(entry.Name()) {
			result = append(result, filepath.Join(base, entry.Name()))
		}
	}
	return result
}

func (m *Manager) readyCore(releaseID string, summary *kernelapi.Release) (browser.Core, bool) {
	for _, dir := range m.installations(releaseID) {
		record, ok := m.readInstall(dir)
		if !ok || record.Release.ReleaseID != releaseID {
			continue
		}
		if summary != nil {
			left, right := record.Release, *summary
			left.Recommended, right.Recommended = false, false
			if left != right {
				continue
			}
		}
		relative := filepath.Clean(filepath.FromSlash(record.Executable))
		if relative == "" || filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" || strings.Contains(relative, ":") || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			continue
		}
		file := filepath.Join(dir, relative)
		resolved, err := filepath.EvalSymlinks(file)
		if err != nil || !samePath(resolved, file) {
			continue
		}
		size, digest, err := executableDigest(file)
		if err == nil && size == record.ExecutableSize && digest == record.ExecutableSHA {
			return browser.Core{CoreId: releaseID, CoreName: record.Release.Version, CorePath: dir}, true
		}
	}
	return browser.Core{}, false
}

func (m *Manager) Prepare(ctx context.Context, generation uint64, releaseID string, progress Progress) (browser.Core, error) {
	if !m.enabled() {
		return browser.Core{}, kernelapi.ErrConfiguration
	}
	if ctx == nil || ctx.Err() != nil {
		return browser.Core{}, ErrPreparation
	}
	if err := m.authority.Check(generation); err != nil {
		return browser.Core{}, err
	}
	a, err := m.source.Artifact(ctx, generation, releaseID)
	if err != nil {
		return browser.Core{}, publicError(err)
	}
	if a.ReleaseID != releaseID || !a.Release.Compatible(runtime.GOOS, runtime.GOARCH, m.version) || !a.ValidPayload() || !kernelapi.AllowsDownloadURL(a.DownloadURL, a.DownloadOrigins, false) {
		return browser.Core{}, kernelapi.ErrUnavailable
	}
	a.DownloadOrigins = append([]string{}, a.DownloadOrigins...)
	m.mu.Lock()
	if running := m.flights[releaseID]; running != nil {
		m.mu.Unlock()
		if running.generation != generation {
			return browser.Core{}, ErrBusy
		}
		select {
		case <-ctx.Done():
			return browser.Core{}, ErrPreparation
		case <-running.done:
		}
		if err := m.authority.Check(generation); err != nil {
			return browser.Core{}, err
		}
		if running.err != nil {
			return browser.Core{}, running.err
		}
		if core, ok, err := m.preparedCore(a); err != nil {
			return browser.Core{}, err
		} else if ok {
			return core, nil
		}
		return browser.Core{}, ErrCache
	}
	current := &flight{generation: generation, done: make(chan struct{})}
	m.flights[releaseID] = current
	m.mu.Unlock()
	core, err := m.prepare(ctx, generation, a, progress)
	m.mu.Lock()
	current.err = publicError(err)
	delete(m.flights, releaseID)
	close(current.done)
	m.mu.Unlock()
	return core, publicError(err)
}

func (m *Manager) preparedCore(a kernelapi.Artifact) (browser.Core, bool, error) {
	for _, dir := range m.installations(a.ReleaseID) {
		if record, ok := m.readInstall(dir); ok && !samePayload(record.artifact(), a) {
			return browser.Core{}, false, ErrIdentity
		}
	}
	if core, ok := m.readyCore(a.ReleaseID, &a.Release); ok {
		return core, true, nil
	}
	return browser.Core{}, false, nil
}

// Saving may bind only an already prepared release. New selections also
// recheck current publication, without downloading or changing a profile.
func (m *Manager) RequireSelection(ctx context.Context, generation uint64, id string, unchanged bool) error {
	if !m.enabled() {
		return kernelapi.ErrConfiguration
	}
	if err := m.authority.Check(generation); err != nil {
		return err
	}
	if !kernelapi.ValidReleaseID(id) {
		return ErrPreparation
	}
	if unchanged {
		if _, ready := m.readyCore(id, nil); !ready {
			return ErrCache
		}
	} else {
		a, err := m.source.Artifact(ctx, generation, id)
		if err != nil {
			return publicError(err)
		}
		if a.ReleaseID != id || !a.Release.Compatible(runtime.GOOS, runtime.GOARCH, m.version) || !a.ValidPayload() {
			return kernelapi.ErrUnavailable
		}
		if _, ready, err := m.preparedCore(a); err != nil {
			return err
		} else if !ready {
			return ErrCache
		}
	}
	return m.authority.Check(generation)
}

func (m *Manager) prepare(ctx context.Context, generation uint64, a kernelapi.Artifact, progress Progress) (browser.Core, error) {
	if core, ready, err := m.preparedCore(a); err != nil {
		return browser.Core{}, err
	} else if ready {
		return core, nil
	}
	var stage string
	err := m.authority.WithIdentity(generation, func(desktopauth.Identity) error {
		if !safeAncestors(m.root) {
			return ErrCache
		}
		if err := os.MkdirAll(m.root, 0700); err != nil || !m.safeDirectory(m.root) {
			return ErrCache
		}
		var err error
		stage, err = os.MkdirTemp(m.root, ".prepare-")
		if err != nil {
			return ErrCache
		}
		return nil
	})
	if err != nil {
		return browser.Core{}, publicError(err)
	}
	defer m.cleanStage(stage)
	emit := func(phase string, percent int) {
		if progress != nil && ctx.Err() == nil && m.authority.Check(generation) == nil {
			progress(phase, percent)
		}
	}
	archive := filepath.Join(stage, "artifact."+a.ArchiveFormat)
	if err := m.download(ctx, generation, a, archive, emit); err != nil {
		return browser.Core{}, err
	}
	emit("verifying", 100)
	if err := browser.VerifyPublishedCoreArchive(archive, a.Size, a.SHA256); err != nil {
		return browser.Core{}, err
	}
	payload := filepath.Join(stage, "payload")
	if err := os.Mkdir(payload, 0700); err != nil {
		return browser.Core{}, ErrCache
	}
	if err := browser.ExtractPublishedCoreArchive(ctx, archive, a.ArchiveFormat, payload, func(percent int, _ string) { emit("extracting", percent) }); err != nil {
		return browser.Core{}, err
	}
	executable, _, found := browser.FindCoreExecutable(payload)
	if !found {
		return browser.Core{}, ErrPreparation
	}
	size, digest, err := executableDigest(executable)
	if err != nil {
		return browser.Core{}, err
	}
	relative, err := filepath.Rel(payload, executable)
	if err != nil {
		return browser.Core{}, ErrCache
	}
	record := installation{Schema: 1, PublisherOrigin: m.origin, Release: a.Release, Format: a.ArchiveFormat, Size: a.Size, SHA256: a.SHA256, Executable: filepath.ToSlash(relative), ExecutableSize: size, ExecutableSHA: digest}
	data, err := json.Marshal(record)
	if err != nil {
		return browser.Core{}, ErrCache
	}
	file, err := os.OpenFile(filepath.Join(payload, receiptName), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return browser.Core{}, ErrCache
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		return browser.Core{}, ErrCache
	}
	base := filepath.Join(m.root, a.ReleaseID)
	final := filepath.Join(base, uuid.NewString())
	err = m.authority.WithIdentity(generation, func(desktopauth.Identity) error {
		if ctx.Err() != nil || !m.safeDirectory(m.root) {
			return ErrPreparation
		}
		if err := os.MkdirAll(base, 0700); err != nil || !m.safeDirectory(base) {
			return ErrCache
		}
		if err := os.Rename(payload, final); err != nil {
			return ErrCache
		}
		return nil
	})
	if err != nil {
		return browser.Core{}, err
	}
	emit("ready", 100)
	return browser.Core{CoreId: a.ReleaseID, CoreName: a.Version, CorePath: final}, nil
}

func executableDigest(filePath string) (int64, string, error) {
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > kernelapi.MaxArtifactBytes {
		return 0, "", ErrCache
	}
	file, err := os.Open(filePath)
	if err != nil {
		return 0, "", ErrCache
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, io.LimitReader(file, info.Size()+1))
	if err != nil || size != info.Size() {
		return 0, "", ErrCache
	}
	return size, hex.EncodeToString(hash.Sum(nil)), nil
}

func samePath(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (m *Manager) safeDirectory(dir string) bool {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	resolved, err := filepath.EvalSymlinks(dir)
	return err == nil && samePath(resolved, dir)
}

func safeAncestors(target string) bool {
	for cursor := filepath.Clean(target); ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err == nil {
			resolved, resolveErr := filepath.EvalSymlinks(cursor)
			return info.IsDir() && info.Mode()&os.ModeSymlink == 0 && resolveErr == nil && samePath(resolved, cursor)
		}
		if !os.IsNotExist(err) || filepath.Dir(cursor) == cursor {
			return false
		}
	}
}

func (m *Manager) cleanStage(stage string) {
	if filepath.Dir(stage) != m.root || !strings.HasPrefix(filepath.Base(stage), ".prepare-") || !m.safeDirectory(stage) {
		return
	}
	_ = os.RemoveAll(stage)
}

func publicError(err error) error {
	if err == nil {
		return nil
	}
	for _, allowed := range []error{desktopauth.ErrUnauthenticated, desktopauth.ErrCancelled, kernelapi.ErrConfiguration, kernelapi.ErrPermission, kernelapi.ErrNotFound, kernelapi.ErrUnavailable, ErrPreparation, ErrCache, ErrBusy, ErrIdentity, browser.ErrPublishedCoreArchive, browser.ErrPublishedCoreIntegrity} {
		if errors.Is(err, allowed) {
			return allowed
		}
	}
	return ErrPreparation
}
