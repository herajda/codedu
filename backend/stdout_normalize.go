package main

import "strings"

// trimTrailingNewline removes a single trailing newline sequence (including CRLF) without touching other whitespace.
func trimTrailingNewline(out string) string {
	if strings.HasSuffix(out, "\r\n") {
		return strings.TrimSuffix(out, "\r\n")
	}
	if strings.HasSuffix(out, "\n") {
		return strings.TrimSuffix(out, "\n")
	}
	if strings.HasSuffix(out, "\r") {
		return strings.TrimSuffix(out, "\r")
	}
	return out
}
