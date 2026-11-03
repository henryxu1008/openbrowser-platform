package kernelcache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"ant-chrome/backend/internal/browser"
	"ant-chrome/internal/desktopauth"
)

const resumeTag = `"synthetic-v1"`

type resumeBody struct {
	io.Reader
	closed  bool
	onClose func() error
}

func (b *resumeBody) Close() error {
	b.closed = true
	if b.onClose != nil {
		return b.onClose()
	}
	return nil
}

type resumeNetworkError struct{}

func (resumeNetworkError) Error() string   { return "synthetic read failure; never expose token" }
func (resumeNetworkError) Timeout() bool   { return false }
func (resumeNetworkError) Temporary() bool { return true }

func resumeResponse(r *http.Request, status int, reader io.Reader, length int64, contentRange string) *http.Response {
	h := make(http.Header)
	h.Set("ETag", resumeTag)
	if contentRange != "" {
		h.Set("Content-Range", contentRange)
	}
	return &http.Response{StatusCode: status, Header: h, Body: &resumeBody{Reader: reader}, ContentLength: length, Request: r}
}

func noResumeInstallation(t *testing.T, m *Manager) {
	t.Helper()
	if len(m.ReadyCores()) != 0 {
		t.Fatal("incomplete transfer became an installation")
	}
	entries, _ := os.ReadDir(m.root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".prepare-") {
			t.Fatal("operation-owned stage survived failure")
		}
	}
}

func TestPrepareResumesPartialBodyWithinOneOperation(t *testing.T) {
	for _, kind := range []string{"unexpected-eof", "clean-eof", "network-read", "unknown-length"} {
		t.Run(kind, func(t *testing.T) {
			m, source, _, _, data := fixture(t)
			cut, calls := len(data)/3, 0
			var first *resumeBody
			var deadline time.Time
			readErr := error(io.ErrUnexpectedEOF)
			if kind == "clean-eof" {
				readErr = io.EOF
			} else if kind == "network-read" {
				readErr = resumeNetworkError{}
			}
			m.httpClient = func() (*http.Client, error) {
				return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Accept-Encoding") != "identity" {
						t.Fatal("artifact transport changed credential or encoding policy")
					}
					current, ok := r.Context().Deadline()
					if !ok || time.Until(current) > 15*time.Minute {
						t.Fatal("missing bounded transfer deadline")
					}
					if calls == 1 {
						deadline = current
						response := resumeResponse(r, 200, io.MultiReader(bytes.NewReader(data[:cut]), iotest.ErrReader(readErr)), int64(len(data)), "")
						if kind == "unknown-length" {
							response.ContentLength = -1
						}
						first = response.Body.(*resumeBody)
						return response, nil
					}
					if calls != 2 || !first.closed || !current.Equal(deadline) || r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", cut) || r.Header.Get("If-Range") != resumeTag {
						t.Fatal("recovery overlapped a body, reset its deadline, or requested a wrong range")
					}
					response := resumeResponse(r, 206, bytes.NewReader(data[cut:]), int64(len(data)-cut), fmt.Sprintf("bytes %d-%d/%d", cut, len(data)-1, len(data)))
					if kind == "unknown-length" {
						response.ContentLength = -1
					}
					return response, nil
				})}, nil
			}
			previous := -1
			core, err := m.Prepare(context.Background(), 1, source.artifact.ReleaseID, func(phase string, percent int) {
				if phase == "downloading" {
					if percent < previous || percent < 0 || percent > 100 {
						t.Fatal("recovery reset or overflowed progress")
					}
					previous = percent
				}
			})
			if err != nil || calls != 2 || len(m.ReadyCores()) != 1 {
				t.Fatalf("bounded recovery failed: %v / calls=%d", err, calls)
			}
			receipt, err := os.ReadFile(filepath.Join(core.CorePath, receiptName))
			if err != nil || strings.Contains(string(receipt), resumeTag) || strings.Contains(string(receipt), "cdn.example") {
				t.Fatal("transport metadata escaped into the installation receipt")
			}
		})
	}
}

