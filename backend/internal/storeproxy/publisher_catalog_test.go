package storeproxy

import (
	"encoding/json"
	"strings"
	"testing"
)

func publisherFixture() string {
	return `{"schemaVersion":1,"targets":[{"id":"probe","label":"Publisher probe","url":"https://probe.example.invalid/proxy-check/v1","expectedStatus":[204]}]}`
}

func TestPublisherCatalogShipsEmptyAndImmutable(t *testing.T) {
	catalog, err := PublisherCatalog()
	if err != nil || catalog.Available() || len(catalog.Targets()) != 0 || len(catalog.Version()) != 64 {
		t.Fatal("invalid shipped fail-closed catalog", err)
	}
	configured, err := decodePublisherCatalog([]byte(publisherFixture()))
	if err != nil || !configured.Available() {
		t.Fatal(err)
	}
	metadata := configured.Targets()
	metadata[0].ExpectedStatus[0] = 299
	metadata[0].URL = "https://replacement.example.invalid/"
	resolved, ok := configured.Resolve("probe", configured.Version())
	if !ok || resolved.ExpectedStatus[0] != 204 || resolved.URL != "https://probe.example.invalid/proxy-check/v1" {
		t.Fatal("catalog metadata mutation changed native resolution")
	}
	if _, ok := configured.Resolve("probe", catalog.Version()); ok {
		t.Fatal("empty/catalog replacement version accepted")
	}
}

func TestPublisherCatalogRejectsMalformedAndAmbiguousReleaseInput(t *testing.T) {
	valid := publisherFixture()
	cases := []string{
		"", "null", "[]", "{}", `{"schemaVersion":1}`, `{"schemaVersion":1,"targets":null}`,
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":"1"`, 1),
		strings.Replace(valid, `"schemaVersion":1`, `"SchemaVersion":1`, 1),
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1),
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":1,"schema\u0056ersion":1`, 1),
		strings.Replace(valid, `"targets":`, `"targets":[],"targets":`, 1),
		strings.Replace(valid, `"id":"probe"`, `"id":"probe","id":"second"`, 1),
		strings.Replace(valid, `"id":"probe"`, `"ID":"probe"`, 1),
		strings.Replace(valid, `"id":"probe"`, `"id":"probe","headers":{}`, 1),
		strings.Replace(valid, `"label":"Publisher probe"`, `"label":null`, 1),
		strings.Replace(valid, `"expectedStatus":[204]`, `"expectedStatus":[302]`, 1),
		strings.Replace(valid, `"expectedStatus":[204]`, `"expectedStatus":[204.1]`, 1),
		strings.Replace(valid, `"expectedStatus":[204]`, `"expectedStatus":null`, 1),
		strings.Replace(valid, "https://probe.example.invalid/", "http://probe.example.invalid/", 1),
		strings.Replace(valid, "https://probe.example.invalid/", "https://user:fixture-secret@probe.example.invalid/", 1),
		strings.Replace(valid, "https://probe.example.invalid/", "https://127.0.0.1/", 1),
		strings.Replace(valid, "/proxy-check/v1", "/proxy-check/v1?token=fixture-secret", 1),
		valid + " {}", valid + strings.Repeat(" ", 32*1024),
		strings.Replace(valid, "Publisher probe", string([]byte{0xff}), 1),
		`{"schemaVersion":1,"targets":[[[[[[[[[[]]]]]]]]]]}`,
	}
	var manifest map[string]any
	_ = json.Unmarshal([]byte(valid), &manifest)
	entries := manifest["targets"].([]any)
	manifest["targets"] = append(entries, entries[0])
	duplicateID, _ := json.Marshal(manifest)
	cases = append(cases, string(duplicateID))
	for index, raw := range cases {
		catalog, err := decodePublisherCatalog([]byte(raw))
		if err != ErrCatalog || catalog.Available() || catalog.Version() != "" {
			t.Errorf("case %d accepted invalid release input", index)
		}
	}
}
