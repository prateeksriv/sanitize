// Package sanitize provides functions for sanitizing text.
package sanitize

import (
	"bytes"
	"html"
	"html/template"
	"io"
	"path"
	"regexp"
	"strings"

	parser "golang.org/x/net/html"
)

// HTMLAllowing sanitizes html, allowing some tags.
// Arrays of allowed tags and allowed attributes may optionally be passed as the second and third arguments.
func HTMLAllowing(s string, args ...[]string) (string, error) {
	var IGNORE_TAGS = []string{"title", "script", "style", "iframe", "frame", "frameset", "noframes", "noembed", "embed", "applet", "object", "base"}
	var DEFAULT_TAGS = []string{"h1", "h2", "h3", "h4", "h5", "h6", "div", "span", "hr", "p", "br", "b", "i", "ol", "ul", "li", "a", "img"}
	var DEFAULT_ATTR = []string{"id", "class", "src", "href", "title", "alt", "name", "rel"}

	allowedTags := DEFAULT_TAGS
	if len(args) > 0 {
		allowedTags = args[0]
	}
	allowedAttributes := DEFAULT_ATTR
	if len(args) > 1 {
		allowedAttributes = args[1]
	}

	// Parse the html
	tokenizer := parser.NewTokenizer(strings.NewReader(s))

	buffer := bytes.NewBufferString("")
	ignore := ""

	for {
		tokenType := tokenizer.Next()
		token := tokenizer.Token()

		switch tokenType {

		case parser.ErrorToken:
			err := tokenizer.Err()
			if err == io.EOF {
				return buffer.String(), nil
			}
			return "", err

		case parser.StartTagToken:

			if len(ignore) == 0 && includes(allowedTags, token.Data) {
				token.Attr = cleanAttributes(token.Attr, allowedAttributes)
				buffer.WriteString(token.String())
			} else if includes(IGNORE_TAGS, token.Data) {
				ignore = token.Data
			}

		case parser.SelfClosingTagToken:

			if len(ignore) == 0 && includes(allowedTags, token.Data) {
				token.Attr = cleanAttributes(token.Attr, allowedAttributes)
				buffer.WriteString(token.String())
			} else if token.Data == ignore {
				ignore = ""
			}

		case parser.EndTagToken:
			if len(ignore) == 0 && includes(allowedTags, token.Data) {
				token.Attr = []parser.Attribute{}
				buffer.WriteString(token.String())
			} else if token.Data == ignore {
				ignore = ""
			}

		case parser.TextToken:
			// We allow text content through, unless ignoring this entire tag and its contents (including other tags)
			if ignore == "" {
				buffer.WriteString(token.String())
			}
		case parser.CommentToken:
			// We ignore comments by default
		case parser.DoctypeToken:
			// We ignore doctypes by default - html5 does not require them and this is intended for sanitizing snippets of text
		default:
			// We ignore unknown token types by default

		}

	}

}

// HTML strips html tags, replace common entities, and escapes <>&;'" in the result.
// Note the returned text may contain entities as it is escaped by HTMLEscapeString, and most entities are not translated.
func HTML(s string) (output string) {

	output = ""

	// Shortcut strings with no tags in them
	if !strings.ContainsAny(s, "<>") {
		output = s
	} else {

		// First remove line breaks etc as these have no meaning outside html tags (except pre)
		// this means pre sections will lose formatting... but will result in less uninentional paras.
		s = strings.Replace(s, "\n", "", -1)

		// Then replace line breaks with newlines, to preserve that formatting
		s = strings.Replace(s, "</p>", "\n", -1)
		s = strings.Replace(s, "<br>", "\n", -1)
		s = strings.Replace(s, "</br>", "\n", -1)
		s = strings.Replace(s, "<br/>", "\n", -1)

		// Walk through the string removing all tags
		b := bytes.NewBufferString("")
		inTag := false
		for _, r := range s {
			switch r {
			case '<':
				inTag = true
			case '>':
				inTag = false
			default:
				if !inTag {
					b.WriteRune(r)
				}
			}
		}
		output = b.String()
	}

	// Remove a few common harmless entities, to arrive at something more like plain text
	output = strings.Replace(output, "&#8216;", "'", -1)
	output = strings.Replace(output, "&#8217;", "'", -1)
	output = strings.Replace(output, "&#8220;", "\"", -1)
	output = strings.Replace(output, "&#8221;", "\"", -1)
	output = strings.Replace(output, "&nbsp;", " ", -1)
	output = strings.Replace(output, "&quot;", "\"", -1)
	output = strings.Replace(output, "&apos;", "'", -1)

	// Translate some entities into their plain text equivalent (for example accents, if encoded as entities)
	output = html.UnescapeString(output)

	// In case we have missed any tags above, escape the text - removes <, >, &, ' and ".
	output = template.HTMLEscapeString(output)

	// After processing, remove some harmless entities &, ' and " which are encoded by HTMLEscapeString
	output = strings.Replace(output, "&#34;", "\"", -1)
	output = strings.Replace(output, "&#39;", "'", -1)
	output = strings.Replace(output, "&amp; ", "& ", -1)     // NB space after
	output = strings.Replace(output, "&amp;amp; ", "& ", -1) // NB space after

	return output
}

// We are very restrictive as this is intended for ascii url slugs
var illegalPath = regexp.MustCompile(`[^[:alnum:]\~\-\./]`)

