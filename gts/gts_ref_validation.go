package gts

import "fmt"

type GtsRefValidationMode string

const (
	GtsRefValidationNone     GtsRefValidationMode = "none"
	GtsRefValidationPresence GtsRefValidationMode = "presence"
	GtsRefValidationFull     GtsRefValidationMode = "full"
)

func ParseGtsRefValidationMode(value string) (GtsRefValidationMode, error) {
	if value == "" {
		return GtsRefValidationFull, nil
	}
	mode := GtsRefValidationMode(value)
	switch mode {
	case GtsRefValidationNone, GtsRefValidationPresence, GtsRefValidationFull:
		return mode, nil
	default:
		return "", fmt.Errorf("gts-ref-validation must be one of: none, presence, full")
	}
}
