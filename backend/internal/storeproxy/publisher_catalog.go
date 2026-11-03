package storeproxy

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"unicode/utf8"
)

// This is a code/release-owned source, not installed user settings. There is no
// path, URL, environment or renderer override. Changing it requires a new build.
//
//go:embed publisher_targets.json
var publisherTargets []byte

func PublisherCatalog() (Catalog, error) { return decodePublisherCatalog(publisherTargets) }

func decodePublisherCatalog(data []byte) (Catalog, error) {
	if len(data) > 32*1024 || !utf8.Valid(data) || !uniqueJSONKeys(data) {
		return Catalog{}, ErrCatalog
	}
	root, ok := exactObject(data, "schemaVersion", "targets")
	if !ok {
		return Catalog{}, ErrCatalog
	}
	var schema int
	var entries []json.RawMessage
	if json.Unmarshal(root["schemaVersion"], &schema) != nil || schema != 1 ||
		!bytes.HasPrefix(bytes.TrimSpace(root["targets"]), []byte("[")) ||
		json.Unmarshal(root["targets"], &entries) != nil || len(entries) > 8 {
		return Catalog{}, ErrCatalog
	}
	targets := make([]Target, 0, len(entries))
	for _, entry := range entries {
		fields, ok := exactObject(entry, "id", "label", "url", "expectedStatus")
		if !ok {
			return Catalog{}, ErrCatalog
		}
		var target Target
		if json.Unmarshal(fields["id"], &target.ID) != nil || json.Unmarshal(fields["label"], &target.Label) != nil ||
			json.Unmarshal(fields["url"], &target.URL) != nil || json.Unmarshal(fields["expectedStatus"], &target.ExpectedStatus) != nil {
			return Catalog{}, ErrCatalog
		}
		targets = append(targets, target)
	}
	return NewCatalog(targets)
}

func exactObject(data []byte, keys ...string) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || len(fields) != len(keys) {
		return nil, false
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return nil, false
		}
	}
	return fields, true
}

// encoding/json normally accepts duplicate and case-insensitive struct fields.
// Reject duplicates before decoding exact, case-sensitive field maps, and bound
// nesting/work before allocating decoded objects. Errors never echo the source.
func uniqueJSONKeys(data []byte) bool {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	nodes := 0
	var value func(int) bool
	value = func(depth int) bool {
		nodes++
		if depth > 6 || nodes > 256 {
			return false
		}
		token, err := d.Token()
		if err != nil {
			return false
		}
		delim, compound := token.(json.Delim)
		if !compound {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				token, err = d.Token()
				key, ok := token.(string)
				if err != nil || !ok || seen[key] {
					return false
				}
				seen[key] = true
				if !value(depth + 1) {
					return false
				}
			}
		case '[':
			for d.More() {
				if !value(depth + 1) {
					return false
				}
			}
		default:
			return false
		}
		end, err := d.Token()
		return err == nil && (delim == '{' && end == json.Delim('}') || delim == '[' && end == json.Delim(']'))
	}
	if !value(0) {
		return false
	}
	_, err := d.Token()
	return err == io.EOF
}
