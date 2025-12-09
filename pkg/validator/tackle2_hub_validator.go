package validator

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type tackle2Hub struct {
	kantra
}

func (t *tackle2Hub) compareViolation(expected, actual konveyor.Violation) ([]ValidationError, bool) {
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
	// Handle Incidents
	for _, i := range expected.Incidents {
		found := false
		for _, ai := range actual.Incidents {
			// For code snips, there is no way to confifgure them
			// So for tackle2Hub we are going to ignore code snips

			// We need to handle the normalization to get to the actual source code
			pathToTest, err := filepath.Rel(filepath.Join(t.kantra.testDir, "source"), i.URI.Filename())
			if err != nil {
				break
			}
			if !strings.Contains(ai.URI.Filename(), pathToTest) {
				continue
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
			validationError = append(validationError, ValidationError{
				Path:     "",
				Message:  fmt.Sprintf("Did not find expected incident: %v", i),
				Expected: expected,
				Actual:   nil,
			})
		}
	}

	return validationError, len(validationError) != 0
}
