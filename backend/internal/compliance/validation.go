package compliance

import (
	"regexp"
	"strings"
)

var fqdnRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// ValidateFQDN checks whether the given string is a valid fully qualified domain name.
func ValidateFQDN(domain string) bool {
	return fqdnRegex.MatchString(domain)
}

// ValidateDomains splits domains and returns valid and invalid lists.
func ValidateDomains(domains []string) (valid, invalid []string) {
	for _, d := range domains {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if ValidateFQDN(d) {
			valid = append(valid, d)
		} else {
			invalid = append(invalid, d)
		}
	}
	return
}
