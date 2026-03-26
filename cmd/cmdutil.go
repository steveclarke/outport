package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// ErrSilent is returned when a command wants to set exit code 1
// without printing an error message to stderr.
var ErrSilent = errors.New("")

// jsonEnvelope is the top-level wrapper for all --json output.
type jsonEnvelope struct {
	OK      bool   `json:"ok"`
	Data    any    `json:"data"`
	Summary string `json:"summary,omitempty"`
}

// jsonErrorEnvelope is the top-level wrapper for --json error output.
type jsonErrorEnvelope struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
	Hint  string `json:"hint,omitempty"`
}

// writeJSON marshals v as indented JSON wrapped in an envelope and writes it to the command's stdout.
func writeJSON(cmd *cobra.Command, v any, summary string) error {
	return writeJSONTo(cmd.OutOrStdout(), v, summary)
}

// writeJSONTo marshals v as indented JSON wrapped in an envelope and writes it to the writer.
func writeJSONTo(w io.Writer, v any, summary string) error {
	envelope := jsonEnvelope{
		OK:      true,
		Data:    v,
		Summary: summary,
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(w, string(data))
	return nil
}

// writeJSONError marshals an error envelope and writes it to the writer.
func writeJSONError(w io.Writer, errMsg, hint string) {
	envelope := jsonErrorEnvelope{
		OK:    false,
		Error: errMsg,
		Hint:  hint,
	}
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return // best-effort
	}
	fmt.Fprintln(w, string(data))
}

// pluralize returns singular when n == 1, plural otherwise.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

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

// NoArgs returns a Cobra arg validator that rejects any positional args.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return FlagErrorf("this command does not accept arguments")
	}
	return nil
}

// MaximumArgs returns a Cobra arg validator that accepts at most n args.
func MaximumArgs(n int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return FlagErrorf("%s", msg)
		}
		return nil
	}
}

// RangeArgs returns a Cobra arg validator that accepts between min and max args (inclusive).
func RangeArgs(min, max int, msg string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > max {
			return FlagErrorf("too many arguments")
		}
		if len(args) < min {
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
