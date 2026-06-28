package review

import "testing"

func TestVerifyReviewResultKeepsSupportedFinding(t *testing.T) {
	line := 2
	confidence := 0.8
	raw := ReviewResult{
		Summary: "Found one issue.",
		Findings: []Finding{{
			Severity:        "warning",
			Category:        "bug",
			File:            "internal/app.go",
			Line:            &line,
			Title:           "Nil value can panic",
			Evidence:        "return user.Name",
			FailureScenario: "A nil user panics when Name is read.",
			Suggestion:      "Check user before reading Name.",
			Confidence:      &confidence,
		}},
	}
	ctx := RepoContext{
		Patches: []PatchContext{{
			Path:  "internal/app.go",
			Patch: "@@ -1,2 +1,3 @@\n func Name(user *User) string {\n+\treturn user.Name\n }\n",
		}},
		FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nfunc Name(user *User) string {\n\treturn user.Name\n}\n"}},
	}

	verified, stats := VerifyReviewResult(raw, ctx)

	if len(verified.Findings) != 1 {
		t.Fatalf("findings = %d, want 1; stats=%+v", len(verified.Findings), stats)
	}
	got := verified.Findings[0]
	if got.Severity != raw.Findings[0].Severity || got.Title != raw.Findings[0].Title || got.Evidence != raw.Findings[0].Evidence || got.Confidence != raw.Findings[0].Confidence {
		t.Fatalf("finding was not preserved: got %+v want %+v", got, raw.Findings[0])
	}
	if stats.TotalFindings != 1 || stats.Kept != 1 || stats.Reasons[VerificationReasonSupported] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestVerifyReviewResultDropsUnsupportedFinding(t *testing.T) {
	raw := ReviewResult{
		Summary:  "Found one issue.",
		Findings: []Finding{findingFixture("warning", "internal/app.go", nil, "database password is logged")},
	}
	ctx := RepoContext{
		Patches:   []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return nil\n"}},
		FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nfunc Run() error { return nil }\n"}},
	}

	verified, stats := VerifyReviewResult(raw, ctx)

	if len(verified.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", verified.Findings)
	}
	if stats.TotalFindings != 1 || stats.Dropped != 1 || stats.Reasons[VerificationReasonUnsupportedEvidence] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestVerifyReviewResultDropsUnavailableFile(t *testing.T) {
	raw := ReviewResult{
		Summary:  "Found one issue.",
		Findings: []Finding{findingFixture("warning", "internal/missing.go", nil, "return user.Name")},
	}
	ctx := RepoContext{
		Patches:   []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return user.Name\n"}},
		FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nreturn user.Name\n"}},
	}

	verified, stats := VerifyReviewResult(raw, ctx)

	if len(verified.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", verified.Findings)
	}
	if stats.Dropped != 1 || stats.Reasons[VerificationReasonUnavailableFile] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestVerifyReviewResultDowngradesLineMismatchWithFileEvidence(t *testing.T) {
	line := 99
	raw := ReviewResult{
		Summary:  "Found one issue.",
		Findings: []Finding{findingFixture("warning", "internal/app.go", &line, "return user.Name")},
	}
	ctx := RepoContext{
		FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nfunc Name(user *User) string {\n\treturn user.Name\n}\n"}},
	}

	verified, stats := VerifyReviewResult(raw, ctx)

	if len(verified.Findings) != 1 {
		t.Fatalf("findings = %d, want 1; stats=%+v", len(verified.Findings), stats)
	}
	got := verified.Findings[0]
	if got.Severity != "question" {
		t.Fatalf("severity = %q, want question", got.Severity)
	}
	if got.Line != nil {
		t.Fatalf("line = %v, want nil after downgrade", *got.Line)
	}
	if got.Evidence == raw.Findings[0].Evidence {
		t.Fatalf("downgraded evidence should include limitation text, got %q", got.Evidence)
	}
	if stats.Downgraded != 1 || stats.Reasons[VerificationReasonLineMismatch] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestVerifyReviewResultDowngradesOmittedContextLimitation(t *testing.T) {
	raw := ReviewResult{
		Summary:  "Found one issue.",
		Findings: []Finding{findingFixture("warning", "internal/app.go", nil, "main_test.go was omitted, so test coverage could not be verified")},
	}
	ctx := RepoContext{
		Patches: []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return nil\n"}},
		Omitted: []OmittedContext{{Path: "main_test.go", Section: SectionRelatedTest, Reason: OmitMissing}},
	}

	verified, stats := VerifyReviewResult(raw, ctx)

	if len(verified.Findings) != 1 {
		t.Fatalf("findings = %d, want 1; stats=%+v", len(verified.Findings), stats)
	}
	if verified.Findings[0].Severity != "question" {
		t.Fatalf("severity = %q, want question", verified.Findings[0].Severity)
	}
	if stats.Downgraded != 1 || stats.Reasons[VerificationReasonOmittedContextDependency] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestVerifyReviewResultDropsConcreteOmittedContextDefect(t *testing.T) {
	raw := ReviewResult{
		Summary:  "Found one issue.",
		Findings: []Finding{findingFixture("warning", "main_test.go", nil, "assertion is wrong")},
	}
	ctx := RepoContext{
		Patches: []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return nil\n"}},
		Omitted: []OmittedContext{{Path: "main_test.go", Section: SectionRelatedTest, Reason: OmitMissing}},
	}

	verified, stats := VerifyReviewResult(raw, ctx)

	if len(verified.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", verified.Findings)
	}
	if stats.Dropped != 1 || stats.Reasons[VerificationReasonOmittedContextDependency] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestVerifyReviewResultPreservesNoFindingResult(t *testing.T) {
	raw := ReviewResult{Summary: "No actionable findings.", Limitations: []string{"Patch context only."}}

	verified, stats := VerifyReviewResult(raw, RepoContext{})

	if len(verified.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", verified.Findings)
	}
	if verified.Summary != raw.Summary || len(verified.Limitations) != 1 {
		t.Fatalf("verified result = %+v, want %+v", verified, raw)
	}
	if stats.TotalFindings != 0 || stats.Reasons[VerificationReasonNoFindings] != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func findingFixture(severity, file string, line *int, evidence string) Finding {
	return Finding{
		Severity:        severity,
		Category:        "bug",
		File:            file,
		Line:            line,
		Title:           "Potential issue",
		Evidence:        evidence,
		FailureScenario: "The code may fail in production.",
		Suggestion:      "Check the behavior before relying on it.",
	}
}
