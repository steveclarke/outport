package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

// FlagError is a special error type that signals the error is due to
// incorrect usage (bad args, bad flags, mutually exclusive options).
// The root command's error handler shows usage when it sees this type.
// Modeled after the GitHub CLI's pattern.
type FlagError struct {
	err error
}

func (e *FlagError) Error() string {
	return e.err.Error()
}

func (e *FlagError) Unwrap() error {
	return e.err
}

// IsFlagError returns true if the error is a FlagError.
func IsFlagError(err error) bool {
	var fe *FlagError
	return errors.As(err, &fe)
}

// FlagErrorf creates a new FlagError with a formatted message.
func FlagErrorf(format string, args ...any) error {
	return &FlagError{err: fmt.Errorf(format, args...)}
}

// ExactArgs returns a Cobra arg validator that requires exactly n args.
// If the wrong number of args is provided, it returns a FlagError with
// the given message (which triggers usage display).
func ExactArgs(n int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return FlagErrorf("too many arguments")
		}
		if len(args) < n {
			return FlagErrorf("%s", msg)
		}
		return nil
	}
}

// MinimumArgs returns a Cobra arg validator that requires at least n args.
func MinimumArgs(n int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return FlagErrorf("%s", msg)
		}
		return nil
	}
}
