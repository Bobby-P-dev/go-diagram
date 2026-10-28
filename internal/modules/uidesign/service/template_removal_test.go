package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	dto "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

func TestUITemplateRequestsFailBeforePersistenceOrDispatch(t *testing.T) {
	service := NewUIDesignService(nil, nil, nil)
	dispatcher := NewJobDispatcher(JobDispatcherConfig{})
	for _, prompt := range []string{"", "Create a bespoke bookstore"} {
		t.Run(prompt, func(t *testing.T) {
			request := dto.CreateUIDesignRequest{TemplateID: "old-template", Prompt: prompt}
			if result, err := service.GenerateUIDesign(context.Background(), request); result != nil || !errors.Is(err, dto.ErrStaticUITemplateUnsupported) {
				t.Fatalf("sync generation did not reject template: result=%v err=%v", result, err)
			}
			if result, err := service.GenerateUIDesignStream(context.Background(), request, func(string, any) {
				t.Error("invalid request must not emit stream events")
			}); result != nil || !errors.Is(err, dto.ErrStaticUITemplateUnsupported) {
				t.Fatalf("stream generation did not reject template: result=%v err=%v", result, err)
			}
			if result, err := dispatcher.DispatchUIDesignJob(context.Background(), request, "owner"); result != nil || !errors.Is(err, dto.ErrStaticUITemplateUnsupported) {
				t.Fatalf("async dispatch did not reject template: result=%v err=%v", result, err)
			}
		})
	}
}

func TestPatchDoesNotSynthesizeStaticHTML(t *testing.T) {
	frame := dto.UIFrameData{
		Title: "Legacy frame",
		Sections: []dto.UISectionDTO{
			{ID: "sec-hero", Type: "hero", Data: map[string]interface{}{"title": "User content"}},
		},
	}
	original, _ := json.Marshal(frame)
	result, _, err := NewPatchEngine().ApplyPatch(&frame, &dto.ChangePlanDTO{})
	if result != nil || err == nil || !strings.Contains(err.Error(), "regenerate") {
		t.Fatalf("expected an explicit regeneration error, got result=%v err=%v", result, err)
	}
	after, _ := json.Marshal(frame)
	if string(after) != string(original) {
		t.Fatal("failed patch mutated the existing frame")
	}
}

func TestPatchPreservesGeneratedHTMLWithoutTemplate(t *testing.T) {
	html := `<main data-rl-id="sec-custom"><h1>Custom bookstore</h1></main>`
	frame := dto.UIFrameData{Title: "Bookstore", RawHtml: html}
	result, _, err := NewPatchEngine().ApplyPatch(&frame, &dto.ChangePlanDTO{})
	if err != nil {
		t.Fatal(err)
	}
	if result.RawHtml != html || result.CodeExport["html"] != html {
		t.Fatal("generated HTML must not be replaced with a static layout")
	}
}
