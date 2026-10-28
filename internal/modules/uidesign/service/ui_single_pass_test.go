package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	dtos "github.com/Bobby-P-dev/go-diagram.git/internal/modules/uidesign/dto"
)

type singlePassClient struct {
	*bespokeMockClient
	methods   []string
	prompts   []string
	failure   error
	afterCall func()
}

func (client *singlePassClient) CallLLM(systemPrompt, prompt string) (string, error) {
	client.methods = append(client.methods, "json")
	client.prompts = append(client.prompts, prompt)
	if systemPrompt != synthesizePrompt {
		client.test.Fatal("unexpected generation system prompt")
	}
	if client.afterCall != nil {
		client.afterCall()
	}
	if client.failure != nil {
		return "", client.failure
	}
	return client.next()
}

func (client *singlePassClient) CallLLMStream(systemPrompt, prompt string, callback LLMStreamCallback) (string, error) {
	raw, err := client.CallLLM(systemPrompt, prompt)
	client.methods[len(client.methods)-1] = "stream"
	if err == nil {
		midpoint := len(raw) / 2
		callback(raw[:midpoint], 1)
		callback(raw[midpoint:], 2)
	}
	return raw, err
}

func (client *singlePassClient) CallLLMText(systemPrompt, prompt string) (string, error) {
	client.test.Fatal("single-pass generation must not invoke a text-only HTML stage")
	return "", nil
}

func (client *singlePassClient) CallLLMTextStream(systemPrompt, prompt string, callback LLMStreamCallback) (string, error) {
	return client.CallLLMText(systemPrompt, prompt)
}

func newSinglePassClient(t *testing.T, outputs ...string) *singlePassClient {
	t.Helper()
	calls := 0
	return &singlePassClient{bespokeMockClient: &bespokeMockClient{test: t, outputs: outputs, calls: &calls}}
}

func TestSinglePassSynthesisCompleteOutput(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "stream"}[streaming], func(t *testing.T) {
			output := synthesisFixtureOutput(t)
			client := newSinglePassClient(t, output)
			var callback LLMStreamCallback
			var chunks []string
			var tokens []int
			if streaming {
				callback = func(delta string, total int) {
					chunks = append(chunks, delta)
					tokens = append(tokens, total)
				}
			}
			syn, err := NewUIDesignCompiler(client).synthesizeStream(context.Background(), "Program studio tari", "mobile", "auto", "light", "#123456", dtos.CompileOptions{PrimaryTask: "Pilih kelas"}, callback)
			if err != nil || syn == nil || syn.RawHTML == "" || len(client.methods) != 1 {
				t.Fatalf("expected one successful call: methods=%v err=%v", client.methods, err)
			}
			if streaming && (client.methods[0] != "stream" || strings.Join(chunks, "") != output || len(tokens) != 2 || tokens[1] != 2) {
				t.Fatal("streaming must forward JSON deltas and token counts unchanged")
			}
			for _, value := range []string{"Program studio tari", "TARGET DEVICE: mobile", "THEME MODE: light", "#123456", "PRIMARY TASK: Pilih kelas", "SINGLE-PASS"} {
				if !strings.Contains(client.prompts[0], value) {
					t.Errorf("request constraint missing: %s", value)
				}
			}
		})
	}
}

func TestSinglePassSynthesisRepair(t *testing.T) {
	var blueprint dtos.UIDesignSynthesisDTO
	if err := json.Unmarshal([]byte(synthesisFixtureOutput(t)), &blueprint); err != nil {
		t.Fatal(err)
	}
	blueprint.RawHTML = ""
	missingHTML, err := json.Marshal(blueprint)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{string(missingHTML), `{"raw_html":"unterminated`, `null`} {
		for _, repaired := range []bool{false, true} {
			second := invalid
			if repaired {
				second = synthesisFixtureOutput(t)
			}
			client := newSinglePassClient(t, invalid, second)
			syn, err := NewUIDesignCompiler(client).synthesizeStream(context.Background(), "Program studio tari", "web", "auto", "light", "", dtos.CompileOptions{}, func(string, int) {})
			if len(client.methods) != 2 || client.methods[0] != "stream" || client.methods[1] != "json" {
				t.Fatalf("expected one streamed attempt and one JSON repair: %v", client.methods)
			}
			if !strings.Contains(client.prompts[1], invalid) || !strings.Contains(client.prompts[1], "previous output was invalid") {
				t.Fatal("repair must include the failed output and validation error")
			}
			if repaired && (err != nil || syn == nil) {
				t.Fatalf("expected repaired implementation: %v", err)
			}
			if !repaired && (err == nil || syn != nil || !strings.Contains(err.Error(), "after one repair")) {
				t.Fatal("persistent invalid output must fail without a template fallback")
			}
		}
	}
}

func TestSinglePassSynthesisCancellationAndProviderError(t *testing.T) {
	for _, phase := range []string{"before", "during"} {
		ctx, cancel := context.WithCancel(context.Background())
		client := newSinglePassClient(t, synthesisFixtureOutput(t))
		if phase == "before" {
			cancel()
		} else {
			client.afterCall = cancel
		}
		syn, err := NewUIDesignCompiler(client).synthesize(ctx, "Program studio tari", "web", "auto", "light", "")
		cancel()
		if !errors.Is(err, context.Canceled) || syn != nil {
			t.Fatalf("canceled generation must not return a result: %v", err)
		}
		if phase == "before" && len(client.methods) != 0 || phase == "during" && len(client.methods) != 1 {
			t.Fatalf("unexpected calls after cancellation: %v", client.methods)
		}
	}
	client := newSinglePassClient(t)
	client.failure = errors.New("provider unavailable")
	_, err := NewUIDesignCompiler(client).synthesize(context.Background(), "Program studio tari", "web", "auto", "light", "")
	if !errors.Is(err, client.failure) || len(client.methods) != 1 {
		t.Fatal("provider failures must propagate without synthesis repair")
	}
}

func TestSinglePassCompileFrontendContract(t *testing.T) {
	for _, device := range []string{"web", "mobile", "desktop"} {
		client := newSinglePassClient(t, synthesisFixtureOutput(t))
		dsl, err := NewUIDesignCompiler(client).Compile(context.Background(), "Program studio tari", device, "auto", "light", "#123456")
		if err != nil || len(dsl.Frames) != 1 || len(client.methods) != 1 {
			t.Fatalf("expected one compiled frame from one call: %v", err)
		}
		frame := dsl.Frames[0]
		if frame.Implementation.Source.HTML != frame.CodeExport["html"] || frame.CodeExport["html"] == "" || !strings.Contains(frame.CodeExport["vue"], "<template>") {
			t.Fatal("canonical source and preview export must match")
		}
		if frame.Theme["mode"] != "light" || frame.Theme["accent"] != "#123456" || frame.Canvas.Device != device {
			t.Fatal("requested theme and device must survive compilation")
		}
		wire, err := json.Marshal(frame)
		if err != nil {
			t.Fatal(err)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(wire, &data); err != nil {
			t.Fatal(err)
		}
		if data["code_export"].(map[string]interface{})["html"] != frame.Implementation.Source.HTML || data["theme"] == nil || data["sections"] == nil {
			t.Fatal("serialized frame must retain the Vue Flow consumer contract")
		}
	}
}
