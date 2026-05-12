package config

import (
	"sort"
	"strings"
)

const skipCommentScanBytes = 500

// - # SKIPPED: … or # SKIPPED … with no leading target token → skip for all targets.
// - # SKIPPED: kantra or # SKIPPED: tackle-hub (comma-separated) → skip only for those targets.
// - "hub" is accepted as an alias for tackle-hub.
func parseSkipCommentPreamble(data []byte) (skipAll bool, onlyTargets []string, found bool) {
	scan := data
	if len(scan) > skipCommentScanBytes {
		scan = scan[:skipCommentScanBytes]
	}

	var merged map[string]struct{}
	lineFound := false
	sawUnconditional := false

	for _, rawLine := range strings.Split(string(scan), "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		body := strings.TrimSpace(line[1:])
		lowerBody := strings.ToLower(body)
		const skippedWord = "skipped"
		if !strings.HasPrefix(lowerBody, skippedWord) {
			continue
		}
		lineFound = true
		rest := body[len(skippedWord):]
		rest = strings.TrimSpace(strings.TrimLeft(rest, ": \t"))
		if rest == "" {
			sawUnconditional = true
			continue
		}
		lineTargets, lineAll := parseSkipRestTargets(rest)
		if lineAll {
			sawUnconditional = true
		} else {
			if merged == nil {
				merged = make(map[string]struct{})
			}
			for _, t := range lineTargets {
				merged[t] = struct{}{}
			}
		}
	}

	if !lineFound {
		return false, nil, false
	}
	if sawUnconditional {
		return true, nil, true
	}
	if len(merged) == 0 {
		return true, nil, true
	}
	out := make([]string, 0, len(merged))
	for t := range merged {
		out = append(out, t)
	}
	sort.Strings(out)
	return false, out, true
}

func parseSkipRestTargets(rest string) (targets []string, skipAll bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return nil, true
	}
	segs := strings.Split(rest, ",")
	var out []string
	for _, seg := range segs {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		w := strings.ToLower(fields[0])
		switch w {
		case "kantra":
			out = append(out, "kantra")
		case "tackle-hub":
			out = append(out, "tackle-hub")
		case "hub":
			out = append(out, "tackle-hub")
		default:
			return nil, true
		}
	}
	if len(out) == 0 {
		return nil, true
	}
	// Dedupe
	seen := make(map[string]struct{})
	dedup := make([]string, 0, len(out))
	for _, t := range out {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		dedup = append(dedup, t)
	}
	return dedup, false
}
