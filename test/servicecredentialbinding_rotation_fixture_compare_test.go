package test

import "testing"

func TestRotationFixtureSpecNestedReferenceDefaults(t *testing.T) {
	fixture := func(referenceField string, policy map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"forProvider": map[string]interface{}{
				referenceField: map[string]interface{}{
					"name":   "rotation-target-build123",
					"policy": policy,
				},
			},
		}
	}

	tests := []struct {
		name     string
		expected map[string]interface{}
		actual   map[string]interface{}
		want     bool
	}{
		{
			name:     "orgRef Required default is tolerated",
			expected: fixture("orgRef", map[string]interface{}{"resolve": "Always"}),
			actual:   fixture("orgRef", map[string]interface{}{"resolve": "Always", "resolution": "Required"}),
			want:     true,
		},
		{
			name:     "spaceRef Required default is tolerated",
			expected: fixture("spaceRef", map[string]interface{}{"resolve": "Always"}),
			actual:   fixture("spaceRef", map[string]interface{}{"resolve": "Always", "resolution": "Required"}),
			want:     true,
		},
		{
			name:     "different nested default is rejected",
			expected: fixture("orgRef", map[string]interface{}{"resolve": "Always"}),
			actual:   fixture("orgRef", map[string]interface{}{"resolve": "Always", "resolution": "Optional"}),
			want:     false,
		},
		{
			name:     "unknown nested field is rejected",
			expected: fixture("orgRef", map[string]interface{}{"resolve": "Always"}),
			actual:   fixture("orgRef", map[string]interface{}{"resolve": "Always", "other": "unexpected"}),
			want:     false,
		},
		{
			name:     "explicit fixture value remains strict",
			expected: fixture("orgRef", map[string]interface{}{"resolve": "Always", "resolution": "Optional"}),
			actual:   fixture("orgRef", map[string]interface{}{"resolve": "Always", "resolution": "Required"}),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RotationFixtureValueMatches(tt.expected, tt.actual); got != tt.want {
				t.Fatalf("RotationFixtureValueMatches() = %t, want %t", got, tt.want)
			}
		})
	}
}
