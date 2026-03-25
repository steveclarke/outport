package config

import "strings"

// ExpandVars performs bash-style parameter expansion on a template string, replacing
// variable references with values from the vars map.
//
// This is the core template engine used by ResolveComputed to produce final environment
// variable values. It is also used directly for hostname and URL template expansion
// during allocation.
//
// Supported syntax:
//
//   - ${var}              — substitute the value of var from the map (empty string if missing)
//   - ${var:-default}     — use default if var is empty or missing from the map
//   - ${var:+replacement} — use replacement if var is non-empty; empty string otherwise
//
// Variable names may contain word characters (a-z, A-Z, 0-9, underscore) and dots.
// Dotted names like "rails.port" are used for service field references. Colon-suffixed
// names like "rails.url:direct" are used for field modifiers -- the full string including
// the colon is looked up as a single key in the vars map.
//
// Nested variable references inside default/replacement text are expanded recursively.
// For example, "${db:-${fallback}}" will expand ${fallback} if "db" is empty.
//
// Malformed or unterminated expressions (e.g., "${" at end of string) are returned
// as literal text rather than causing an error.
func ExpandVars(template string, vars map[string]string) string {
	var b strings.Builder
	i := 0

	for i < len(template) {
		// Look for ${
		if i+1 < len(template) && template[i] == '$' && template[i+1] == '{' {
			end, result := expandExpr(template, i+2, vars)
			b.WriteString(result)
			i = end
		} else {
			b.WriteByte(template[i])
			i++
		}
	}

	return b.String()
}

// expandExpr parses a single ${...} expression starting just after the opening "${".
// It handles three forms:
//   - Simple substitution: ${var} looks up "var" in the vars map.
//   - Default value: ${var:-default} returns the var's value if non-empty, otherwise
//     expands and returns the default text.
//   - Conditional replacement: ${var:+replacement} returns the expanded replacement
//     text if var is non-empty, otherwise returns an empty string.
//   - Field modifier: ${service.field:modifier} treats the entire "service.field:modifier"
//     string as a single key lookup (used for things like ${rails.url:direct}).
//
// Returns two values: the index in template immediately after this expression's closing
// "}", and the expanded string result. If the expression is malformed or unterminated,
// the raw text is returned as a literal.
func expandExpr(template string, start int, vars map[string]string) (int, string) {
	// Extract the variable name (word chars and dots)
	nameEnd := start
	for nameEnd < len(template) && isVarChar(template[nameEnd]) {
		nameEnd++
	}

	if nameEnd >= len(template) {
		// Unterminated — return as literal
		return nameEnd, "${" + template[start:nameEnd]
	}

	varName := template[start:nameEnd]

	switch {
	case template[nameEnd] == '}':
		// ${var}
		return nameEnd + 1, vars[varName]

	case template[nameEnd] == ':':
		// Could be :- (default), :+ (conditional), or a modifier like :direct
		if nameEnd+1 >= len(template) {
			return nameEnd, "${" + varName
		}

		op := template[nameEnd+1]

		if op == '-' {
			// ${var:-default}
			body, end := extractBody(template, nameEnd+2, vars)
			val, ok := vars[varName]
			if ok && val != "" {
				return end, val
			}
			return end, body
		}

		if op == '+' {
			// ${var:+replacement}
			body, end := extractBody(template, nameEnd+2, vars)
			val, ok := vars[varName]
			if ok && val != "" {
				return end, body
			}
			return end, ""
		}

		// Not an operator — treat colon and everything after as part of the var name
		// This handles modifiers like ${rails.url:direct}
		modEnd := nameEnd + 1
		for modEnd < len(template) && isVarChar(template[modEnd]) {
			modEnd++
		}
		if modEnd < len(template) && template[modEnd] == '}' {
			fullName := template[start:modEnd]
			return modEnd + 1, vars[fullName]
		}

		// Unknown format — return as literal
		return nameEnd, "${" + varName

	default:
		// Unknown character after var name — return as literal
		return nameEnd, "${" + varName
	}
}

// extractBody reads the body of a :- or :+ expression, handling nested ${...}
// references. Scans until the closing "}" that matches the opening operator.
// Nested ${...} expressions are expanded recursively (their closing braces are
// consumed by expandExpr, not by this function).
// Note: literal "}" characters inside the body are not supported — they will be
// interpreted as the end of the expression. Use ${var} references for values
// that might contain braces.
func extractBody(template string, start int, vars map[string]string) (string, int) {
	var b strings.Builder
	i := start

	for i < len(template) {
		if template[i] == '}' {
			// Found the closing brace for this expression
			return b.String(), i + 1
		} else if i+1 < len(template) && template[i] == '$' && template[i+1] == '{' {
			// Nested variable — expand it (expandExpr consumes through its own closing })
			end, result := expandExpr(template, i+2, vars)
			b.WriteString(result)
			i = end
		} else {
			b.WriteByte(template[i])
			i++
		}
	}

	// Unterminated — return what we have
	return b.String(), i
}

// isVarChar reports whether c is a valid character in a template variable name.
// Valid characters are letters (a-z, A-Z), digits (0-9), underscores, and dots.
// Dots allow dotted names like "rails.port" for service field references.
func isVarChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '.'
}
