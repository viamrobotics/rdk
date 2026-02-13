package capture

import (
	"encoding/json"
	"fmt"
)

// CaptureOverride represents a single capture override from the selective capture sensor.
type CaptureOverride struct {
	ResourceName string   `json:"resource_name"` // Required
	Method       string   `json:"method"`        // Required
	FrequencyHz  *float32 `json:"frequency_hz"`  // Optional (nil means use machine config)
	Tags         []string `json:"tags"`          // Optional (nil means use service-level tags)
}

// CaptureOverrides is the container for override readings from the sensor.
type CaptureOverrides struct {
	Overrides []CaptureOverride `json:"overrides"`
}

// parseOverridesFromReadings extracts and parses override data from sensor readings.
// Returns a slice of CaptureOverride structs, or an error if parsing fails.
// The key parameter specifies which key in the readings map contains the override data.
func parseOverridesFromReadings(readings map[string]interface{}, key string) ([]CaptureOverride, error) {
	overridesData, ok := readings[key]
	if !ok {
		// No overrides key present - return empty slice (use machine config)
		return []CaptureOverride{}, nil
	}

	// Marshal the overrides data to JSON and unmarshal into our struct
	overridesJSON, err := json.Marshal(overridesData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal overrides data: %w", err)
	}

	var overrides []CaptureOverride
	if err := json.Unmarshal(overridesJSON, &overrides); err != nil {
		return nil, fmt.Errorf("failed to unmarshal overrides: %w", err)
	}

	// Validate that required fields are present
	for i, override := range overrides {
		if override.ResourceName == "" {
			return nil, fmt.Errorf("override %d missing required field 'resource_name'", i)
		}
		if override.Method == "" {
			return nil, fmt.Errorf("override %d missing required field 'method'", i)
		}
	}

	return overrides, nil
}

// overridesMapEqual compares two override maps for equality.
// Returns true if the maps are identical, false otherwise.
func overridesMapEqual(a, b map[string]CaptureOverride) bool {
	if len(a) != len(b) {
		return false
	}

	for key, aVal := range a {
		bVal, exists := b[key]
		if !exists {
			return false
		}

		// Compare all fields
		if aVal.ResourceName != bVal.ResourceName ||
			aVal.Method != bVal.Method {
			return false
		}

		// Compare optional FrequencyHz (handle nil pointers)
		if (aVal.FrequencyHz == nil) != (bVal.FrequencyHz == nil) {
			return false
		}
		if aVal.FrequencyHz != nil && *aVal.FrequencyHz != *bVal.FrequencyHz {
			return false
		}

		// Compare optional Tags
		if !tagsEqual(aVal.Tags, bVal.Tags) {
			return false
		}
	}

	return true
}

// tagsEqual compares two tag slices for equality.
func tagsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// buildOverrideKey creates a unique key for an override based on resource name and method.
func buildOverrideKey(resourceName, method string) string {
	return fmt.Sprintf("%s/%s", resourceName, method)
}
