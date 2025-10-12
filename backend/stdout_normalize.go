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

// normalizeLineEndings maps CRLF/CR to LF for easier comparisons.
func normalizeLineEndings(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// interpretCommonEscapes expands simple escape sequences (e.g., \n, \t) found in literals.
func interpretCommonEscapes(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	replacer := strings.NewReplacer(
		`\r\n`, "\n",
		`\n`, "\n",
		`\r`, "\n",
		`\t`, "\t",
	)
	return replacer.Replace(s)
}

func normalizeActualStdout(s string) string {
	return normalizeLineEndings(s)
}

func normalizeExpectedStdout(s string) string {
	return interpretCommonEscapes(normalizeLineEndings(s))
}
