// See the LICENSE file for license details.

package util

import (
	"errors"
	"strings"
)

// -----------------------------------------------------------------------------

// SanitizeUrlPath normalizes a URL path. If trailingSlash < 0, it removes the ending slash, if any;
// if trailingSlash > 0, it always adds one; if trailingSlash == 0, it keeps the original form.
func SanitizeUrlPath(path string, trailingSlash int) (string, error) {
	// Nothing to sanitize?
	if len(path) == 0 {
		return "/", nil
	}

	// Convert backslashes.
	path = strings.ReplaceAll(path, "\\", "/")

	// Check whether a trailing slash is present.
	endsWithSlash := strings.HasSuffix(path, "/")

	// Split the path into fragments and remove empty ones.
	newPathFragments := make([]string, 0)
	pathFragments := strings.Split(path, "/")
	for _, frag := range pathFragments {
		if len(frag) > 0 && frag != "." {
			if frag != ".." {
				newPathFragments = append(newPathFragments, frag)
			} else {
				if len(newPathFragments) > 0 {
					newPathFragments = newPathFragments[0 : len(newPathFragments)-1]
				} else {
					return "", errors.New("invalid path")
				}
			}
		}
	}

	// Join the path again.
	path = "/" + strings.Join(newPathFragments, "/")

	// Add a trailing slash if required.
	if len(newPathFragments) > 0 {
		if trailingSlash > 0 || (trailingSlash == 0 && endsWithSlash) {
			path += "/"
		}
	}

	// Done.
	return path, nil
}
