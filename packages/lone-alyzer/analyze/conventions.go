package analyze

import "regexp"

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
		"bool", "str", "string", "char", "void", "any", "infer":
		return true
	default:
		return false
	}
}
