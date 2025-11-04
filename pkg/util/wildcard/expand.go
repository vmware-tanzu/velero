package wildcard

import (
	"errors"
	"strings"
	"unicode"

	"github.com/gobwas/glob"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ShouldExpandWildcards(includes []string, excludes []string) bool {
	wildcardFound := false
	for _, include := range includes {
		// Special case: "*" alone means "match all" - don't expand
		if include == "*" {
			return false
		}

		if containsWildcardPattern(include) {
			wildcardFound = true
		}
	}

	for _, exclude := range excludes {
		if containsWildcardPattern(exclude) {
			wildcardFound = true
		}
	}

	return wildcardFound
}

// containsWildcardPattern checks if a pattern contains any wildcard symbols
// Supported patterns: *, ?, [abc], {a,b,c}
// Note: . and + are treated as literal characters (not wildcards)
// Note: ** and consecutive asterisks are NOT supported (will cause validation error)
func containsWildcardPattern(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[{")
}

func validateWildcardPatterns(patterns []string) error {
	for _, pattern := range patterns {
		// Check for invalid regex-only patterns that we don't support
		if strings.ContainsAny(pattern, "|()") {
			return errors.New("wildcard pattern contains unsupported regex symbols: |, (, )")
		}

		// Check for consecutive asterisks (2 or more)
		if strings.Contains(pattern, "**") {
			return errors.New("wildcard pattern contains consecutive asterisks (only single * allowed)")
		}

		// Check for malformed brace patterns
		if err := validateBracePatterns(pattern); err != nil {
			return err
		}
	}
	return nil
}

// validateBracePatterns checks for malformed brace patterns like unclosed braces or empty braces
func validateBracePatterns(pattern string) error {
	openBraces := 0
	i := 0

	for i < len(pattern) {
		switch pattern[i] {
		case '{':
			start := i
			openBraces++
			i++

			// Look for the closing brace
			hasContent := false
			for i < len(pattern) && pattern[i] != '}' {
				if pattern[i] != ',' && !unicode.IsSpace(rune(pattern[i])) {
					hasContent = true
				}
				i++
			}

			if i >= len(pattern) {
				return errors.New("wildcard pattern contains unclosed brace '{'")
			}

			// Check if brace content is empty or just commas/spaces
			braceContent := pattern[start+1 : i]
			if !hasContent || braceContent == "" || strings.Trim(braceContent, ", \t") == "" {
				return errors.New("wildcard pattern contains empty brace pattern '{}'")
			}

			openBraces--
		case '}':
			if openBraces == 0 {
				return errors.New("wildcard pattern contains unmatched closing brace '}'")
			}
			openBraces--
		}
		i++
	}

	if openBraces > 0 {
		return errors.New("wildcard pattern contains unclosed brace '{'")
	}

	return nil
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

	// Validate patterns before processing
	if err := validateWildcardPatterns(patterns); err != nil {
		return nil, err
	}

	matchedSet := make(map[string]struct{})

	for _, pattern := range patterns {
		// If the pattern is a non-wildcard pattern, we can just add it to the result
		if !containsWildcardPattern(pattern) {
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

// GetWildcardResult returns the final list of namespaces after applying wildcard include/exclude logic
func GetWildcardResult(expandedIncludes []string, expandedExcludes []string) []string {
	// Set check: set of expandedIncludes - set of expandedExcludes
	expandedIncludesSet := sets.New(expandedIncludes...)
	expandedExcludesSet := sets.New(expandedExcludes...)
	selectedNamespacesSet := expandedIncludesSet.Difference(expandedExcludesSet)

	// Convert the set to a slice
	selectedNamespaces := make([]string, 0, selectedNamespacesSet.Len())
	for ns := range selectedNamespacesSet {
		selectedNamespaces = append(selectedNamespaces, ns)
	}

	return selectedNamespaces
}
