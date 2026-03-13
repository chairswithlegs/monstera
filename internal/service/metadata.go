package service

import (
	"encoding/json"
	"unicode"
)

const maxMetadataValueLen = 2048

// encodeMetadata converts a string map into valid JSON bytes for audit metadata.
// Values are sanitized (length-limited and control characters stripped) so that
// the result is always valid, safe JSON. If encoding fails, returns []byte("{}").
func encodeMetadata(m map[string]string) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}
	sanitized := make(map[string]string, len(m))
	for k, v := range m {
		sanitized[k] = sanitizeMetadataValue(v)
	}
	b, err := json.Marshal(sanitized)
	if err != nil {
		return []byte("{}")
	}
	return b
}

// sanitizeMetadataValue limits length and strips control characters so the value
// is safe to embed in JSON and does not grow unbounded.
func sanitizeMetadataValue(s string) string {
	var out []rune
	for _, r := range s {
		if len(out) >= maxMetadataValueLen {
			break
		}
		if r == unicode.ReplacementChar || unicode.IsControl(r) {
			continue
		}
		out = append(out, r)
	}
	if len(out) == 0 && len(s) > 0 {
		return "[invalid]"
	}
	return string(out)
}