func TestPrepareRejectsUnverifiableResume(t *testing.T) {
	cases := []string{"no-tag", "weak-tag", "invalid-tag", "duplicate-tag", "changed-tag", "missing-resume-tag", "duplicate-resume-tag", "ignored-range", "forbidden", "unsatisfiable", "missing-range", "wrong-start", "wrong-end", "wrong-total", "duplicate-range", "wrong-length", "encoded", "duplicate-encoding", "uncompressed", "second-short", "too-long", "corrupt-bytes", "arbitrary-read-error", "body-close-error"}
	for _, kind := range cases {
		t.Run(kind, func(t *testing.T) {
			m, source, _, _, data := fixture(t)
			cut, calls := len(data)/3, 0
			m.httpClient = func() (*http.Client, error) {
				return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					if calls > 2 {
						t.Fatal("recovery exceeded one continuation")
					}
					if calls == 1 {
						err := error(io.ErrUnexpectedEOF)
						if kind == "arbitrary-read-error" {
							err = errors.New("synthetic-token must remain private")
						}
						response := resumeResponse(r, 200, io.MultiReader(bytes.NewReader(data[:cut]), iotest.ErrReader(err)), int64(len(data)), "")
						switch kind {
						case "no-tag":
							response.Header.Del("ETag")
						case "weak-tag":
							response.Header.Set("ETag", `W/"synthetic-v1"`)
						case "invalid-tag":
							response.Header.Set("ETag", "unquoted")
						case "duplicate-tag":
							response.Header.Add("ETag", resumeTag)
						case "body-close-error":
							response.Body.(*resumeBody).onClose = func() error { return errors.New("synthetic close error") }
						}
						return response, nil
					}
					response := resumeResponse(r, 206, bytes.NewReader(data[cut:]), int64(len(data)-cut), fmt.Sprintf("bytes %d-%d/%d", cut, len(data)-1, len(data)))
					switch kind {
					case "changed-tag":
						response.Header.Set("ETag", `"other"`)
					case "missing-resume-tag":
						response.Header.Del("ETag")
					case "duplicate-resume-tag":
						response.Header.Add("ETag", resumeTag)
					case "ignored-range":
						response.StatusCode = 200
					case "forbidden":
						response.StatusCode = 403
					case "unsatisfiable":
						response.StatusCode = 416
					case "missing-range":
						response.Header.Del("Content-Range")
					case "wrong-start":
						response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", cut+1, len(data)-1, len(data)))
					case "wrong-end":
						response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", cut, len(data)-2, len(data)))
					case "wrong-total":
						response.Header.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", cut, len(data)-1, len(data)+1))
					case "duplicate-range":
						response.Header.Add("Content-Range", response.Header.Get("Content-Range"))
					case "wrong-length":
						response.ContentLength--
					case "encoded":
						response.Header.Set("Content-Encoding", "gzip")
					case "duplicate-encoding":
						response.Header.Add("Content-Encoding", "identity")
						response.Header.Add("Content-Encoding", "gzip")
					case "uncompressed":
						response.Uncompressed = true
					case "second-short":
						response.Body = &resumeBody{Reader: bytes.NewReader(data[cut : len(data)-1])}
					case "too-long":
						response.ContentLength = -1
						response.Body = &resumeBody{Reader: io.MultiReader(bytes.NewReader(data[cut:]), strings.NewReader("extra"))}
					case "corrupt-bytes":
						corrupt := append([]byte{}, data[cut:]...)
						corrupt[0] ^= 0xff
						response.Body = &resumeBody{Reader: bytes.NewReader(corrupt)}
					}
					return response, nil
				})}, nil
			}
			_, err := m.Prepare(context.Background(), 1, source.artifact.ReleaseID, nil)
			if err == nil || strings.Contains(err.Error(), "synthetic-token") {
				t.Fatalf("invalid stream accepted or raw error leaked: %v", err)
			}
			expected := 2
			if kind == "no-tag" || kind == "weak-tag" || kind == "invalid-tag" || kind == "duplicate-tag" || kind == "arbitrary-read-error" || kind == "body-close-error" {
				expected = 1
			}
			if calls != expected {
				t.Fatalf("unexpected request budget: %d, want %d", calls, expected)
			}
			if kind == "corrupt-bytes" && !errors.Is(err, browser.ErrPublishedCoreIntegrity) {
				t.Fatalf("whole-file integrity check was bypassed: %v", err)
			}
			noResumeInstallation(t, m)
		})
	}
}

