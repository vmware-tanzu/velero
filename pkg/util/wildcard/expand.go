package wildcard

import (
	"strings"

	"github.com/gobwas/glob"
)

func ShouldExpandWildcards(includes []string, excludes []string) bool {
	for _, include := range includes {
		// Should never hit this case, but just in case
		if include == "*" {
			return false
		}

		if strings.Contains(include, "*") {
			return true
		}
	}

	for _, exclude := range excludes {
		if strings.Contains(exclude, "*") {
			return true
		}
	}

	return false
}

func ExpandWildcards(activeNamespaces []string, includes []string, excludes []string) ([]string, []string, error) {

	expandedIncludes, err := expandWildcards(includes, activeNamespaces)
	if err != nil {
		return nil, nil, err
	}

	expandedExcludes, err := expandWildcards(excludes, activeNamespaces)
	if err != nil {
		return nil, nil, err
	}

	return expandedIncludes, expandedExcludes, nil
}

// expands wildcard patterns into a list of namespaces, while normally passing non-wildcard patterns
func expandWildcards(patterns []string, activeNamespaces []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	matchedSet := make(map[string]struct{})

	for _, pattern := range patterns {
		// Special case "*" to match all namespaces
		// This case should never happen since it is handled by the caller
		if pattern == "*" {
			for _, ns := range activeNamespaces {
				matchedSet[ns] = struct{}{}
			}
			continue
		}

		// If the pattern is a non-wildcard pattern, we can just add it to the result
		if !strings.Contains(pattern, "*") {
			matchedSet[pattern] = struct{}{}
			continue
		}

		// Compile glob pattern
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, err
		}

		// Match against all namespaces
		for _, ns := range activeNamespaces {
			if g.Match(ns) {
				matchedSet[ns] = struct{}{}
			}
		}
	}

	// Convert set to slice
	result := make([]string, 0, len(matchedSet))
	for ns := range matchedSet {
		result = append(result, ns)
	}

	return result, nil
}
