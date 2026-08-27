package config

// MergeBool resolves a boolean value using the precedence:
// explicit CLI flag > environment variable > config file > fallback default.
func MergeBool(explicit bool, explicitSet bool, fallback bool, sources ...*bool) bool {
	if explicitSet {
		return explicit
	}
	for _, s := range sources {
		if s != nil {
			return *s
		}
	}
	return fallback
}

// MergeString resolves a string value using the precedence:
// explicit CLI flag > environment variable > config file > fallback default.
func MergeString(explicit string, explicitSet bool, fallback string, sources ...*string) string {
	if explicitSet {
		return explicit
	}
	for _, s := range sources {
		if s != nil {
			return *s
		}
	}
	return fallback
}

// MergeStringSlice resolves a string slice using the precedence:
// explicit CLI flag > environment variable > config file. The first non-empty
// source wins. Unlike the scalar helpers, there is no built-in fallback value.
func MergeStringSlice(explicit []string, explicitSet bool, sources ...[]string) []string {
	if explicitSet {
		return explicit
	}
	for _, s := range sources {
		if len(s) > 0 {
			return s
		}
	}
	return nil
}
