package main

import (
	"regexp"
)

var mdImageRegex = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
var htmlImageRegex = regexp.MustCompile(`(?i)<img[^>]+>`)

// stripMarkdownImages removes markdown image syntax ![alt](url) and HTML <img> tags
// from the text and replaces it with [Image Removed] to save context window.
func stripMarkdownImages(text string) string {
	text = mdImageRegex.ReplaceAllString(text, "[Image Removed]")
	return htmlImageRegex.ReplaceAllString(text, "[Image Removed]")
}
