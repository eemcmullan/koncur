package targets

import (
	"strings"
)

// ParseLabelSelector parses a label selector string into included and excluded labels.
// The label selector format supports:
// - OR operations with "||"
// - Negation with "!" prefix for exclusions
// - Key-value pairs in format "key=value"
//
// Examples:
//   - "konveyor.io/target=cloud-readiness || konveyor.io/target=linux" -> Included: ["konveyor.io/target=cloud-readiness", "konveyor.io/target=linux"]
//   - "!konveyor.io/target=windows" -> Excluded: ["konveyor.io/target=windows"]
//   - "konveyor.io/target=quarkus || !konveyor.io/source=java8" -> Included: ["konveyor.io/target=quarkus"], Excluded: ["konveyor.io/source=java8"]
func ParseLabelSelector(selector string) Labels {
	labels := Labels{
		Included: []string{},
		Excluded: []string{},
	}

	if selector == "" {
		return labels
	}

	// Split by OR operator
	parts := strings.Split(selector, "||")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's an exclusion (starts with !)
		if strings.HasPrefix(part, "!") {
			// Remove the ! prefix and add to excluded
			excluded := strings.TrimPrefix(part, "!")
			excluded = strings.TrimSpace(excluded)
			if excluded != "" {
				labels.Excluded = append(labels.Excluded, excluded)
			}
		} else {
			// Add to included
			labels.Included = append(labels.Included, part)
		}
	}

	return labels
}
