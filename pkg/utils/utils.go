package utils

import "encoding/base64"

// GetHTMLString : base64 -> string
func GetHTMLString(base string) string {
	content, _ := base64.StdEncoding.DecodeString(base)
	return string(content)
}
