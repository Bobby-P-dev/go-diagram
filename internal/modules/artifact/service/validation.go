package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"
)

func computeRequestHash(payload interface{}) string {
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validateActor(userID string) error {
	if !artifact.ValidID(userID) {
		return artifact.ErrUnauthenticated
	}
	return nil
}

func validateID(value string) error {
	if !artifact.ValidID(value) {
		return fmt.Errorf("%w: malformed resource ID", artifact.ErrInvalidInput)
	}
	return nil
}

func pageQuery(query artifact.PageQuery) (artifact.PageQuery, error) {
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 100 || len(query.Cursor) > 2048 {
		return query, artifact.ErrInvalidInput
	}
	return query, nil
}

func normalizeRevisionContext(schemaVersion int, triggerMessageID *string, sources []artifact.RevisionRef) (int, *string, []artifact.RevisionRef, error) {
	if schemaVersion == 0 {
		schemaVersion = 1
	}
	if schemaVersion != 1 {
		return 0, nil, nil, artifact.ErrContentInvalid
	}
	if triggerMessageID != nil {
		normalized := strings.ToLower(strings.TrimSpace(*triggerMessageID))
		if err := validateID(normalized); err != nil {
			return 0, nil, nil, err
		}
		triggerMessageID = &normalized
	}
	if len(sources) > 100 {
		return 0, nil, nil, artifact.ErrInvalidInput
	}
	normalizedSources := make([]artifact.RevisionRef, 0, len(sources))
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		source.ArtifactID = strings.ToLower(strings.TrimSpace(source.ArtifactID))
		source.RevisionID = strings.ToLower(strings.TrimSpace(source.RevisionID))
		if !artifact.ValidID(source.ArtifactID) || !artifact.ValidID(source.RevisionID) || seen[source.ArtifactID] {
			return 0, nil, nil, artifact.ErrInvalidInput
		}
		if source.LinkType == "" {
			source.LinkType = "context_source"
		}
		if source.LinkType != "context_source" && source.LinkType != "derived_from" {
			return 0, nil, nil, artifact.ErrInvalidInput
		}
		seen[source.ArtifactID] = true
		normalizedSources = append(normalizedSources, source)
	}
	sort.Slice(normalizedSources, func(first, second int) bool {
		return normalizedSources[first].ArtifactID < normalizedSources[second].ArtifactID
	})
	return schemaVersion, triggerMessageID, normalizedSources, nil
}
