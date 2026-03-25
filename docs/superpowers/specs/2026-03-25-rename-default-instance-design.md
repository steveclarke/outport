# Rename: Default to Current Instance

## Summary

When `outport rename` is run with a single argument from a directory containing
`outport.yml`, it should default to renaming the current directory's instance.
This matches the pattern already established by `outport promote`.

## Current Behavior

```
outport rename <old> <new>   # Always requires both arguments
```

## Proposed Behavior

```
outport rename <new>          # Rename current instance to <new>
outport rename <old> <new>    # Explicit form (unchanged)
```

**Usage line:** `rename [old] <new>`

## Implementation

1. **New `RangeArgs` validator** in `cmd/cmdutil.go` — accepts between `min` and
   `max` positional args, returning a `FlagError` with the provided message if
   out of range.

2. **Rename command changes** in `cmd/rename.go`:
   - Change `Args` from `ExactArgs(2, ...)` to `RangeArgs(1, 2, ...)`.
   - In `runRename`: if 1 arg, use `ctx.Instance` as `oldName` and `args[0]` as
     `newName`. If 2 args, same as today.

3. **Tests** in `cmd/cmd_test.go`:
   - 1-arg rename from a registered non-main instance directory.
   - 1-arg rename when current instance is "main" (valid operation).

## Error Cases

- **Same name:** `outport rename <name>` where `<name>` equals current instance
  — existing error: "old and new instance names are the same."
- **Not in a project directory:** `loadProjectContext()` fails via
  `config.FindDir()` — no change needed.
- **Zero args:** `RangeArgs` validator rejects with usage hint.
- **Three+ args:** `RangeArgs` validator rejects with "too many arguments."
