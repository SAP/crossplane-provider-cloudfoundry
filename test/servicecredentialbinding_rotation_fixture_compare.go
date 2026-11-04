package test

import "reflect"

// RotationFixtureValueMatches checks that a fetched spec still matches the
// fixture while allowing fields defaulted by Crossplane at the top-level spec.
func RotationFixtureValueMatches(expected, actual any) bool {
	return rotationFixtureMapMatches(expected, actual, true)
}

func rotationFixtureMapMatches(expected, actual any, allowManagedDefaults bool) bool {
	return rotationFixtureMapMatchesAtPath(expected, actual, allowManagedDefaults, nil)
}

func rotationFixtureMapMatchesAtPath(expected, actual any, allowManagedDefaults bool, path []string) bool {
	expectedMap, isMap := expected.(map[string]interface{})
	if !isMap {
		return reflect.DeepEqual(expected, actual)
	}
	actualMap, ok := actual.(map[string]interface{})
	if !ok || !rotationFixtureExpectedFieldsMatch(expectedMap, actualMap, path) {
		return false
	}
	return rotationFixtureActualFieldsAllowed(expectedMap, actualMap, allowManagedDefaults, path)
}

func rotationFixtureExpectedFieldsMatch(expected, actual map[string]interface{}, path []string) bool {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok || !rotationFixtureMapMatchesAtPath(expectedValue, actualValue, false, append(path, key)) {
			return false
		}
	}
	return true
}

func rotationFixtureActualFieldsAllowed(expected, actual map[string]interface{}, allowManagedDefaults bool, path []string) bool {
	for key, actualValue := range actual {
		if _, exists := expected[key]; exists {
			continue
		}
		if allowManagedDefaults && rotationFixtureDefaultedSpecField(key) {
			continue
		}
		if rotationFixtureDefaultedReferencePolicyField(path, key, actualValue) {
			continue
		}
		return false
	}
	return true
}

func rotationFixtureDefaultedSpecField(field string) bool {
	switch field {
	case "managementPolicies", "providerConfigRef", "deletionPolicy":
		return true
	default:
		return false
	}
}

func rotationFixtureDefaultedReferencePolicyField(path []string, field string, actual any) bool {
	if field != "resolution" || actual != "Required" || len(path) < 2 || path[len(path)-1] != "policy" {
		return false
	}
	referenceField := path[len(path)-2]
	switch referenceField {
	case "orgRef", "orgSelector", "spaceRef", "spaceSelector", "serviceInstanceRef", "domainRef":
		return true
	default:
		return false
	}
}
