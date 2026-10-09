package services

import (
	"strings"
	"testing"
)

func TestSanitizeStaticHTML(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Strips form onsubmit event handler",
			input:    `<form class="space-y-4" onsubmit="return false"><input type="text"/></form>`,
			expected: `<form class="space-y-4"><input type="text"/></form>`,
		},
		{
			name:     "Strips button onclick event handler",
			input:    `<button class="btn btn-primary" onclick="alert('hack')">Submit</button>`,
			expected: `<button class="btn btn-primary">Submit</button>`,
		},
		{
			name:     "Neutralizes javascript: in anchor href",
			input:    `<a href="javascript:void(0)" class="text-sm text-indigo-600">Lupa password?</a>`,
			expected: `<a href="#" class="text-sm text-indigo-600">Lupa password?</a>`,
		},
		{
			name:     "Neutralizes javascript: semicolon in anchor href",
			input:    `<a href="javascript:;" class="text-sm">Bantuan</a>`,
			expected: `<a href="#" class="text-sm">Bantuan</a>`,
		},
		{
			name:     "Strips script tags and inline block",
			input:    `<div><script type="text/javascript">console.log("bad");</script><span>Content</span></div>`,
			expected: `<div><span>Content</span></div>`,
		},
		{
			name:     "Strips iframe blocks",
			input:    `<div><iframe src="https://evil.com"></iframe><span>Safe</span></div>`,
			expected: `<div><span>Safe</span></div>`,
		},
		{
			name:     "Strips doctype, head and body wrapper tags while preserving inner content",
			input:    `<!DOCTYPE html><html><head><title>Title</title></head><body><div data-rl-id="sec-main">Hello World</div></body></html>`,
			expected: `<div data-rl-id="sec-main">Hello World</div>`,
		},
		{
			name:     "Preserves valid Tailwind classes and attributes",
			input:    `<div class="p-4 bg-slate-900 border border-slate-800 text-white font-medium" data-rl-id="sec-hero">Dashboard</div>`,
			expected: `<div class="p-4 bg-slate-900 border border-slate-800 text-white font-medium" data-rl-id="sec-hero">Dashboard</div>`,
		},
		{
			name:     "Strips markdown fences",
			input:    "```html\n<div data-rl-id=\"sec-1\">Clean</div>\n```",
			expected: `<div data-rl-id="sec-1">Clean</div>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeStaticHTML(tc.input)
			if got != tc.expected {
				t.Errorf("\nInput:    %s\nExpected: %s\nGot:      %s", tc.input, tc.expected, got)
			}
			// Verify bespokeDisallowedPattern passes
			if bespokeDisallowedPattern.MatchString(got) {
				t.Errorf("Sanitized HTML still matches bespokeDisallowedPattern: %s", got)
			}
		})
	}
}

func TestSanitizeStaticHTMLProtectsFullLoginForm(t *testing.T) {
	rawInput := `<form onsubmit="return false" class="bg-white p-6 rounded-xl shadow-lg border border-slate-200" data-rl-id="sec-login">
		<h2 class="text-xl font-bold text-slate-900">Masuk ke Portal</h2>
		<div class="mt-4">
			<label class="block text-sm font-medium text-slate-700">Email</label>
			<input type="email" onchange="validate(this)" class="w-full px-3 py-2 border rounded-lg" />
		</div>
		<div class="mt-4 flex justify-between items-center">
			<a href="javascript:void(0)" class="text-sm text-blue-600 hover:underline">Lupa Password?</a>
		</div>
		<button type="submit" onclick="doLogin()" class="mt-6 w-full py-2 bg-indigo-600 text-white font-semibold rounded-lg">Masuk</button>
		<script>document.title = 'Login';</script>
	</form>`

	sanitized := sanitizeStaticHTML(rawInput)

	if strings.Contains(sanitized, "onsubmit") {
		t.Errorf("onsubmit was not stripped")
	}
	if strings.Contains(sanitized, "onchange") {
		t.Errorf("onchange was not stripped")
	}
	if strings.Contains(sanitized, "onclick") {
		t.Errorf("onclick was not stripped")
	}
	if strings.Contains(sanitized, "javascript:") {
		t.Errorf("javascript: was not neutralized")
	}
	if strings.Contains(sanitized, "<script") {
		t.Errorf("script tag was not removed")
	}
	if bespokeDisallowedPattern.MatchString(sanitized) {
		t.Errorf("Disallowed pattern still triggered on sanitized login form: %s", sanitized)
	}
}
