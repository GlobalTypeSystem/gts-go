package gts

import "fmt"

// GtsRefValidationMode controls how strictly x-gts-*-ref annotations are
// checked when an entity is validated.
type GtsRefValidationMode string

const (
	// GtsRefValidationNone disables reference validation entirely.
	GtsRefValidationNone GtsRefValidationMode = "none"
	// GtsRefValidationAnyPresent requires only that referenced entities exist
	// in the registry, without validating them.
	GtsRefValidationAnyPresent GtsRefValidationMode = "any-present"
	// GtsRefValidationAnyValid requires referenced entities to exist and to be
	// valid. This is the default mode.
	GtsRefValidationAnyValid GtsRefValidationMode = "any-valid"
)

// ParseGtsRefValidationMode parses a mode string into a GtsRefValidationMode.
// An empty string yields GtsRefValidationAnyValid (the default); any other
// unrecognized value returns an error.
func ParseGtsRefValidationMode(value string) (GtsRefValidationMode, error) {
	if value == "" {
		return GtsRefValidationAnyValid, nil
	}
	mode := GtsRefValidationMode(value)
	switch mode {
	case GtsRefValidationNone, GtsRefValidationAnyPresent, GtsRefValidationAnyValid:
		return mode, nil
	default:
		return "", fmt.Errorf("gts-ref-validation must be one of: none, any-present, any-valid")
	}
}
