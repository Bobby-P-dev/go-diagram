package service

import (
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

func sanitizeStaticHTML(source string) string {
	source = strings.TrimSpace(source)
	for _, fence := range []string{"```html", "```HTML", "```"} {
		if strings.HasPrefix(source, fence) {
			source = strings.TrimPrefix(source, fence)
			source = strings.TrimSuffix(strings.TrimSpace(source), "```")
			break
		}
	}
	var output strings.Builder
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	blocked := ""
	depth := 0
	bodyWrapper := false
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if blocked == "" {
				output.Write(tokenizer.Raw())
			}
			break
		}
		raw := string(tokenizer.Raw())
		token := tokenizer.Token()
		if blocked != "" {
			if token.Data == blocked {
				if kind == html.StartTagToken {
					depth++
				} else if kind == html.EndTagToken {
					depth--
					if depth == 0 {
						blocked = ""
					}
				}
			}
			continue
		}
		switch kind {
		case html.DoctypeToken, html.CommentToken:
			continue
		case html.StartTagToken, html.SelfClosingTagToken:
			switch token.Data {
			case "script", "iframe", "object", "head", "template", "noscript", "foreignobject", "math", "xmp", "plaintext", "noembed", "noframes", "animate", "animatemotion", "animatetransform", "set":
				if kind != html.SelfClosingTagToken {
					blocked, depth = token.Data, 1
				}
				continue
			case "embed", "base", "meta", "link":
				continue
			case "html":
				continue
			}
			changed := sanitizeHTMLAttributes(&token)
			if token.Data == "body" {
				if len(token.Attr) == 0 {
					continue
				}
				token.Data = "div"
				token.DataAtom = 0
				bodyWrapper = true
				changed = true
			}
			if changed {
				output.WriteString(token.String())
			} else {
				output.WriteString(raw)
			}
		case html.EndTagToken:
			switch token.Data {
			case "html", "head", "script", "iframe", "object", "embed", "template", "noscript", "foreignobject", "math", "xmp", "plaintext", "noembed", "noframes", "animate", "animatemotion", "animatetransform", "set", "base", "meta", "link":
				continue
			case "body":
				if bodyWrapper {
					output.WriteString("</div>")
					bodyWrapper = false
				}
				continue
			}
			output.WriteString(raw)
		default:
			output.WriteString(raw)
		}
	}
	return strings.TrimSpace(output.String())
}

func sanitizeHTMLAttributes(token *html.Token) bool {
	changed := false
	attributes := make([]html.Attribute, 0, len(token.Attr))
	for _, attribute := range token.Attr {
		key := strings.ToLower(attribute.Key)
		if strings.HasPrefix(key, "on") || strings.HasPrefix(key, "v-") || strings.HasPrefix(key, "@") || strings.HasPrefix(key, ":") || key == "srcdoc" || key == "is" {
			changed = true
			continue
		}
		switch key {
		case "href", "src", "xlink:href", "action", "formaction", "poster", "background", "cite", "data":
			if !safeStaticURL(attribute.Val) {
				attribute.Val = "#"
				changed = true
			}
		case "srcset", "imagesrcset":
			for _, candidate := range strings.Split(attribute.Val, ",") {
				parts := strings.Fields(candidate)
				if len(parts) > 0 && !safeStaticURL(parts[0]) {
					attribute.Val = ""
					changed = true
					break
				}
			}
		}
		attributes = append(attributes, attribute)
	}
	token.Attr = attributes
	return changed
}

func safeStaticURL(value string) bool {
	value = strings.Map(func(character rune) rune {
		if character <= 0x20 || character == 0x7f {
			return -1
		}
		return character
	}, value)
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "", "https", "http", "mailto", "tel":
		return true
	default:
		return false
	}
}

func replaceTextContentOnly(source string, pattern *regexp.Regexp, replacement string) string {
	if pattern == nil {
		return source
	}
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	var output strings.Builder
	inStyle := false
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			output.Write(tokenizer.Raw())
			break
		}
		raw := string(tokenizer.Raw())
		token := tokenizer.Token()
		if kind == html.StartTagToken && token.Data == "style" {
			inStyle = true
		} else if kind == html.EndTagToken && token.Data == "style" {
			inStyle = false
		} else if kind == html.TextToken && !inStyle {
			raw = pattern.ReplaceAllString(raw, replacement)
		}
		output.WriteString(raw)
	}
	return output.String()
}
