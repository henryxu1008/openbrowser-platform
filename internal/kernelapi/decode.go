package kernelapi

import (
	"bytes"
	"encoding/json"
	"io"
	"unicode/utf8"
)

var summaryKeys = []string{"releaseId", "version", "os", "arch", "minClientVersion", "maxClientVersionExclusive", "recommended"}

// Objects are small fixed contracts. Reject duplicates, missing/unknown fields,
// null scalars and trailing JSON rather than accepting ambiguous metadata.
func object(data []byte, keys []string) (map[string]json.RawMessage, error) {
	if !utf8.Valid(data) {
		return nil, ErrUnavailable
	}
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, ErrUnavailable
	}
	allowed := make(map[string]bool, len(keys))
	for _, key := range keys {
		allowed[key] = true
	}
	fields := make(map[string]json.RawMessage, len(keys))
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || !allowed[key] || fields[key] != nil {
			return nil, ErrUnavailable
		}
		var raw json.RawMessage
		if d.Decode(&raw) != nil {
			return nil, ErrUnavailable
		}
		fields[key] = raw
	}
	token, err = d.Token()
	if err != nil || token != json.Delim('}') || len(fields) != len(keys) || d.Decode(new(any)) != io.EOF {
		return nil, ErrUnavailable
	}
	return fields, nil
}

func textField(fields map[string]json.RawMessage, key string) (string, error) {
	var value *string
	if json.Unmarshal(fields[key], &value) != nil || value == nil {
		return "", ErrUnavailable
	}
	return *value, nil
}

func release(fields map[string]json.RawMessage) (Release, error) {
	var r Release
	values := []*string{&r.ReleaseID, &r.Version, &r.OS, &r.Arch, &r.MinClientVersion, &r.MaxClientVersionExclusive}
	for index, key := range summaryKeys[:6] {
		value, err := textField(fields, key)
		if err != nil {
			return Release{}, err
		}
		*values[index] = value
	}
	var recommended *bool
	if json.Unmarshal(fields["recommended"], &recommended) != nil || recommended == nil {
		return Release{}, ErrUnavailable
	}
	r.Recommended = *recommended
	return r, nil
}

func decodePage(data []byte) ([]Release, string, error) {
	fields, err := object(data, []string{"releases", "nextCursor"})
	if err != nil {
		return nil, "", err
	}
	var entries []json.RawMessage
	if bytes.Equal(bytes.TrimSpace(fields["releases"]), []byte("null")) || json.Unmarshal(fields["releases"], &entries) != nil || len(entries) > 100 {
		return nil, "", ErrUnavailable
	}
	next, err := textField(fields, "nextCursor")
	if err != nil || next != "" && !ValidReleaseID(next) {
		return nil, "", ErrUnavailable
	}
	result := make([]Release, 0, len(entries))
	for _, entry := range entries {
		fields, err := object(entry, summaryKeys)
		if err != nil {
			return nil, "", err
		}
		r, err := release(fields)
		if err != nil {
			return nil, "", err
		}
		result = append(result, r)
	}
	return result, next, nil
}

func decodeArtifact(data []byte) (Artifact, error) {
	fields, err := object(data, append(append([]string{}, summaryKeys...), "archiveFormat", "size", "sha256", "downloadUrl", "downloadOrigins"))
	if err != nil {
		return Artifact{}, err
	}
	r, err := release(fields)
	if err != nil {
		return Artifact{}, err
	}
	a := Artifact{Release: r}
	for key, target := range map[string]*string{"archiveFormat": &a.ArchiveFormat, "sha256": &a.SHA256, "downloadUrl": &a.DownloadURL} {
		*target, err = textField(fields, key)
		if err != nil {
			return Artifact{}, err
		}
	}
	var size *int64
	if json.Unmarshal(fields["size"], &size) != nil || size == nil || *size <= 0 || *size > MaxArtifactBytes {
		return Artifact{}, ErrUnavailable
	}
	a.Size = *size
	if !a.ValidPayload() {
		return Artifact{}, ErrUnavailable
	}
	if json.Unmarshal(fields["downloadOrigins"], &a.DownloadOrigins) != nil || !AllowsDownloadURL(a.DownloadURL, a.DownloadOrigins, false) {
		return Artifact{}, ErrUnavailable
	}
	return a, nil
}
