package artifact

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	typePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,49}$`)
)

func ValidID(value string) bool {
	return uuidPattern.MatchString(value) && value != "00000000-0000-0000-0000-000000000000"
}

func ResolveFormat(artifactType string) (string, error) {
	artifactType = strings.TrimSpace(strings.ToLower(artifactType))
	if !typePattern.MatchString(artifactType) {
		return "", fmt.Errorf("%w: invalid artifact type", ErrInvalidInput)
	}
	switch artifactType {
	case "brainstorm", "prd":
		return "markdown", nil
	case "wireframe", "ui_design":
		return "frame", nil
	default:
		return "graph", nil
	}
}

func Slugify(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	var builder strings.Builder
	lastHyphen := true
	for _, character := range lower {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteRune(character)
			lastHyphen = false
		} else if !lastHyphen {
			builder.WriteByte('-')
			lastHyphen = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		slug = "artifact"
	}
	if len(slug) > 100 {
		slug = slug[:100]
		slug = strings.Trim(slug, "-")
	}
	return slug
}

func ValidateSlug(slug string) error {
	if len(slug) < 1 || len(slug) > 100 || !slugPattern.MatchString(slug) {
		return fmt.Errorf("%w: invalid slug format, must be 1..100 lowercase alphanumeric words separated by single hyphens", ErrInvalidInput)
	}
	return nil
}

func ValidateContent(format string, doc *string, nodes, edges, frame json.RawMessage) error {
	if doc != nil {
		if err := ValidateText(*doc, 1000000, true); err != nil {
			return ErrContentInvalid
		}
	}
	switch format {
	case "markdown":
		if doc == nil || strings.TrimSpace(*doc) == "" {
			return fmt.Errorf("%w: markdown document content is required", ErrContentInvalid)
		}
		if len(nodes) > 0 || len(edges) > 0 || len(frame) > 0 {
			return fmt.Errorf("%w: markdown artifact must not contain graph or frame payloads", ErrContentInvalid)
		}
	case "graph":
		if doc != nil || len(frame) > 0 {
			return fmt.Errorf("%w: graph artifact must not contain document or frame payloads", ErrContentInvalid)
		}
		if len(nodes) == 0 || !isArrayJSON(nodes) {
			return fmt.Errorf("%w: graph_nodes must be a JSON array", ErrContentInvalid)
		}
		if len(edges) == 0 || !isArrayJSON(edges) {
			return fmt.Errorf("%w: graph_edges must be a JSON array", ErrContentInvalid)
		}
	case "frame":
		if doc != nil || len(nodes) > 0 || len(edges) > 0 {
			return fmt.Errorf("%w: frame artifact must not contain document or root graph payloads", ErrContentInvalid)
		}
		if len(frame) == 0 || !isObjectJSON(frame) {
			return fmt.Errorf("%w: frame_data must be a JSON object", ErrContentInvalid)
		}
		var frameMap map[string]json.RawMessage
		if err := json.Unmarshal(frame, &frameMap); err != nil {
			return fmt.Errorf("%w: invalid frame_data json", ErrContentInvalid)
		}
		if !isArrayJSON(frameMap["nodes"]) || !isArrayJSON(frameMap["edges"]) {
			return fmt.Errorf("%w: frame_data must contain 'nodes' and 'edges' arrays", ErrContentInvalid)
		}
	default:
		return fmt.Errorf("%w: unsupported format: %s", ErrContentInvalid, format)
	}
	return nil
}
func isArrayJSON(data json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(data))
	return json.Valid(data) && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
}

func isObjectJSON(data json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(data))
	return json.Valid(data) && strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}

func ValidateKey(key string) error {
	if len(key) < 1 || len(key) > 128 {
		return fmt.Errorf("%w: Idempotency-Key must contain 1..128 printable ASCII characters", ErrInvalidInput)
	}
	for _, char := range key {
		if char < '!' || char > '~' {
			return fmt.Errorf("%w: invalid Idempotency-Key", ErrInvalidInput)
		}
	}
	return nil
}

func ValidateText(value string, maximum int, required bool) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') || utf8.RuneCountInString(value) > maximum || (required && strings.TrimSpace(value) == "") {
		return fmt.Errorf("%w: text is empty, too long or invalid", ErrInvalidInput)
	}
	return nil
}
