package publish

import _ "embed"

//go:embed runtime-manifest.json
var runtimeManifest []byte

// RuntimeManifest is the existing release owner's pinned runtime source.
func RuntimeManifest() []byte { return append([]byte(nil), runtimeManifest...) }
