package validator

import (
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"strings"

	konveyor "github.com/konveyor/analyzer-lsp/output/v1/konveyor"
)

type tagCompare interface {
	compareTag(expected string, actual []string) (*ValidationError, bool)
}
type violationCompare interface {
	compareViolation(expected, actual konveyor.Violation) ([]ValidationError, bool)
}
type errorsCompare interface {
	compareErrors(expected, actual string) (*ValidationError, bool)
}
type unmatchedCompare interface {
	compareUnmatched(expected string, actual []string) (*ValidationError, bool)
}
type skippedCompare interface {
	compareSkipped(expected string, actual []string) (*ValidationError, bool)
}

func findExpectedString(expected string, actual []string) bool {
	for _, a := range actual {
		if expected == a {
			return true
		}
	}
	return false
}

type comparer interface {
	tagCompare
	violationCompare
	errorsCompare
	unmatchedCompare
	skippedCompare
}

func getComparer(targetType, testDir string) comparer {
	k := &kantra{testDir: testDir}
	switch targetType {
	case "kantra":
		return k
	case "tackle-hub":
		return &tackle2Hub{kantra: *k}
	case "tackle-ui":
		return k
	case "kai-rpc":
		return k
	case "vscode":
		return k
	}
	return nil
}

// ValidationResult contains the result of validation
type ValidationResult struct {
	Passed bool
	Errors []ValidationError
}

// ValidationError represents a single validation failure
type ValidationError struct {
	Path     string
	Message  string
	Expected any
	Actual   any
}

// Validate performs exact match validation between actual and expected rulesets
// This function now takes file paths and compares the raw YAML content
func Validate(actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	return ValidateFiles("", "", actual, expected)
}

// ValidateFiles performs exact match validation by comparing YAML files directly
func ValidateFiles(testDir, targetType string, actual, expected []konveyor.RuleSet) (*ValidationResult, error) {
	result := &ValidationResult{
		Passed: true,
		Errors: []ValidationError{},
	}

	errors := []ValidationError{}
	comparer := getComparer(targetType, testDir)

	for _, ers := range expected {
		for _, rs := range actual {
			if rs.Name != ers.Name {
				continue
			}

			if !maps.Equal(ers.Errors, rs.Errors) {
				for k, eerr := range ers.Errors {
					if err, ok := comparer.compareErrors(eerr, rs.Errors[k]); ok {
						errors = append(errors, *err)
					}
				}
			}

			if !reflect.DeepEqual(rs.Tags, ers.Tags) {
				for _, erstags := range ers.Tags {
					if err, ok := comparer.compareTag(erstags, rs.Tags); ok {
						errors = append(errors, *err)
					}
				}
			}
			if !reflect.DeepEqual(rs.Insights, ers.Insights) {
				for k, ersinsights := range ers.Insights {
					if err, ok := comparer.compareViolation(ersinsights, rs.Insights[k]); ok {

						newMessage := "Did not find Insights\n\t"
						for _, e := range err {
							newMessage = fmt.Sprintf("%s\n\t%s", newMessage, e.Message)
						}

						errors = append(errors, ValidationError{
							Path:     "",
							Message:  newMessage,
							Expected: ersinsights,
						})
					}
				}

			}
			if !reflect.DeepEqual(rs.Violations, ers.Violations) {
				for k, ersinsights := range ers.Violations {
					if err, ok := comparer.compareViolation(ersinsights, rs.Violations[k]); ok {

						newMessage := "Did not find violations\n\t"
						for _, e := range err {
							newMessage = fmt.Sprintf("%s\n\t%s", newMessage, e.Message)
						}

						errors = append(errors, ValidationError{
							Path:     "",
							Message:  newMessage,
							Expected: ersinsights,
						})
					}
				}
			}
			if !reflect.DeepEqual(rs.Unmatched, ers.Unmatched) {
				for _, ersunmatched := range ers.Unmatched {
					if err, ok := comparer.compareUnmatched(ersunmatched, rs.Unmatched); ok {
						errors = append(errors, *err)
					}
				}
			}
			if !reflect.DeepEqual(rs.Skipped, ers.Skipped) {
				for _, ersskipped := range ers.Skipped {
					if err, ok := comparer.compareSkipped(ersskipped, rs.Skipped); ok {
						errors = append(errors, *err)
					}
				}
			}
		}
		errors = append(errors, ValidationError{Path: fmt.Sprintf("ruleset/%s", ers.Name)})
	}

	// If not equal, generate detailed diff
	result.Passed = len(errors) == 0
	result.Errors = errors

	return result, nil
}

// normalizeYAMLPaths normalizes paths in YAML by removing test directory paths
// and normalizing file:// URIs to use consistent base paths
func normalizeYAMLPaths(yamlStr, testDir, targetType string) string {
	// Replace the test directory path with empty string
	if testDir != "" {
		yamlStr = strings.ReplaceAll(yamlStr, testDir, "")
	}

	// Normalize file:// URIs by removing variable base paths
	// Common patterns:
	// - file:///opt/input/source/ (kantra)
	// - file:///shared/source/{repo-name}/ (tackle-hub)
	// - file:///root/.m2/repository/ (maven cache)

	// Replace kantra source path
	yamlStr = strings.ReplaceAll(yamlStr, "file:///opt/input/source/", "file:///source/")

	// Replace tackle-hub source paths using regex to match any repo name
	// Pattern: file:///shared/source/{anything}/ -> file:///source/
	re := regexp.MustCompile(`file:///shared/source/[^/]+/`)
	yamlStr = re.ReplaceAllString(yamlStr, "file:///source/")

	// Normalize maven repository paths
	yamlStr = strings.ReplaceAll(yamlStr, "file:///root/.m2/repository/", "file:///m2/")
	yamlStr = strings.ReplaceAll(yamlStr, "file:///cache/m2/repository/", "file:///m2/")

	// Apply tackle-hub specific filtering
	if targetType == "tackle-hub" {
		// Remove codeSnip fields to reduce noise in diffs
		// This removes lines starting with "codeSnip:" and continuation lines
		lines := strings.Split(yamlStr, "\n")
		var filtered []string
		skipNext := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Check if this is a codeSnip line
			if strings.HasPrefix(trimmed, "codeSnip:") {
				// Skip this line and check if next lines are part of multiline value
				skipNext = true
				continue
			}

			// If we're in skip mode, check if this line is part of the multiline content
			if skipNext {
				// If line starts with spaces and doesn't look like a new field, skip it
				if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && !strings.Contains(trimmed, ":") {
					continue
				}
				// Otherwise, we've reached the next field
				skipNext = false
			}

			filtered = append(filtered, line)
		}
		yamlStr = strings.Join(filtered, "\n")
	}

	return yamlStr
}
