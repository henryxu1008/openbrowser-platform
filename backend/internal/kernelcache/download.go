package kernelcache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"ant-chrome/internal/kernelapi"
)

type downloadReader struct {
	context    context.Context
	check      func() error
	body       io.Reader
	read, size int64
	progress   Progress
	readErr    error
}

func (r *downloadReader) Read(p []byte) (int, error) {
	if err := r.context.Err(); err != nil {
		return 0, ErrPreparation
	}
	if err := r.check(); err != nil {
		return 0, err
	}
	n, err := r.body.Read(p)
	r.readErr = err
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
	if m.httpClient == nil || ctx == nil || ctx.Err() != nil {
		return ErrPreparation
	}
	client, err := m.httpClient()
	if err != nil || client == nil {
		return ErrPreparation
	}
	// A recovery continues this operation; it never receives a fresh timeout.
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	copy := *client
	copy.Jar = nil
	copy.Timeout = 15 * time.Minute
	targetURL, resuming := a.DownloadURL, false
	copy.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		// The validator belongs to the initial final resource. Never forward a
		// partial request/validator to another resource, even on an allowed host.
		if resuming || len(via) > kernelapi.MaxDownloadRedirects || !kernelapi.AllowsDownloadURL(request.URL.String(), a.DownloadOrigins, true) {
			return ErrPreparation
		}
		request.Header.Del("Authorization")
		request.Header.Del("Proxy-Authorization")
		request.Header.Del("Cookie")
		if err := m.authority.Check(generation); err != nil {
			return err
		}
		targetURL = request.URL.String()
		return nil
	}
	var file *os.File
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()
	var offset int64
	var validator string
	for attempt := 0; attempt < 2; attempt++ {
		if ctx.Err() != nil {
			return ErrPreparation
		}
		if err := m.authority.Check(generation); err != nil {
			return err
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			return ErrPreparation
		}
		request.Close = true
		request.Header.Set("Accept", "application/octet-stream")
		request.Header.Set("Accept-Encoding", "identity")
		if resuming {
			request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			request.Header.Set("If-Range", validator)
		}
		response, err := copy.Do(request)
		if err != nil {
			if checkErr := m.authority.Check(generation); checkErr != nil {
				return checkErr
			}
			return ErrPreparation
		}
		remaining := a.Size - offset
		encodings := response.Header.Values("Content-Encoding")
		identityEncoding := len(encodings) == 0 || len(encodings) == 1 && strings.EqualFold(strings.TrimSpace(encodings[0]), "identity")
		valid := !response.Uncompressed && identityEncoding && (response.ContentLength < 0 || response.ContentLength == remaining)
		if resuming {
			tag, ok := strongETag(response.Header)
			valid = valid && response.StatusCode == http.StatusPartialContent && ok && tag == validator && len(response.Header.Values("Content-Range")) == 1 && response.Header.Get("Content-Range") == fmt.Sprintf("bytes %d-%d/%d", offset, a.Size-1, a.Size)
		} else {
			valid = valid && response.StatusCode == http.StatusOK
			validator, _ = strongETag(response.Header)
		}
		if !valid {
			_ = response.Body.Close()
			return ErrPreparation
		}
		if file == nil {
			file, err = os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				_ = response.Body.Close()
				return ErrCache
			}
		}
		reader := &downloadReader{context: ctx, body: response.Body, read: offset, size: a.Size, check: func() error { return m.authority.Check(generation) }, progress: progress}
		written, writeErr := io.Copy(file, io.LimitReader(reader, remaining+1))
		offset += written
		bodyErr := response.Body.Close()
		syncErr := file.Sync()
		if ctx.Err() != nil {
			return ErrPreparation
		}
		if err := m.authority.Check(generation); err != nil {
			return err
		}
		if writeErr == nil && offset == a.Size && bodyErr == nil && syncErr == nil {
			closeErr := file.Close()
			file = nil
			if closeErr != nil {
				return ErrPreparation
			}
			return m.authority.Check(generation)
		}
		// Only a partial body read may recover. Disk/close errors, arbitrary
		// errors, cancellation, and a second interruption finish this attempt.
		readFailed := reader.readErr != nil && resumableRead(reader.readErr) && (writeErr == nil || errors.Is(writeErr, reader.readErr))
		if attempt == 0 && offset > 0 && offset < a.Size && reader.read == offset && readFailed && validator != "" && bodyErr == nil && syncErr == nil {
			resuming = true
			continue
		}
		if writeErr != nil {
			return publicError(writeErr)
		}
		return ErrPreparation
	}
	return ErrPreparation
}

// RFC 9110 8.8.3 and 15.3.7.3: combine only the same strong validator.
func strongETag(header http.Header) (string, bool) {
	values := header.Values("ETag")
	if len(values) != 1 {
		return "", false
	}
	tag := values[0]
	if len(tag) < 2 || tag[0] != '"' || tag[len(tag)-1] != '"' {
		return "", false
	}
	for i := 1; i < len(tag)-1; i++ {
		if tag[i] < 0x21 || tag[i] == '"' || tag[i] == 0x7f {
			return "", false
		}
	}
	return tag, true
}

func resumableRead(err error) bool {
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError)
}
