package service

import (
	"fmt"

	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/diagram/dto"
)

func (service *DiagramService) ListTemplates() ([]dto.TemplateDTO, error) {
	if service.templates == nil {
		return nil, fmt.Errorf("template service is not configured")
	}

	templates, err := service.templates.GetAllTemplates()
	if err != nil {
		return nil, err
	}

	result := make([]dto.TemplateDTO, 0, len(templates))
	for _, t := range templates {
		result = append(result, dto.TemplateDTO{
			ID:          t.ID,
			Name:        t.Name,
			Category:    t.Category,
			Description: t.Description,
			DiagramType: t.DiagramType,
			Nodes:       t.Nodes,
			Edges:       t.Edges,
			IsFeatured:  t.IsFeatured,
		})
	}
	return result, nil
}
