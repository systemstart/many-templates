package resolve

import (
	"fmt"
	"strings"
)

// Resolve takes a URI string and returns a local filesystem path.
// Supported schemes: file:// (or bare path), oci://, https://, ocm://.
// For now, only file:// and bare paths are implemented; others return an error.
// The returned cleanup function should be called (if non-nil) when the path is
// no longer needed — for file:// it is always nil.
func Resolve(uri string) (path string, cleanup func(), err error) {
	switch {
	case strings.HasPrefix(uri, "file://"):
		return strings.TrimPrefix(uri, "file://"), nil, nil

	case strings.HasPrefix(uri, "oci://"):
		return resolveOCI(strings.TrimPrefix(uri, "oci://"))

	case strings.HasPrefix(uri, "https://"):
		return resolveHTTPS(uri) // keep full URL for net/http

	case strings.HasPrefix(uri, "ocm://"):
		return resolveOCM(strings.TrimPrefix(uri, "ocm://"))

	case schemePrefix(uri) != "":
		return "", nil, fmt.Errorf("unsupported scheme: %s", schemePrefix(uri))

	default:
		// Bare path — treat as implicit file://.
		return uri, nil, nil
	}
}

// IsRemote returns true if the URI has a recognized remote scheme (oci://, https://, ocm://).
func IsRemote(uri string) bool {
	return schemePrefix(uri) != ""
}

// schemePrefix returns the scheme portion (e.g. "ftp://") if uri looks like it
// contains a scheme, or "" if it appears to be a bare path.
func schemePrefix(uri string) string {
	idx := strings.Index(uri, "://")
	if idx > 0 {
		return uri[:idx+3]
	}
	return ""
}