func TestPrepareDoesNotResumeAfterCancellationOrRevocation(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(fmt.Sprint(revoke), func(t *testing.T) {
			m, source, authority, _, data := fixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			m.httpClient = func() (*http.Client, error) {
				return &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					response := resumeResponse(r, 200, bytes.NewReader(data[:len(data)/3]), int64(len(data)), "")
					response.Body.(*resumeBody).onClose = func() error {
						if revoke {
							authority.revoke()
						} else {
							cancel()
						}
						return nil
					}
					return response, nil
				})}, nil
			}
			_, err := m.Prepare(ctx, 1, source.artifact.ReleaseID, nil)
			if err == nil || calls != 1 || revoke && !errors.Is(err, desktopauth.ErrUnauthenticated) {
				t.Fatalf("revoked operation continued: %v / %d", err, calls)
			}
			noResumeInstallation(t, m)
		})
	}
}

func TestResumePinsValidatedFinalURLAndRejectsFurtherRedirect(t *testing.T) {
	for _, redirect := range []bool{false, true} {
		t.Run(fmt.Sprint(redirect), func(t *testing.T) {
			m, source, _, _, data := fixture(t)
			source.artifact.DownloadOrigins = append(source.artifact.DownloadOrigins, "https://assets.example.test")
			const finalURL = "https://assets.example.test/core.zip?signature=synthetic-signature"
			final, _ := url.Parse(finalURL)
			jar, _ := cookiejar.New(nil)
			jar.SetCookies(final, []*http.Cookie{{Name: "synthetic-cookie", Value: "must-not-leave"}})
			deadline := time.Now().Add(time.Minute)
			ctx, cancel := context.WithDeadline(context.Background(), deadline)
			defer cancel()
			cut, calls := len(data)/3, 0
			m.httpClient = func() (*http.Client, error) {
				return &http.Client{Jar: jar, Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					actual, ok := r.Context().Deadline()
					if !ok || !actual.Equal(deadline) || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != "" {
						t.Fatal("short caller deadline or credential isolation changed")
					}
					switch calls {
					case 1:
						response := resumeResponse(r, 302, strings.NewReader(""), 0, "")
						response.Header.Set("Location", finalURL)
						return response, nil
					case 2:
						if r.URL.String() != finalURL || r.Header.Get("Range") != "" {
							t.Fatal("initial redirect did not reach the approved final resource")
						}
						return resumeResponse(r, 200, bytes.NewReader(data[:cut]), int64(len(data)), ""), nil
					case 3:
						if r.URL.String() != finalURL || r.Header.Get("Range") != fmt.Sprintf("bytes=%d-", cut) || r.Header.Get("If-Range") != resumeTag {
							t.Fatal("continuation replayed the initial redirect or lost its resource validator")
						}
						if redirect {
							response := resumeResponse(r, 302, strings.NewReader(""), 0, "")
							response.Header.Set("Location", "https://assets.example.test/another-resource")
							return response, nil
						}
						return resumeResponse(r, 206, bytes.NewReader(data[cut:]), int64(len(data)-cut), fmt.Sprintf("bytes %d-%d/%d", cut, len(data)-1, len(data))), nil
					default:
						t.Fatal("continuation followed another redirect")
						return nil, ErrPreparation
					}
				})}, nil
			}
			core, err := m.Prepare(ctx, 1, source.artifact.ReleaseID, nil)
			if calls != 3 || redirect && err == nil || !redirect && err != nil {
				t.Fatalf("unexpected redirected recovery: %v / %d", err, calls)
			}
			if redirect {
				noResumeInstallation(t, m)
			} else {
				receipt, readErr := os.ReadFile(filepath.Join(core.CorePath, receiptName))
				if readErr != nil || strings.Contains(string(receipt), "synthetic-signature") || strings.Contains(string(receipt), "assets.example") || strings.Contains(source.artifact.DownloadURL, "signature") {
					t.Fatal("signed target escaped operation memory")
				}
			}
		})
	}
}
