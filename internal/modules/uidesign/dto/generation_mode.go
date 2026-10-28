package dto

import "errors"

var ErrStaticUITemplateUnsupported = errors.New("template_id is no longer supported for UI Design; provide a prompt instead")

func (req CreateUIDesignRequest) ValidateGenerationMode() error {
	if req.TemplateID != "" {
		return ErrStaticUITemplateUnsupported
	}
	return nil
}
