package validator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type kantra struct {
	testDir string
}

func (k *kantra) compareTag(expected string, actual []string) (*ValidationError, bool) {
	if findExpectedString(expected, actual) {
		return nil, false
	}
	// Didn't find expected tag
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected tag: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}

func (k *kantra) compareViolation(expected, actual konveyor.Violation) ([]ValidationError, bool) {
	validationError := []ValidationError{}
	if expected.Category != actual.Category {
		validationError = append(validationError, ValidationError{
			Path:     "",
			Message:  fmt.Sprintf("Did not find expected category: %v", expected.Category),
			Expected: expected,
			Actual:   nil,
		})
	}
	if expected.Effort != actual.Effort {
		validationError = append(validationError, ValidationError{
			Path:     "",
			Message:  fmt.Sprintf("Did not find expected effort: %v", expected.Effort),
			Expected: expected,
			Actual:   nil,
		})
	}
	// Handle Links
	for _, l := range expected.Links {
		found := false
		for _, al := range actual.Links {
			if l.Title == al.Title && l.URL == al.Title {
				found = true
				break
			}
		}
		if !found {
			validationError = append(validationError, ValidationError{
				Path:     "",
				Message:  fmt.Sprintf("Did not find expected links: %v", l),
				Expected: expected,
				Actual:   nil,
			})
		}
	}
	// Handle Labels
	for _, l := range expected.Labels {
		if findExpectedString(l, actual.Labels) {
			continue
		}
		validationError = append(validationError, ValidationError{
			Path:     "",
			Message:  fmt.Sprintf("Did not find expected label: %v", l),
			Expected: expected,
			Actual:   nil,
		})
	}
	// Handle Incidents - collect all missing incidents and report as one error
	var missingIncidents []konveyor.Incident
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			if strings.TrimSpace(i.CodeSnip) != strings.TrimSpace(ai.CodeSnip) {
				continue
			}
			// Skip URI comparison if either URI is empty
			if string(i.URI) == "" || string(ai.URI) == "" {
				if string(i.URI) != string(ai.URI) {
					continue
				}
			} else {
				pathToTest, err := filepath.Rel(filepath.Join(k.testDir, "source"), i.URI.Filename())
				if err != nil {
					break
				}
				if !strings.Contains(ai.URI.Filename(), pathToTest) {
					continue
				}
			}
			if i.Message != ai.Message {
				continue
			}
			if i.LineNumber != ai.LineNumber {
				continue
			}
			if !reflect.DeepEqual(i.Variables, ai.Variables) {
				continue
			}
			found = true
		}
		if !found {
			missingIncidents = append(missingIncidents, i)
		}
	}

	// If there are missing incidents, create a single consolidated error
	if len(missingIncidents) > 0 {
		// Build a summary message
		var uris []string
		for _, inc := range missingIncidents {
			if string(inc.URI) != "" {
				uris = append(uris, string(inc.URI))
			}
		}

		message := fmt.Sprintf("Missing %d incident(s)", len(missingIncidents))
		if len(uris) > 0 {
			message += fmt.Sprintf(" for files: %s", strings.Join(uris, ", "))
		}

		validationError = append(validationError, ValidationError{
			Path:     "",
			Message:  message,
			Expected: fmt.Sprintf("%d incidents", len(expected.Incidents)),
			Actual:   fmt.Sprintf("%d incidents (missing %d)", len(actual.Incidents), len(missingIncidents)),
		})
	}

	return validationError, len(validationError) != 0
}

func (k *kantra) compareErrors(expected, actual string) (*ValidationError, bool) {
	if expected != actual {
		return &ValidationError{
			Path:     "",
			Message:  fmt.Sprintf("Did not find expected error: %s", expected),
			Expected: expected,
			Actual:   nil,
		}, true
	}
	return nil, false
}

func (k *kantra) compareUnmatched(expected string, actual []string) (*ValidationError, bool) {
	if findExpectedString(expected, actual) {
		return nil, false
	}
	// Didn't find expected tag
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected unmatched rule: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}

func (k *kantra) compareSkipped(expected string, actual []string) (*ValidationError, bool) {
	if findExpectedString(expected, actual) {
		return nil, false
	}
	// Didn't find expected tag
	return &ValidationError{
		Path:     "",
		Message:  fmt.Sprintf("Did not find expected skipped rule: %s", expected),
		Expected: expected,
		Actual:   nil,
	}, true
}
