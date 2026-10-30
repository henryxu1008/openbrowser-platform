package kernelcache

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"ant-chrome/internal/kernelapi"
)

type downloadReader struct {
	context    context.Context
	check      func() error
	body       io.Reader
	read, size int64
	progress   Progress
}

func (r *downloadReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, ErrPreparation
	}
	if err := r.check(); err != nil {
		return 0, err
	}
	n, err := r.body.Read(p)
	r.read += int64(n)
	if r.progress != nil {
		percent := int(r.read * 100 / r.size)
		if percent > 100 {
			percent = 100
		}
		r.progress("downloading", percent)
	}
	return n, err
}

func (m *Manager) download(ctx context.Context, generation uint64, a kernelapi.Artifact, filePath string, progress Progress) error {
	if m.httpClient == nil {
		return ErrPreparation
	}
	client, err := m.httpClient()
	if err != nil || client == nil {
		return ErrPreparation
	}
	copy := *client
	copy.Jar = nil
	copy.Timeout = 15 * time.Minute
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) > kernelapi.MaxDownloadRedirects || !kernelapi.AllowsDownloadURL(request.URL.String(), a.DownloadOrigins, true) {
			return ErrPreparation
		}
		request.Header.Del("Authorization")
		request.Header.Del("Proxy-Authorization")
		request.Header.Del("Cookie")
		return m.authority.Check(generation)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.DownloadURL, nil)
	if err != nil {
		return ErrPreparation
	}
	request.Close = true
	request.Header.Set("Accept", "application/octet-stream")
	if err := m.authority.Check(generation); err != nil {
		return err
	}
	response, err := copy.Do(request)
	if err != nil {
		return ErrPreparation
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength >= 0 && response.ContentLength != a.Size {
		return ErrPreparation
	}
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrCache
	}
	reader := &downloadReader{context: ctx, body: response.Body, size: a.Size, check: func() error { return m.authority.Check(generation) }, progress: progress}
	written, writeErr := io.Copy(file, io.LimitReader(reader, a.Size+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil {
		return publicError(writeErr)
	}
	if written != a.Size || syncErr != nil || closeErr != nil {
		return ErrPreparation
	}
	return m.authority.Check(generation)
}
