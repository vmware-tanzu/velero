package wildcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldExpandWildcards(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		expected bool
	}{
		{
			name:     "no wildcards",
			includes: []string{"ns1", "ns2"},
			excludes: []string{"ns3", "ns4"},
			expected: false,
		},
		{
			name:     "includes has star - should not expand",
			includes: []string{"*"},
			excludes: []string{"ns1"},
			expected: false,
		},
		{
			name:     "includes has star after a wildcard pattern - should not expand",
			includes: []string{"ns*", "*"},
			excludes: []string{"ns1"},
			expected: false,
		},
		{
			name:     "includes has wildcard pattern",
			includes: []string{"ns*"},
			excludes: []string{"ns1"},
			expected: true,
		},
		{
			name:     "excludes has wildcard pattern",
			includes: []string{"ns1"},
			excludes: []string{"ns*"},
			expected: true,
		},
		{
			name:     "both have wildcard patterns",
			includes: []string{"app-*"},
			excludes: []string{"test-*"},
			expected: true,
		},
		{
			name:     "includes has star and wildcard - star takes precedence",
			includes: []string{"*", "ns*"},
			excludes: []string{},
			expected: false,
		},
		{
			name:     "double asterisk should be detected as wildcard",
			includes: []string{"**"},
			excludes: []string{},
			expected: true, // ** is a wildcard pattern (but will error during validation)
		},
		{
			name:     "empty slices",
			includes: []string{},
			excludes: []string{},
			expected: false,
		},
		{
			name:     "complex wildcard patterns",
			includes: []string{"*-prod"},
			excludes: []string{"test-*-staging"},
			expected: true,
		},
		{
			name:     "question mark wildcard",
			includes: []string{"ns?"},
			excludes: []string{},
			expected: true, // question mark is now considered a wildcard
		},
		{
			name:     "character class wildcard",
			includes: []string{"ns[abc]"},
			excludes: []string{},
			expected: true, // character class is considered wildcard
		},
		{
			name:     "brace alternatives wildcard",
			includes: []string{"ns{prod,staging}"},
			excludes: []string{},
			expected: true, // brace alternatives are considered wildcard
		},
		{
			name:     "dot is literal - not wildcard",
			includes: []string{"app.prod"},
			excludes: []string{},
			expected: false, // dot is literal, not wildcard
		},
		{
			name:     "plus is literal - not wildcard",
			includes: []string{"app+"},
			excludes: []string{},
			expected: false, // plus is literal, not wildcard
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldExpandWildcards(tt.includes, tt.excludes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExpandWildcards(t *testing.T) {
	tests := []struct {
		name             string
		activeNamespaces []string
		includes         []string
		excludes         []string
		expectedIncludes []string
		expectedExcludes []string
		expectError      bool
	}{
		{
			name:             "no wildcards",
			activeNamespaces: []string{"ns1", "ns2", "ns3"},
			includes:         []string{"ns1", "ns4"},
			excludes:         []string{"ns2"},
			expectedIncludes: []string{"ns1", "ns4"},
			expectedExcludes: []string{"ns2"},
			expectError:      false,
		},
		{
			name:             "wildcard in includes",
			activeNamespaces: []string{"app-prod", "app-staging", "db-prod", "test-ns"},
			includes:         []string{"app-*"},
			excludes:         []string{"test-ns"},
			expectedIncludes: []string{"app-prod", "app-staging"},
			expectedExcludes: []string{"test-ns"},
			expectError:      false,
		},
		{
			name:             "wildcard in excludes",
			activeNamespaces: []string{"app-prod", "app-staging", "db-prod", "test-ns"},
			includes:         []string{"app-prod"},
			excludes:         []string{"*-staging"},
			expectedIncludes: []string{"app-prod"},
			expectedExcludes: []string{"app-staging"},
			expectError:      false,
		},
		{
			name:             "wildcards in both",
			activeNamespaces: []string{"app-prod", "app-staging", "db-prod", "db-staging", "test-ns"},
			includes:         []string{"*-prod"},
			excludes:         []string{"*-staging"},
			expectedIncludes: []string{"app-prod", "db-prod"},
			expectedExcludes: []string{"app-staging", "db-staging"},
			expectError:      false,
		},
		{
			name:             "star pattern in includes",
			activeNamespaces: []string{"ns1", "ns2", "ns3"},
			includes:         []string{"*"},
			excludes:         []string{},
			expectedIncludes: []string{"ns1", "ns2", "ns3"},
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "empty active namespaces",
			activeNamespaces: []string{},
			includes:         []string{"app-*"},
			excludes:         []string{"test-*"},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "empty includes and excludes",
			activeNamespaces: []string{"ns1", "ns2"},
			includes:         []string{},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "complex patterns",
			activeNamespaces: []string{"my-app-prod", "my-app-staging", "your-app-prod", "system-ns"},
			includes:         []string{"*-app-*"},
			excludes:         []string{"*-staging"},
			expectedIncludes: []string{"my-app-prod", "my-app-staging", "your-app-prod"},
			expectedExcludes: []string{"my-app-staging"},
			expectError:      false,
		},
		{
			name:             "double asterisk should error",
			activeNamespaces: []string{"ns1", "ns2", "ns3"},
			includes:         []string{"**"},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      true, // ** is invalid
		},
		{
			name:             "double asterisk in pattern should error",
			activeNamespaces: []string{"ns1", "ns2", "ns3"},
			includes:         []string{"app-**"},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      true, // app-** contains ** which is invalid
		},
		{
			name:             "question mark patterns",
			activeNamespaces: []string{"ns1", "ns2", "ns10", "test"},
			includes:         []string{"ns?"},
			excludes:         []string{},
			expectedIncludes: []string{"ns1", "ns2"}, // ? matches single character
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "character class patterns",
			activeNamespaces: []string{"nsa", "nsb", "nsc", "nsx", "ns1"},
			includes:         []string{"ns[abc]"},
			excludes:         []string{},
			expectedIncludes: []string{"nsa", "nsb", "nsc"}, // [abc] matches a, b, or c
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "brace alternative patterns",
			activeNamespaces: []string{"app-prod", "app-staging", "app-dev", "db-prod"},
			includes:         []string{"app-{prod,staging}"},
			excludes:         []string{},
			expectedIncludes: []string{"app-prod", "app-staging"}, // {prod,staging} matches either
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "literal dot and plus patterns",
			activeNamespaces: []string{"app.prod", "app-prod", "app+", "app"},
			includes:         []string{"app.prod", "app+"},
			excludes:         []string{},
			expectedIncludes: []string{"app.prod", "app+"}, // . and + are literal
			expectedExcludes: nil,
			expectError:      false,
		},
		{
			name:             "unsupported regex patterns should error",
			activeNamespaces: []string{"ns1", "ns2"},
			includes:         []string{"ns(1|2)"},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      true, // |, (, ) are not supported
		},
		{
			name:             "unclosed brace patterns should error",
			activeNamespaces: []string{"app-prod"},
			includes:         []string{"app-{prod,staging"},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      true, // unclosed brace
		},
		{
			name:             "empty brace patterns should error",
			activeNamespaces: []string{"app-prod"},
			includes:         []string{"app-{}"},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      true, // empty braces
		},
		{
			name:             "unmatched closing brace should error",
			activeNamespaces: []string{"app-prod"},
			includes:         []string{"app-prod}"},
			excludes:         []string{},
			expectedIncludes: nil,
			expectedExcludes: nil,
			expectError:      true, // unmatched closing brace
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			includes, excludes, err := ExpandWildcards(tt.activeNamespaces, tt.includes, tt.excludes)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.expectedIncludes, includes)
			assert.ElementsMatch(t, tt.expectedExcludes, excludes)
		})
	}
}

func TestExpandWildcardsPrivate(t *testing.T) {
	tests := []struct {
		name             string
		patterns         []string
		activeNamespaces []string
		expected         []string
		expectError      bool
	}{
		{
			name:             "empty patterns",
			patterns:         []string{},
			activeNamespaces: []string{"ns1", "ns2"},
			expected:         nil,
			expectError:      false,
		},
		{
			name:             "non-wildcard patterns",
			patterns:         []string{"ns1", "ns3"},
			activeNamespaces: []string{"ns1", "ns2"},
			expected:         []string{"ns1", "ns3"}, // includes ns3 even if not in active
			expectError:      false,
		},
		{
			name:             "star pattern",
			patterns:         []string{"*"},
			activeNamespaces: []string{"ns1", "ns2", "ns3"},
			expected:         []string{"ns1", "ns2", "ns3"},
			expectError:      false,
		},
		{
			name:             "simple wildcard",
			patterns:         []string{"app-*"},
			activeNamespaces: []string{"app-prod", "app-staging", "db-prod"},
			expected:         []string{"app-prod", "app-staging"},
			expectError:      false,
		},
		{
			name:             "multiple patterns",
			patterns:         []string{"app-*", "db-prod", "*-test"},
			activeNamespaces: []string{"app-prod", "app-staging", "db-prod", "service-test", "other"},
			expected:         []string{"app-prod", "app-staging", "db-prod", "service-test"},
			expectError:      false,
		},
		{
			name:             "wildcard with no matches",
			patterns:         []string{"missing-*"},
			activeNamespaces: []string{"app-prod", "db-staging"},
			expected:         []string{}, // returns empty slice, not nil
			expectError:      false,
		},
		{
			name:             "brace patterns work correctly",
			patterns:         []string{"app-{prod,staging}"},
			activeNamespaces: []string{"app-prod", "app-staging", "app-dev", "app-{prod,staging}"},
			expected:         []string{"app-prod", "app-staging"}, // brace patterns do expand
			expectError:      false,
		},
		{
			name:             "duplicate matches from multiple patterns",
			patterns:         []string{"app-*", "*-prod"},
			activeNamespaces: []string{"app-prod", "app-staging", "db-prod"},
			expected:         []string{"app-prod", "app-staging", "db-prod"}, // no duplicates
			expectError:      false,
		},
		{
			name:             "question mark pattern - glob wildcard",
			patterns:         []string{"ns?"},
			activeNamespaces: []string{"ns1", "ns2", "ns10"},
			expected:         []string{"ns1", "ns2"}, // ? is a glob pattern for single character
			expectError:      false,
		},
		{
			name:             "character class patterns",
			patterns:         []string{"ns[12]"},
			activeNamespaces: []string{"ns1", "ns2", "ns3", "nsa"},
			expected:         []string{"ns1", "ns2"}, // [12] matches 1 or 2
			expectError:      false,
		},
		{
			name:             "character range patterns",
			patterns:         []string{"ns[a-c]"},
			activeNamespaces: []string{"nsa", "nsb", "nsc", "nsd", "ns1"},
			expected:         []string{"nsa", "nsb", "nsc"}, // [a-c] matches a to c
			expectError:      false,
		},
		{
			name:             "negated character class",
			patterns:         []string{"ns[!abc]"},
			activeNamespaces: []string{"nsa", "nsb", "nsc", "nsd", "ns1"},
			expected:         []string{"nsd", "ns1"}, // [!abc] matches anything except a, b, c
			expectError:      false,
		},
		{
			name:             "brace alternatives",
			patterns:         []string{"app-{prod,test}"},
			activeNamespaces: []string{"app-prod", "app-test", "app-staging", "db-prod"},
			expected:         []string{"app-prod", "app-test"}, // {prod,test} matches either
			expectError:      false,
		},
		{
			name:             "double asterisk should error",
			patterns:         []string{"**"},
			activeNamespaces: []string{"app-prod", "app.staging", "db/prod"},
			expected:         nil,
			expectError:      true, // ** is not allowed
		},
		{
			name:             "literal dot and plus",
			patterns:         []string{"app.prod", "service+"},
			activeNamespaces: []string{"app.prod", "appXprod", "service+", "service"},
			expected:         []string{"app.prod", "service+"}, // . and + are literal
			expectError:      false,
		},
		{
			name:             "unsupported regex symbols should error",
			patterns:         []string{"ns(1|2)"},
			activeNamespaces: []string{"ns1", "ns2"},
			expected:         nil,
			expectError:      true, // |, (, ) not supported
		},
		{
			name:             "double asterisk should error",
			patterns:         []string{"**"},
			activeNamespaces: []string{"ns1", "ns2"},
			expected:         nil,
			expectError:      true, // ** not allowed
		},
		{
			name:             "double asterisk in pattern should error",
			patterns:         []string{"app-**-prod"},
			activeNamespaces: []string{"app-prod"},
			expected:         nil,
			expectError:      true, // ** not allowed anywhere
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandWildcards(tt.patterns, tt.activeNamespaces)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else if len(tt.expected) == 0 {
				assert.Empty(t, result)
			} else {
				assert.ElementsMatch(t, tt.expected, result)
			}
		})
	}
}

