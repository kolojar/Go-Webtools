package debugtools

import (
	"fmt"
	"strings"
)

// MapToString converts map to string
func MapToString[K comparable, V any](m map[K]V) string {
	result := "{"
	for k, v := range m {
		result += "[" + fmt.Sprint(k) + ", " + fmt.Sprint(v) + "], "
	}
	result = strings.TrimSuffix(result, ", ")
	result += "}"
	return result
}
