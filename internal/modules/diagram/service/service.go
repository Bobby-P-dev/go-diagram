package service

import "github.com/Bobby-P-dev/go-diagram.git/internal/platform/aiusage"

type DiagramService struct {
	ai        AIClient
	projects  ProjectRepository
	messages  MessageRepository
	versions  VersionRepository
	usage     aiusage.Recorder
	templates TemplateRepository
}

func NewDiagramService(ai AIClient, projects ProjectRepository, messages MessageRepository, versions VersionRepository, usage aiusage.Recorder, templates TemplateRepository) *DiagramService {
	return &DiagramService{ai: ai, projects: projects, messages: messages, versions: versions, usage: usage, templates: templates}
}