// Edge case tests
func TestExpandWildcardsEdgeCases(t *testing.T) {
	t.Run("nil inputs", func(t *testing.T) {
		includes, excludes, err := ExpandWildcards(nil, nil, nil)
		assert.NoError(t, err)
		assert.Nil(t, includes)
		assert.Nil(t, excludes)
	})

	t.Run("empty string patterns", func(t *testing.T) {
		activeNamespaces := []string{"ns1", "ns2"}
		result, err := expandWildcards([]string{""}, activeNamespaces)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{""}, result) // empty string is treated as literal
	})

	t.Run("whitespace patterns", func(t *testing.T) {
		activeNamespaces := []string{"ns1", " ", "ns2"}
		result, err := expandWildcards([]string{" "}, activeNamespaces)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{" "}, result)
	})

	t.Run("special characters in namespace names", func(t *testing.T) {
		activeNamespaces := []string{"ns-1", "ns_2", "ns.3", "ns@4"}
		result, err := expandWildcards([]string{"ns*"}, activeNamespaces)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"ns-1", "ns_2", "ns.3", "ns@4"}, result)
	})

	t.Run("complex glob combinations", func(t *testing.T) {
		activeNamespaces := []string{"app1-prod", "app2-prod", "app1-test", "db-prod", "service"}
		result, err := expandWildcards([]string{"app?-{prod,test}"}, activeNamespaces)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"app1-prod", "app2-prod", "app1-test"}, result)
	})

	t.Run("escaped characters", func(t *testing.T) {
		activeNamespaces := []string{"app*", "app-prod", "app?test", "app-test"}
		result, err := expandWildcards([]string{"app\\*"}, activeNamespaces)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"app*"}, result)
	})

	t.Run("mixed literal and wildcard patterns", func(t *testing.T) {
		activeNamespaces := []string{"app.prod", "app-prod", "app_prod", "test.ns"}
		result, err := expandWildcards([]string{"app.prod", "app?prod"}, activeNamespaces)
		assert.NoError(t, err)
		assert.ElementsMatch(t, []string{"app.prod", "app-prod", "app_prod"}, result)
	})

	t.Run("conservative asterisk validation", func(t *testing.T) {
		tests := []struct {
			name        string
			pattern     string
			shouldError bool
		}{
			{"single asterisk", "*", false},
			{"double asterisk", "**", true},
			{"triple asterisk", "***", true},
			{"quadruple asterisk", "****", true},
			{"mixed with double", "app-**", true},
			{"double in middle", "app-**-prod", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := expandWildcards([]string{tt.pattern}, []string{"test"})
				if tt.shouldError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("malformed pattern validation", func(t *testing.T) {
		tests := []struct {
			name        string
			pattern     string
			shouldError bool
		}{
			{"unclosed bracket", "ns[abc", true},
			{"unclosed brace", "app-{prod,staging", true},
			{"nested unclosed", "ns[a{bc", true},
			{"valid bracket", "ns[abc]", false},
			{"valid brace", "app-{prod,staging}", false},
			{"empty bracket", "ns[]", true}, // empty brackets are invalid
			{"empty brace", "app-{}", true}, // empty braces are invalid
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := expandWildcards([]string{tt.pattern}, []string{"test"})
				if tt.shouldError {
					assert.Error(t, err, "Expected error for pattern: %s", tt.pattern)
				} else {
					assert.NoError(t, err, "Expected no error for pattern: %s", tt.pattern)
				}
			})
		}
	})
}
