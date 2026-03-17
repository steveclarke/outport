package config

import "strings"

// ExpandVars performs bash-style parameter expansion on a template string.
//
// Supported syntax:
//   - ${var}            — substitute the value of var
//   - ${var:-default}   — use default if var is empty or unset
//   - ${var:+replacement} — use replacement if var is non-empty; empty otherwise
//
// Variable references (${var}) inside default/replacement text are also expanded.
// Variable names may contain word characters and dots (e.g., "rails.port").
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

// expandExpr parses a ${...} expression starting just after the opening "${".
// Returns the index after the closing "}" and the expanded result.
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
// references and brace depth. Returns the expanded body and index after the closing "}".
func extractBody(template string, start int, vars map[string]string) (string, int) {
	var b strings.Builder
	i := start
	depth := 1

	for i < len(template) && depth > 0 {
		if template[i] == '}' {
			depth--
			if depth == 0 {
				break
			}
			b.WriteByte('}')
			i++
		} else if i+1 < len(template) && template[i] == '$' && template[i+1] == '{' {
			// Nested variable — expand it
			end, result := expandExpr(template, i+2, vars)
			b.WriteString(result)
			i = end
		} else {
			b.WriteByte(template[i])
			i++
		}
	}

	if i < len(template) {
		i++ // skip closing }
	}

	return b.String(), i
}

func isVarChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_' || c == '.'
}
