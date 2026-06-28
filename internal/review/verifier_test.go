package review

import (
	"reflect"
	"strings"
	"testing"
)

type verifierEvalFixture struct {
	name             string
	context          RepoContext
	raw              ReviewResult
	wantFindings     []wantVerifiedFinding
	wantMissingTests []string
	wantLimitations  []string
	wantStats        VerificationStats
}

type wantVerifiedFinding struct {
	Severity string
	File     string
	Evidence string
}

func TestVerifierEvalFixtures(t *testing.T) {
	line2 := 2
	line99 := 99
	fixtures := []verifierEvalFixture{
		{
			name: "true positive patch evidence",
			context: RepoContext{
				Patches: []PatchContext{{Path: "internal/app.go", Patch: "@@ -1,2 +1,3 @@\n func Name(user *User) string {\n+\treturn user.Name\n }\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", &line2, "return user.Name"),
			}},
			wantFindings: []wantVerifiedFinding{{Severity: "warning", File: "internal/app.go", Evidence: "return user.Name"}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Kept:          1,
				KeptRate:      1,
				Reasons:       map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "unsupported evidence dropped",
			context: RepoContext{
				Patches:   []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return nil\n"}},
				FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nfunc Run() error { return nil }\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", nil, "database password is logged"),
			}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Dropped:       1,
				DroppedRate:   1,
				Reasons:       map[VerificationReason]int{VerificationReasonUnsupportedEvidence: 1},
			},
		},
		{
			name: "unavailable file dropped",
			context: RepoContext{
				Patches: []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return user.Name\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/missing.go", nil, "return user.Name"),
			}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Dropped:       1,
				DroppedRate:   1,
				Reasons:       map[VerificationReason]int{VerificationReasonUnavailableFile: 1},
			},
		},
		{
			name: "line mismatch downgraded when file evidence exists",
			context: RepoContext{
				FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nfunc Name(user *User) string {\n\treturn user.Name\n}\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", &line99, "return user.Name"),
			}},
			wantFindings: []wantVerifiedFinding{{Severity: "question", File: "internal/app.go", Evidence: "return user.Name Verification limitation: line_mismatch."}},
			wantStats: VerificationStats{
				TotalFindings:  1,
				Downgraded:     1,
				DowngradedRate: 1,
				Reasons:        map[VerificationReason]int{VerificationReasonLineMismatch: 1},
			},
		},
		{
			name: "omitted context limitation downgraded",
			context: RepoContext{
				Patches: []PatchContext{{Path: "internal/app.go", Patch: "@@ -1 +1 @@\n+return nil\n"}},
				Omitted: []OmittedContext{{Path: "main_test.go", Section: SectionRelatedTest, Reason: OmitMissing}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", nil, "main_test.go was omitted, so test coverage could not be verified"),
			}},
			wantFindings: []wantVerifiedFinding{{Severity: "question", File: "internal/app.go", Evidence: "main_test.go was omitted, so test coverage could not be verified Verification limitation: omitted_context_dependency."}},
			wantStats: VerificationStats{
				TotalFindings:  1,
				Downgraded:     1,
				DowngradedRate: 1,
				Reasons:        map[VerificationReason]int{VerificationReasonOmittedContextDependency: 1},
			},
		},
		{
			name: "no finding baseline",
			raw:  ReviewResult{Summary: "No actionable findings.", Limitations: []string{"Patch context only."}},
			wantLimitations: []string{
				"Patch context only.",
			},
			wantStats: VerificationStats{
				NoFindingCount: 1,
				Reasons:        map[VerificationReason]int{VerificationReasonNoFindings: 1},
			},
		},
		{
			name: "paraphrased evidence supported by identifiers and literals",
			context: RepoContext{
				FullFiles: []FileContext{{Path: "internal/http.go", Content: "package app\nfunc Do(req *http.Request) error {\n\tclient.Timeout = 0\n\treturn client.Do(req)\n}\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{{
				Severity:        "warning",
				Category:        "bug",
				File:            "internal/http.go",
				Title:           "HTTP client can hang forever",
				Evidence:        "Do leaves client timeout set to zero before client.Do(req)",
				FailureScenario: "A stalled upstream can block forever.",
				Suggestion:      "Configure a positive timeout.",
			}}},
			wantFindings: []wantVerifiedFinding{{Severity: "warning", File: "internal/http.go", Evidence: "Do leaves client timeout set to zero before client.Do(req)"}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Kept:          1,
				KeptRate:      1,
				Reasons:       map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "short code snippet with operator expression kept",
			context: RepoContext{
				Patches: []PatchContext{{Path: "internal/auth.go", Patch: "@@ -8,3 +8,4 @@\n func Login(err error) bool {\n+\tif err == nil { return false }\n }\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/auth.go", nil, "if err == nil"),
			}},
			wantFindings: []wantVerifiedFinding{{Severity: "warning", File: "internal/auth.go", Evidence: "if err == nil"}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Kept:          1,
				KeptRate:      1,
				Reasons:       map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "generic short evidence dropped",
			context: RepoContext{
				Patches: []PatchContext{{Path: "internal/auth.go", Patch: "@@ -1 +1 @@\n+if err == nil { return false }\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/auth.go", nil, "nil"),
			}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Dropped:       1,
				DroppedRate:   1,
				Reasons:       map[VerificationReason]int{VerificationReasonUnsupportedEvidence: 1},
			},
		},
		{
			name: "full-file-only evidence kept",
			context: RepoContext{
				Patches:   []PatchContext{{Path: "internal/cache.go", Patch: "@@ -1 +1 @@\n+func Get() string { return cached }\n"}},
				FullFiles: []FileContext{{Path: "internal/cache.go", Content: "package cache\nvar cached string\nfunc Get() string { return cached }\nfunc Reset() { cached = \"\" }\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/cache.go", nil, "func Reset() { cached = \"\" }"),
			}},
			wantFindings: []wantVerifiedFinding{{Severity: "warning", File: "internal/cache.go", Evidence: "func Reset() { cached = \"\" }"}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Kept:          1,
				KeptRate:      1,
				Reasons:       map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "related test evidence supports missing test finding",
			context: RepoContext{
				Patches:      []PatchContext{{Path: "internal/http.go", Patch: "@@ -1 +1 @@\n+return client.Do(req)\n"}},
				FullFiles:    []FileContext{{Path: "internal/http.go", Content: "package httpx\nfunc Do() error { return client.Do(req) }\n"}},
				RelatedTests: []FileContext{{Path: "internal/http_test.go", Content: "package httpx\nfunc TestDoSuccess(t *testing.T) {}\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{{
				Severity:        "suggestion",
				Category:        "test",
				File:            "internal/http_test.go",
				Title:           "Transport errors need coverage",
				Evidence:        "TestDoSuccess is the only related test",
				FailureScenario: "Transport errors can regress without a test.",
				Suggestion:      "Add an error-path test.",
			}}, MissingTests: []string{"transport error path"}},
			wantFindings:     []wantVerifiedFinding{{Severity: "suggestion", File: "internal/http_test.go", Evidence: "TestDoSuccess is the only related test"}},
			wantMissingTests: []string{"transport error path"},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Kept:          1,
				KeptRate:      1,
				Reasons:       map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "docs config evidence supports config finding",
			context: RepoContext{
				RepoDocs: []FileContext{{Path: ".github/ai-review.yml", Content: "review:\n  max_files: 20\n  mode: advisory\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{{
				Severity:        "suggestion",
				Category:        "config",
				File:            ".github/ai-review.yml",
				Title:           "Review mode remains advisory",
				Evidence:        "mode: advisory",
				FailureScenario: "Users may expect blocking checks.",
				Suggestion:      "Document the advisory setting.",
			}}},
			wantFindings: []wantVerifiedFinding{{Severity: "suggestion", File: ".github/ai-review.yml", Evidence: "mode: advisory"}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Kept:          1,
				KeptRate:      1,
				Reasons:       map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "docs config text cannot support unrelated code defect",
			context: RepoContext{
				Patches:  []PatchContext{{Path: "internal/server.go", Patch: "@@ -1 +1 @@\n+func handler() {}\n"}},
				RepoDocs: []FileContext{{Path: ".github/ai-review.yml", Content: "review:\n  timeout: 0\n  handler: default\n"}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{{
				Severity:        "warning",
				Category:        "bug",
				File:            "internal/server.go",
				Title:           "Handler timeout is disabled",
				Evidence:        "timeout handler",
				FailureScenario: "Requests can hang.",
				Suggestion:      "Set a timeout in code.",
			}}},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Dropped:       1,
				DroppedRate:   1,
				Reasons:       map[VerificationReason]int{VerificationReasonUnsupportedEvidence: 1},
			},
		},
		{
			name: "missing tests preserved but unsupported concrete defect dropped",
			context: RepoContext{
				Patches:      []PatchContext{{Path: "internal/store.go", Patch: "@@ -1 +1 @@\n+func Save() error { return nil }\n"}},
				RelatedTests: []FileContext{{Path: "internal/store_test.go", Content: "package store\nfunc TestSaveSuccess(t *testing.T) {}\n"}},
			},
			raw: ReviewResult{
				Summary: "One issue.",
				Findings: []Finding{
					findingFixture("warning", "internal/store.go", nil, "password is logged"),
				},
				MissingTests: []string{"database failure path"},
				Limitations:  []string{"Only bounded context was available."},
			},
			wantMissingTests: []string{"database failure path"},
			wantLimitations:  []string{"Only bounded context was available."},
			wantStats: VerificationStats{
				TotalFindings: 1,
				Dropped:       1,
				DroppedRate:   1,
				Reasons:       map[VerificationReason]int{VerificationReasonUnsupportedEvidence: 1},
			},
		},
		{
			name: "mixed kept downgraded dropped distribution",
			context: RepoContext{
				FullFiles: []FileContext{{Path: "internal/app.go", Content: "package app\nfunc Name(user *User) string {\n\treturn user.Name\n}\n"}},
			},
			raw: ReviewResult{Summary: "Mixed issues.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", nil, "return user.Name"),
				findingFixture("warning", "internal/app.go", &line99, "return user.Name"),
				findingFixture("warning", "internal/app.go", nil, "database password is logged"),
			}},
			wantFindings: []wantVerifiedFinding{
				{Severity: "warning", File: "internal/app.go", Evidence: "return user.Name"},
				{Severity: "question", File: "internal/app.go", Evidence: "return user.Name Verification limitation: line_mismatch."},
			},
			wantStats: VerificationStats{
				TotalFindings:  3,
				Kept:           1,
				Downgraded:     1,
				Dropped:        1,
				KeptRate:       1.0 / 3.0,
				DowngradedRate: 1.0 / 3.0,
				DroppedRate:    1.0 / 3.0,
				Reasons: map[VerificationReason]int{
					VerificationReasonSupported:           1,
					VerificationReasonLineMismatch:        1,
					VerificationReasonUnsupportedEvidence: 1,
				},
			},
		},
		{
			name: "static check evidence supports matching finding",
			context: RepoContext{
				StaticChecks: []StaticCheckEvidence{{
					SourceType:   EvidenceSourceStaticCheck,
					Tool:         "go vet",
					ExitCategory: GoAnalyzerExitFailure,
					Path:         "internal/app.go",
					Line:         &line2,
					Message:      "fmt.Println call has possible formatting directive %s",
				}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{{
				Severity:        "warning",
				Category:        "bug",
				File:            "internal/app.go",
				Line:            &line2,
				Title:           "Formatting directive is ignored",
				Evidence:        "fmt.Println call has possible formatting directive %s",
				FailureScenario: "The intended value is not formatted into the output.",
				Suggestion:      "Use fmt.Printf or remove the directive.",
			}}},
			wantFindings: []wantVerifiedFinding{{Severity: "warning", File: "internal/app.go", Evidence: "fmt.Println call has possible formatting directive %s"}},
			wantStats: VerificationStats{
				TotalFindings:            1,
				Kept:                     1,
				KeptRate:                 1,
				StaticCheckEvidenceCount: 1,
				StaticCheckSupported:     1,
				StaticCheckSkipped:       map[GoAnalyzerExitCategory]int{GoAnalyzerExitFailure: 1},
				Reasons:                  map[VerificationReason]int{VerificationReasonSupported: 1},
			},
		},
		{
			name: "unrelated static check evidence cannot support finding",
			context: RepoContext{
				StaticChecks: []StaticCheckEvidence{{
					SourceType:   EvidenceSourceStaticCheck,
					Tool:         "go test",
					ExitCategory: GoAnalyzerExitFailure,
					Path:         "internal/other.go",
					Message:      "undefined: otherSymbol",
				}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", nil, "undefined: otherSymbol"),
			}},
			wantStats: VerificationStats{
				TotalFindings:            1,
				Dropped:                  1,
				DroppedRate:              1,
				StaticCheckEvidenceCount: 1,
				StaticCheckSkipped:       map[GoAnalyzerExitCategory]int{GoAnalyzerExitFailure: 1},
				Reasons:                  map[VerificationReason]int{VerificationReasonUnavailableFile: 1},
			},
		},
		{
			name: "generic static check overlap is insufficient",
			context: RepoContext{
				StaticChecks: []StaticCheckEvidence{{
					SourceType:   EvidenceSourceStaticCheck,
					Tool:         "go test",
					ExitCategory: GoAnalyzerExitFailure,
					Path:         "internal/app.go",
					Message:      "test failed with error",
				}},
			},
			raw: ReviewResult{Summary: "One issue.", Findings: []Finding{
				findingFixture("warning", "internal/app.go", nil, "test error"),
			}},
			wantStats: VerificationStats{
				TotalFindings:            1,
				Dropped:                  1,
				DroppedRate:              1,
				StaticCheckEvidenceCount: 1,
				StaticCheckSkipped:       map[GoAnalyzerExitCategory]int{GoAnalyzerExitFailure: 1},
				Reasons:                  map[VerificationReason]int{VerificationReasonUnsupportedEvidence: 1},
			},
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			verified, stats := VerifyReviewResult(fixture.raw, fixture.context)
			assertVerifiedFindings(t, verified.Findings, fixture.wantFindings)
			if !reflect.DeepEqual(verified.MissingTests, fixture.wantMissingTests) {
				t.Fatalf("missing tests = %#v, want %#v", verified.MissingTests, fixture.wantMissingTests)
			}
			if !reflect.DeepEqual(verified.Limitations, fixture.wantLimitations) {
				t.Fatalf("limitations = %#v, want %#v", verified.Limitations, fixture.wantLimitations)
			}
			assertVerificationStats(t, stats, fixture.wantStats)
			assertStatsContainOnlyAggregateValues(t, fixture.name, stats)
		})
	}
}

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

