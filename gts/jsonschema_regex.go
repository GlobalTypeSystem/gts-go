/*
Copyright © 2025 Global Type System
Released under Apache License 2.0
*/

package gts

import (
	"errors"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// jsonschemaRegexpEngine adapts the GTS profile to jsonschema/v6. The library
// treats any engine error in format: "regex" as a format violation, which
// "not" can invert. Panic only for engine failures to abort the whole operation;
// unsupported expressions remain ordinary validation errors.
// Every Compile or Validate using this adapter must run under guardRegexEngine.
func jsonschemaRegexpEngine(source string) (jsonschema.Regexp, error) {
	re, err := compileRegexProfile(source)
	var failure regexEngineFailure
	if errors.As(err, &failure) {
		panic(failure)
	}
	if err != nil {
		return nil, err
	}
	return re, nil
}

// guardRegexEngine contains the adapter's panics within jsonschema operations
// and returns engine failures as errors. Unrelated panics propagate unchanged.
func guardRegexEngine(run func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			failure, ok := r.(regexEngineFailure)
			if !ok {
				panic(r)
			}
			err = failure
		}
	}()
	return run()
}
