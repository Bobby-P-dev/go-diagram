package service

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

func isVoidHTMLElement(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}

func findHTMLTarget(source, targetID string, semanticFallback bool) (string, int, int, error) {
	if targetID == "" {
		return "", -1, -1, fmt.Errorf("empty HTML target")
	}
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	offset := 0
	plainIDStart, semanticStart := -1, -1
	for {
		kind := tokenizer.Next()
		start := offset
		offset += len(tokenizer.Raw())
		if kind == html.ErrorToken {
			break
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		for _, attribute := range token.Attr {
			if attribute.Key == "data-rl-id" && attribute.Val == targetID {
				return findBalancedTagFromIndex(source, start)
			}
			if plainIDStart < 0 && attribute.Key == "id" && attribute.Val == targetID {
				plainIDStart = start
			}
		}
		if semanticFallback && semanticStart < 0 && (targetID == token.Data || targetID == "sec-"+token.Data) && (token.Data == "header" || token.Data == "footer") {
			semanticStart = start
		}
	}
	if plainIDStart >= 0 {
		return findBalancedTagFromIndex(source, plainIDStart)
	}
	if semanticStart >= 0 {
		return findBalancedTagFromIndex(source, semanticStart)
	}
	return "", -1, -1, fmt.Errorf("target %q not found in HTML", targetID)
}

func findBalancedTagFromIndex(source string, start int) (string, int, int, error) {
	if start < 0 || start >= len(source) || source[start] != '<' {
		return "", -1, -1, fmt.Errorf("invalid start tag index %d", start)
	}
	tokenizer := html.NewTokenizer(strings.NewReader(source[start:]))
	var stack []string
	offset := start
	first := true
	for {
		kind := tokenizer.Next()
		offset += len(tokenizer.Raw())
		if kind == html.ErrorToken {
			return "", -1, -1, fmt.Errorf("incomplete element at index %d", start)
		}
		if first && kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			return "", -1, -1, fmt.Errorf("expected opening tag at index %d", start)
		}
		first = false
		token := tokenizer.Token()
		if err := trackHTMLStructure(&stack, kind, token.Data); err != nil {
			return "", -1, -1, err
		}
		if len(stack) == 0 {
			return source[start:offset], start, offset, nil
		}
	}
}

func validateHTMLStructure(source string) error {
	tokenizer := html.NewTokenizer(strings.NewReader(source))
	var stack []string
	for {
		kind := tokenizer.Next()
		if kind == html.ErrorToken {
			if tokenizer.Err() != io.EOF || len(tokenizer.Raw()) > 0 || len(stack) > 0 {
				return fmt.Errorf("incomplete HTML implementation")
			}
			return nil
		}
		token := tokenizer.Token()
		if err := trackHTMLStructure(&stack, kind, token.Data); err != nil {
			return err
		}
	}
}

func trackHTMLStructure(stack *[]string, kind html.TokenType, tag string) error {
	switch kind {
	case html.StartTagToken:
		if !isVoidHTMLElement(tag) {
			*stack = append(*stack, tag)
		}
	case html.EndTagToken:
		if isVoidHTMLElement(tag) {
			return nil
		}
		if len(*stack) == 0 || (*stack)[len(*stack)-1] != tag {
			return fmt.Errorf("unbalanced closing HTML tag </%s>", tag)
		}
		*stack = (*stack)[:len(*stack)-1]
	}
	return nil
}
