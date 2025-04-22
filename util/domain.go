// See the LICENSE file for license details.

package util

import (
	"strings"
)

// -----------------------------------------------------------------------------

// DoesSubdomainMatch checks if the given domain matches the given pattern.
func DoesSubdomainMatch(domain string, pattern string) bool {
	domainIdx := strings.Index(domain, "://")
	patternIdx := strings.Index(pattern, "://")
	if domainIdx < 0 || patternIdx < 0 {
		return false
	}
	if domain[:domainIdx] != pattern[:patternIdx] {
		return false
	}

	domain = domain[domainIdx+3:]
	if len(domain) > 253 {
		return false // Invalid domain length
	}
	pattern = pattern[patternIdx+3:]

	domainParts := strings.Split(domain, ".")
	patternParts := strings.Split(pattern, ".")
	if len(patternParts) == 0 {
		return false
	}

	patternIdx = len(patternParts)
	domainIdx = len(domainParts)
	for domainIdx > 0 {
		if patternIdx == 0 {
			return false
		}

		domainIdx -= 1
		patternIdx -= 1

		p := patternParts[patternIdx]
		if p == "*" {
			return true
		}
		if p != domainParts[domainIdx] {
			return false
		}
	}
	return patternIdx == 0
}
