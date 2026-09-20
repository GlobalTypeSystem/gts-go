package gts

import "fmt"

type GtsRefValidationMode string

const (
	GtsRefValidationNone       GtsRefValidationMode = "none"
	GtsRefValidationAnyPresent GtsRefValidationMode = "any-present"
	GtsRefValidationAnyValid   GtsRefValidationMode = "any-valid"
)

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
