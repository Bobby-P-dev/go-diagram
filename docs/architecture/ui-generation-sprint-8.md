# Sprint 8: Non-Destructive HTML Sanitizer

## Step 1: Semantic HTML Preservation & Context-Safe Replacement

- **Preserved Semantic `<footer>`**: Removed the destructive regex rewrite of `<footer>` tags into `<div data-rl-kind="footer">`.
- **Fixed Root-Cause CFO Hallucination False Positive**:
  - The hallucination check in `ui_validator.go` was previously matching `"cfo"` inside JSON-escaped HTML strings (where `\u003cfooter` has the substring `cfo`).
  - Updated `ui_validator.go` to match domain keywords using word boundaries (`\bkw\b`) on structured fields (`sec.Type`, `sec.Purpose`, `sec.Data`), completely avoiding false positives on HTML tags.
- **Safe Text-Node Replacement (`replaceTextContentOnly`)**:
  - Replaced crude `patt.ReplaceAllString` across raw HTML with a tokenizer-based scanner.
  - Neutralizes prohibited terms ONLY within visible text nodes.
  - HTML tag names, attribute keys/values (`class`, `id`, `data-rl-id`, `href`), and `<style>` blocks remain 100% untouched.

## Step 2: HTML Tokenizer Sanitizer & Structural Targeting

- **Replaced Brittle Regex Sanitization with `golang.org/x/net/html` Tokenizer**:
  - Strips malicious elements (`<script>`, `<iframe>`, `<object>`, `<embed>`, inline event handlers, `javascript:` URIs) cleanly without regex edge cases.
  - Body wrapper tags with classes (e.g., `<body class="min-h-screen bg-slate-950 font-sans">`) are converted into container `<div>` tags, preserving root typography and background classes instead of dropping them.
  - Eliminated arbitrary width overwrites and hardcoded border injections on tables.
- **Balanced Structural Element Matching (`findHTMLTarget` & `findBalancedTagFromIndex`)**:
  - Locates `data-rl-id` elements using token stream tracking.
  - Safely deletes or extracts elements with exact matching closing tags without cutting off child elements or parent layout containers.

## Step 3: Offline Verification & Test Suite

- Added comprehensive test suite in `internal/modules/uidesign/service/ui_nondestructive_sanitizer_test.go`:
  - `TestNonDestructiveSanitizer_PreservesSemanticFooter`: Verifies `<footer>` preservation and absence of false-positive CFO stripping.
  - `TestNonDestructiveSanitizer_PreservesBodyAttributes`: Verifies retention of body styling attributes.
  - `TestNonDestructiveSanitizer_TextContentOnlyReplacement`: Verifies attribute, class, and URL safety during text cleansing.
  - `TestNonDestructiveSanitizer_PreservesStyleBlockVariables`: Verifies `<style>` CSS rules are preserved.
  - `TestNonDestructiveSanitizer_TableWrappingSafe`: Verifies responsive tables without clashing border overrides.
  - `TestNonDestructiveSanitizer_SafeSectionStripping`: Verifies deep nested tag structure integrity during section removal.
- All verification commands (`make lint-spec`, `make verify`, `make test-race`, `git diff --check`) pass 100% offline.