func assertVerifiedFindings(t *testing.T, got []Finding, want []wantVerifiedFinding) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i].Severity != want[i].Severity || got[i].File != want[i].File || got[i].Evidence != want[i].Evidence {
			t.Fatalf("finding[%d] = severity=%q file=%q evidence=%q, want %#v", i, got[i].Severity, got[i].File, got[i].Evidence, want[i])
		}
	}
}

func assertVerificationStats(t *testing.T, got VerificationStats, want VerificationStats) {
	t.Helper()
	if got.TotalFindings != want.TotalFindings ||
		got.Kept != want.Kept ||
		got.Downgraded != want.Downgraded ||
		got.Dropped != want.Dropped ||
		got.NoFindingCount != want.NoFindingCount ||
		got.KeptRate != want.KeptRate ||
		got.DowngradedRate != want.DowngradedRate ||
		got.DroppedRate != want.DroppedRate ||
		got.StaticCheckEvidenceCount != want.StaticCheckEvidenceCount ||
		got.StaticCheckSupported != want.StaticCheckSupported ||
		!reflect.DeepEqual(got.StaticCheckSkipped, want.StaticCheckSkipped) ||
		!reflect.DeepEqual(got.Reasons, want.Reasons) {
		t.Fatalf("stats = %+v, want %+v", got, want)
	}
}

func assertStatsContainOnlyAggregateValues(t *testing.T, fixtureName string, stats VerificationStats) {
	t.Helper()
	text := fixtureName + " " + stats.String()
	for _, disallowed := range []string{
		"package ", "func ", "return ", "password", "secret", "token", "private key", "installation",
		"raw prompt", "raw model", "webhook payload",
	} {
		if containsFold(text, disallowed) {
			t.Fatalf("stats summary leaked %q: %q", disallowed, text)
		}
	}
}

func containsFold(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
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