// Path makes a string safe to use as an url path.
func Path(text string) string {
	// Start with lowercase string
	filePath := strings.ToLower(text)
	filePath = strings.Replace(filePath, "..", "", -1)
	filePath = path.Clean(filePath)

	// Remove illegal characters for paths, flattening accents and replacing some common separators with -
	filePath = cleanString(filePath, illegalPath)

	// NB this may be of length 0, caller must check
	return filePath
}

// Remove all other unrecognised characters apart from
var illegalName = regexp.MustCompile(`[^[:alnum:]-.]`)

// Name makes a string safe to use in a file name.
func Name(text string) string {
	// Start with lowercase string
	fileName := strings.ToLower(text)
	fileName = path.Clean(path.Base(fileName))

	// Remove illegal characters for names, replacing some common separators with -
	fileName = cleanString(fileName, illegalName)

	// NB this may be of length 0, caller must check
	return fileName
}

// A very limited list of transliterations to catch common european names translated to urls.
// This set could be expanded with at least caps and many more characters.
var transliterations = map[rune]string{
	'À': "A",
	'Á': "A",
	'Â': "A",
	'Ã': "A",
	'Ä': "A",
	'Å': "AA",
	'Æ': "AE",
	'Ç': "C",
	'È': "E",
	'É': "E",
	'Ê': "E",
	'Ë': "E",
	'Ì': "I",
	'Í': "I",
	'Î': "I",
	'Ï': "I",
	'Ð': "D",
	'Ł': "L",
	'Ñ': "N",
	'Ò': "O",
	'Ó': "O",
	'Ô': "O",
	'Õ': "O",
	'Ö': "O",
	'Ø': "OE",
	'Ù': "U",
	'Ú': "U",
	'Ü': "U",
	'Û': "U",
	'Ý': "Y",
	'Þ': "Th",
	'ß': "ss",
	'à': "a",
	'á': "a",
	'â': "a",
	'ã': "a",
	'ä': "a",
	'å': "aa",
	'æ': "ae",
	'ç': "c",
	'è': "e",
	'é': "e",
	'ê': "e",
	'ë': "e",
	'ì': "i",
	'í': "i",
	'î': "i",
	'ï': "i",
	'ð': "d",
	'ł': "l",
	'ñ': "n",
	'ń': "n",
	'ò': "o",
	'ó': "o",
	'ô': "o",
	'õ': "o",
	'ō': "o",
	'ö': "o",
	'ø': "oe",
	'ś': "s",
	'ù': "u",
	'ú': "u",
	'û': "u",
	'ū': "u",
	'ü': "u",
	'ý': "y",
	'þ': "th",
	'ÿ': "y",
	'ż': "z",
	'Œ': "OE",
	'œ': "oe",
}

// Accents replaces a set of accented characters with ascii equivalents.
func Accents(text string) string {
	// Replace some common accent characters
	b := bytes.NewBufferString("")
	for _, c := range text {
		// Check transliterations first
		if val, ok := transliterations[c]; ok {
			b.WriteString(val)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// If the attribute contains data: or javascript: anywhere, ignore it
// we don't allow this in attributes as it is so frequently used for xss
// NB we allow spaces in the value, and lowercase.
var illegalAttr = regexp.MustCompile(`(d\s*a\s*t\s*a|j\s*a\s*v\s*a\s*s\s*c\s*r\s*i\s*p\s*t\s*)\s*:`)

// We are far more restrictive with href attributes.
var legalHrefAttr = regexp.MustCompile(`\A/[^/\\]?|mailto://|http://|https://`)

// cleanAttributes returns an array of attributes after removing malicious ones.
func cleanAttributes(a []parser.Attribute, allowed []string) []parser.Attribute {
	if len(a) == 0 {
		return a
	}

	cleaned := make([]parser.Attribute, 0)
	for _, attr := range a {
		if includes(allowed, attr.Key) {

			val := strings.ToLower(attr.Val)

			// Check for illegal attribute values
			if illegalAttr.FindString(val) != "" {
				attr.Val = ""
			}

			// Check for legal href values - / mailto:// http:// or https://
			if attr.Key == "href" {
				if legalHrefAttr.FindString(val) == "" {
					attr.Val = ""
				}
			}

			// If we still have an attribute, append it to the array
			if attr.Val != "" {
				cleaned = append(cleaned, attr)
			}
		}
	}
	return cleaned
}

// A list of characters we consider separators in normal strings and replace with our canonical separator - rather than removing.
var separators = regexp.MustCompile(`[ &_=+:]`)

// cleanString replaces separators with - and removes characters listed in the regexp provided from string.
// Accents, spaces, and all characters not in A-Za-z0-9 are replaced.
func cleanString(s string, r *regexp.Regexp) string {

	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")

	// Flatten accents first so that if we remove non-ascii we still get a legible name
	s = Accents(s)

	// Replace certain joining characters with a dash
	s = separators.ReplaceAllString(s, "-")

	// Remove all other unrecognised characters - NB we do allow any printable characters
	s = r.ReplaceAllString(s, "")

	// Remove any double dashes caused by existing - in name
	s = strings.Replace(s, "--", "-", -1)

	return s
}

// includes checks for inclusion of a string in a []string.
func includes(a []string, s string) bool {
	for _, as := range a {
		if as == s {
			return true
		}
	}
	return false
}
