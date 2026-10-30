package service

import "github.com/Bobby-P-dev/go-diagram.git/internal/modules/artifact"

type Service struct {
	repo artifact.Repository
}

var _ ArtifactService = (*Service)(nil)

func NewService(repo artifact.Repository) *Service {
	return &Service{repo: repo}
}
