// Package reputation provides domain sanitization (typo fixing) for email addresses.
// This is Capa 2, Step 2 of the Anti-Bounce Engine: catch classic counter-entry
// mistakes before they burn a send attempt or damage domain reputation.
package reputation

import (
	"net/mail"
	"strings"
)

// SanitizeResult holds the outcome of sanitizing a single email address.
type SanitizeResult struct {
	Original  string
	Fixed     string
	WasFixed  bool
	IsInvalid bool // True if the email is malformed beyond fixable typos
}

// domainCorrections maps known typos to their canonical form.
// Keys must be lowercase. Values are the correct domain.
var domainCorrections = map[string]string{
	// Gmail variants
	"gmai.com":        "gmail.com",
	"gmial.com":       "gmail.com",
	"gmal.com":        "gmail.com",
	"gnail.com":       "gmail.com",
	"gamil.com":       "gmail.com",
	"gmaill.com":      "gmail.com",
	"gmil.com":        "gmail.com",
	"gmail.com.ar":    "gmail.com",
	"gmail.con":       "gmail.com",
	"gmail.cpm":       "gmail.com",
	"gmail.cob":       "gmail.com",
	"gmail.co":        "gmail.com",
	// Hotmail variants
	"hotmial.com":     "hotmail.com",
	"homtail.com":     "hotmail.com",
	"hotamil.com":     "hotmail.com",
	"hotmail.con":     "hotmail.com",
	"hotmail.cpm":     "hotmail.com",
	"hotmail.co":      "hotmail.com",
	"hotmail.com.ar":  "hotmail.com",
	"hotmali.com":     "hotmail.com",
	"hotmai.com":      "hotmail.com",
	// Outlook variants
	"outlok.com":      "outlook.com",
	"outllok.com":     "outlook.com",
	"outlook.con":     "outlook.com",
	// Yahoo variants
	"yaho.com":        "yahoo.com",
	"yahooo.com":      "yahoo.com",
	"yahoo.con":       "yahoo.com",
	"yahoo.com.ar":    "yahoo.com",
	// Live variants
	"live.com.ar":     "live.com",
	// Fibertel (Argentina)
	"fibertel.com":    "fibertel.com.ar",
	// Arnet (Argentina)
	"arnet.com":       "arnet.com.ar",
}

// Sanitize cleans and attempts to fix a single email address.
// It normalizes case, strips invisible characters, fixes double dots,
// trims trailing punctuation, and corrects known domain typos.
func Sanitize(raw string) SanitizeResult {
	original := raw

	// Step 1: Normalize — trim whitespace, lowercase
	email := strings.TrimSpace(raw)
	email = strings.ToLower(email)

	// Step 2: Remove invisible/zero-width Unicode chars (common from ERP exports)
	email = removeInvisibleChars(email)

	// Step 3: Strip trailing punctuation that shouldn't be there
	email = strings.TrimRight(email, ".,;: ")

	// Step 4: Remove spaces inside the email
	email = strings.ReplaceAll(email, " ", "")

	// Step 5: Fix double dots (e.g. "juan..perez@gmail.com")
	for strings.Contains(email, "..") {
		email = strings.ReplaceAll(email, "..", ".")
	}

	// Step 6: Split on @ and attempt domain correction
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return SanitizeResult{Original: original, Fixed: email, WasFixed: false, IsInvalid: true}
	}

	local, domain := parts[0], parts[1]

	// Step 7: Correct known domain typos
	if corrected, ok := domainCorrections[domain]; ok {
		domain = corrected
	}

	fixed := local + "@" + domain

	// Step 8: Final RFC 5322 validation
	if _, err := mail.ParseAddress(fixed); err != nil {
		return SanitizeResult{Original: original, Fixed: fixed, WasFixed: fixed != original, IsInvalid: true}
	}

	return SanitizeResult{
		Original: original,
		Fixed:    fixed,
		WasFixed: fixed != original,
	}
}

// removeInvisibleChars strips Unicode characters that are invisible but break email parsing.
// Commonly injected by Windows clipboard pastes, ERP exports, or spreadsheet copies.
func removeInvisibleChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		// Zero-width space, non-breaking space, zero-width no-break space, BOM
		case '\u200B', '\u00A0', '\uFEFF', '\u200C', '\u200D', '\u2060':
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
