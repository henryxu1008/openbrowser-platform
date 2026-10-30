package kernelapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"ant-chrome/internal/apiconfig"
	"ant-chrome/internal/desktopauth"
)

const maxResponseBytes = 1 << 20

type Client struct {
	origin, osName, arch, version string
	auth                          Auth
	http                          *http.Client
}

func (c *Client) Origin() string { return c.origin }

func New(config apiconfig.Config, auth Auth, clientVersion string) (*Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	return newClient(config, auth, clientVersion, runtime.GOOS, runtime.GOARCH, &http.Client{Transport: transport})
}

func newClient(config apiconfig.Config, auth Auth, version, osName, arch string, client *http.Client) (*Client, error) {
	if !config.Valid() || auth == nil || client == nil || !validClientVersion(version) || osName != "windows" && osName != "linux" && osName != "darwin" || arch != "amd64" && arch != "arm64" {
		return nil, ErrConfiguration
	}
	copy := *client
	copy.Timeout = 15 * time.Second
	copy.Jar = nil
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{origin: config.APIOrigin, auth: auth, version: version, osName: osName, arch: arch, http: &copy}, nil
}

func (c *Client) request(ctx context.Context, generation uint64, path, cursor string, catalog bool) ([]byte, error) {
	if err := c.auth.Check(generation); err != nil {
		return nil, err
	}
	query := url.Values{"os": {c.osName}, "arch": {c.arch}, "clientVersion": {c.version}}
	if catalog {
		query.Set("limit", "100")
		if cursor != "" {
			query.Set("cursor", cursor)
		}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.origin+"/api/v1/browser/kernel-releases"+path+"?"+query.Encode(), nil)
	if err != nil {
		return nil, ErrUnavailable
	}
	request.Header.Set("Accept", "application/json")
	if err := c.auth.AuthorizeRequest(generation, request); err != nil {
		return nil, err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if err := c.auth.Check(generation); err != nil {
		return nil, err
	}
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return nil, desktopauth.ErrUnauthenticated
	case http.StatusForbidden:
		return nil, ErrPermission
	case http.StatusNotFound:
		return nil, ErrNotFound
	default:
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(data) > maxResponseBytes {
		return nil, ErrUnavailable
	}
	if err := c.auth.Check(generation); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Client) Catalog(ctx context.Context, generation uint64) ([]Release, error) {
	result := []Release{}
	cursor, previous := "", ""
	for page := 0; page < 32; page++ {
		data, err := c.request(ctx, generation, "", cursor, true)
		if err != nil {
			return nil, err
		}
		releases, next, err := decodePage(data)
		if err != nil || next != "" && next <= cursor {
			return nil, ErrUnavailable
		}
		for _, release := range releases {
			if !release.Compatible(c.osName, c.arch, c.version) || release.ReleaseID <= cursor || release.ReleaseID <= previous || next != "" && release.ReleaseID > next {
				return nil, ErrUnavailable
			}
			previous = release.ReleaseID
			result = append(result, release)
			if len(result) > 1000 {
				return nil, ErrUnavailable
			}
		}
		if next == "" {
			if err := c.auth.Check(generation); err != nil {
				return nil, err
			}
			return result, nil
		}
		cursor = next
	}
	return nil, ErrUnavailable
}

func (c *Client) Artifact(ctx context.Context, generation uint64, releaseID string) (Artifact, error) {
	if !ValidReleaseID(releaseID) {
		return Artifact{}, ErrNotFound
	}
	data, err := c.request(ctx, generation, "/"+releaseID+"/artifact", "", false)
	if err != nil {
		return Artifact{}, err
	}
	artifact, err := decodeArtifact(data)
	if err != nil || artifact.ReleaseID != releaseID || !artifact.Release.Compatible(c.osName, c.arch, c.version) {
		return Artifact{}, ErrUnavailable
	}
	if err := c.auth.Check(generation); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}
