package loneanalyzer

import (
	"regexp"
	"strings"
)

var (
	snakeCaseRegex  = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	allCapsRegex    = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	camelCaseRegex  = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*$`)
	intrinsicRegex  = regexp.MustCompile(`^#[a-z_][a-z0-9_]*$`)
)

// IsSnakeCase checks if an identifier follows snake_case (e.g. `foo_bar`, `counter`, `_tmp`).
func IsSnakeCase(ident string) bool {
	return snakeCaseRegex.MatchString(ident)
}

// IsAllCaps checks if an identifier follows ALL_CAPS (e.g. `MAX_COUNT`, `DEFAULT_PORT`).
func IsAllCaps(ident string) bool {
	return allCapsRegex.MatchString(ident)
}

// IsCamelCase checks if an identifier follows CamelCase / PascalCase (e.g. `Option`, `HttpServer`).
func IsCamelCase(ident string) bool {
	return camelCaseRegex.MatchString(ident)
}

// HasTraitSuffix checks if a trait type identifier ends with the "Trait" suffix (e.g. "ComparableTrait").
func HasTraitSuffix(name string) bool {
	return strings.HasSuffix(name, "Trait")
}

// EndsWithTraitOrNamedTrait returns true if an identifier ends with "trait" (case-insensitive)
// or is named "Trait" / "trait", while ignoring unrelated English words like "portrait" or "strait".
func EndsWithTraitOrNamedTrait(name string) bool {
	lower := strings.ToLower(name)
	if lower == "trait" {
		return true
	}
	if !strings.HasSuffix(lower, "trait") {
		return false
	}
	// Exclude legitimate words that end with "trait" but are not referring to traits
	if strings.HasSuffix(lower, "portrait") || strings.HasSuffix(lower, "strait") || strings.HasSuffix(lower, "distrait") {
		return false
	}
	return true
}

// IsIntrinsic checks if an identifier is an intrinsic prefixed with '#' (e.g. `#panic`, `#print`).
func IsIntrinsic(ident string) bool {
	return intrinsicRegex.MatchString(ident)
}

// IsPrimitiveType returns true if the type identifier matches standard built-in primitives or compiler placeholders.
func IsPrimitiveType(name string) bool {
	switch name {
	case "i8", "i16", "i32", "i64", "i128",
		"u8", "u16", "u32", "u64", "u128",
		"f32", "f64",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float", "float32", "float64",
		"bool", "str", "string", "char", "void", "any", "infer",
		"ptr", "ref", "Self":
		return true
	default:
		return false
	}
}
