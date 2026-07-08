package comment

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github-ai-reviewer/internal/review"
)

func TestDiffRightLinesMapsAddedAndContextLines(t *testing.T) {
	patch := `@@ -10,4 +20,5 @@ func main() {
 context
-old
+new
 another
+added
\ No newline at end of file`
	lines := diffRightLines(patch)
	for _, want := range []int{20, 21, 22, 23} {
		if _, ok := lines[want]; !ok {
			t.Fatalf("line %d missing from %#v", want, lines)
		}
	}
	if _, ok := lines[24]; ok {
		t.Fatalf("unexpected line 24 in %#v", lines)
	}
}

func TestPublisherCreatesInlineCommentForDiffFinding(t *testing.T) {
	line := 21
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{
			Filename: "main.go",
			Patch:    "@@ -1,2 +20,3 @@\n context\n+badCall()\n more\n",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{{
			Severity:        "blocker",
			File:            "main.go",
			Line:            &line,
			Title:           "Bad call",
			Evidence:        "badCall is introduced",
			FailureScenario: "runtime failure",
			Suggestion:      "guard it",
		}},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 1 {
		t.Fatalf("created review = %+v", fake.createdPullRequestReview)
	}
	created := fake.createdPullRequestReview.Comments[0]
	if created.Path != "main.go" || created.Line != 21 || created.Side != "RIGHT" || fake.createdPullRequestReview.CommitID != "abc123" || fake.createdPullRequestReview.Event != "COMMENT" || fake.createdPullRequestReview.Body == "" {
		t.Fatalf("created review = %+v", fake.createdReview)
	}
	if !strings.Contains(created.Body, InlineMarker) || !strings.Contains(created.Body, "Bad call") {
		t.Fatalf("created body = %q", created.Body)
	}
	if fake.createdBody == "" {
		t.Fatal("summary issue comment was not created")
	}
}

func TestPublisherSkipsLowValueInlineFindings(t *testing.T) {
	line := 20
	lowConfidence := 0.69
	tests := []struct {
		name    string
		finding review.Finding
	}{
		{
			name: "question severity",
			finding: review.Finding{
				Severity:        "question",
				File:            "main.go",
				Line:            &line,
				Title:           "Question",
				Evidence:        "badCall()",
				FailureScenario: "unclear failure",
				Suggestion:      "check intent",
			},
		},
		{
			name: "low confidence",
			finding: review.Finding{
				Severity:        "warning",
				File:            "main.go",
				Line:            &line,
				Title:           "Low confidence",
				Evidence:        "badCall()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &lowConfidence,
			},
		},
		{
			name: "missing failure scenario",
			finding: review.Finding{
				Severity:   "warning",
				File:       "main.go",
				Line:       &line,
				Title:      "Incomplete",
				Evidence:   "badCall()",
				Suggestion: "guard it",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeInlineCommenter{files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}}}
			pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
			err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{tt.finding}})
			if err != nil {
				t.Fatalf("PublishForHead() error = %v", err)
			}
			if fake.createdBody == "" {
				t.Fatal("summary issue comment was not created")
			}
			if len(fake.createdPullRequestReview.Comments) != 0 || fake.updatedReviewBody != "" {
				t.Fatalf("unexpected inline output: created=%+v updated=%q", fake.createdPullRequestReview, fake.updatedReviewBody)
			}
		})
	}
}

func TestPublisherCreatesInlineCommentForHighConfidenceFinding(t *testing.T) {
	line := 20
	confidence := 0.70
	fake := &fakeInlineCommenter{files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}}}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{{
		Severity:        "blocker",
		File:            "main.go",
		Line:            &line,
		Title:           "High confidence",
		Evidence:        "badCall()",
		FailureScenario: "runtime failure",
		Suggestion:      "guard it",
		Confidence:      &confidence,
	}}})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 1 || fake.createdPullRequestReview.Comments[0].Body == "" {
		t.Fatal("expected inline review comment")
	}
}

func TestRenderInlineFindingStartsWithVisibleSeverityLabel(t *testing.T) {
	line := 20
	blocker := eligibleFinding("main.go", line, "Nil response can panic", "resp is dereferenced")
	blocker.Severity = "blocker"
	warning := eligibleFinding("main.go", line, "Missing error check", "err is ignored")
	warning.Severity = "warning"

	tests := []struct {
		name    string
		finding review.Finding
		want    string
	}{
		{name: "blocker", finding: blocker, want: "🚨 **Blocking risk:** Nil response can panic"},
		{name: "warning", finding: warning, want: "⚠️ **Potential issue:** Missing error check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := renderInlineFinding(tt.finding, review.LanguageEnglish)
			if !strings.HasPrefix(body, tt.want) {
				t.Fatalf("body prefix mismatch\nwant prefix: %q\nbody:\n%s", tt.want, body)
			}
			if strings.HasPrefix(strings.TrimSpace(body), InlineMarker) {
				t.Fatalf("body starts with hidden marker:\n%s", body)
			}
		})
	}
}

func TestRenderInlineFindingUsesCollapsedDetails(t *testing.T) {
	line := 20
	confidence := 0.84
	finding := eligibleFinding("main.go", line, "Nil response can panic", "resp is dereferenced before err is checked")
	finding.FailureScenario = "transport error returns nil resp"
	finding.Suggestion = "Check err before using resp."
	finding.Confidence = &confidence

	body := renderInlineFinding(finding, review.LanguageEnglish)
	detailsStart := strings.Index(body, "<details>")
	detailsEnd := strings.Index(body, "</details>")
	if detailsStart < 0 || detailsEnd < detailsStart {
		t.Fatalf("body missing details block:\n%s", body)
	}
	visible := body[:detailsStart]
	details := body[detailsStart:detailsEnd]
	for _, notWant := range []string{"- Evidence:", "- Failure scenario:", "- Confidence:"} {
		if strings.Contains(visible, notWant) {
			t.Fatalf("visible section contains %q:\n%s", notWant, body)
		}
	}
	for _, want := range []string{
		"<summary>Details</summary>",
		"**Evidence:** resp is dereferenced before err is checked",
		"**Failure scenario:** transport error returns nil resp",
		"**Confidence:** 0.84",
	} {
		if !strings.Contains(details, want) {
			t.Fatalf("details missing %q:\n%s", want, body)
		}
	}
	if !strings.Contains(visible, "**Suggestion:** Check err before using resp.") {
		t.Fatalf("visible section missing suggestion:\n%s", body)
	}
}

func TestRenderInlineFindingOmitsMissingConfidence(t *testing.T) {
	line := 20
	finding := eligibleFinding("main.go", line, "Missing error check", "err is ignored")

	body := renderInlineFinding(finding, review.LanguageEnglish)
	if strings.Contains(body, "Confidence") || strings.Contains(body, "0.00") {
		t.Fatalf("body fabricated confidence:\n%s", body)
	}
}

func TestRenderInlineFindingLocalizesChineseLabels(t *testing.T) {
	line := 20
	confidence := 0.91
	finding := eligibleFinding("main.go", line, "空指针风险", "resp 在检查 err 前被使用")
	finding.Severity = "blocker"
	finding.FailureScenario = "请求失败时 resp 可能为空"
	finding.Suggestion = "先检查 err。"
	finding.Confidence = &confidence

	body := renderInlineFinding(finding, review.LanguageSimplifiedChinese)
	for _, want := range []string{
		"🚨 **阻塞风险:** 空指针风险",
		"**建议:** 先检查 err。",
		"<summary>详情</summary>",
		"**证据:** resp 在检查 err 前被使用",
		"**失败场景:** 请求失败时 resp 可能为空",
		"**置信度:** 0.91",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestRenderPullRequestReviewBodyIsHumanFriendly(t *testing.T) {
	english := renderPullRequestReviewBody(2, review.LanguageEnglish)
	for _, want := range []string{"Review Cat left 2 inline comment(s).", "advisory", "non-blocking"} {
		if !strings.Contains(english, want) {
			t.Fatalf("English body missing %q: %s", want, english)
		}
	}
	if strings.Contains(english, "AI Review found") {
		t.Fatalf("English body uses stale report wording: %s", english)
	}

	chinese := renderPullRequestReviewBody(3, review.LanguageSimplifiedChinese)
	for _, want := range []string{"Review Cat 留下了 3 条行级评论", "建议", "不会阻塞"} {
		if !strings.Contains(chinese, want) {
			t.Fatalf("Chinese body missing %q: %s", want, chinese)
		}
	}
}

func TestPublisherAppliesJobInlinePolicy(t *testing.T) {
	line20 := 20
	line21 := 21
	line22 := 22
	confidenceLow := 0.80
	confidenceHigh := 0.95
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{
			Filename: "main.go",
			Patch:    "@@ -1 +20,4 @@\n+first()\n+second()\n+third()\n",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	effective := review.MergeEffectiveReviewConfig(review.DefaultGlobalReviewConfig(review.LanguageEnglish), nil)
	effective.InlineMaxComments = 1
	effective.InlineSeverityThreshold = review.SeverityWarning
	effective.InlineConfidenceThreshold = 0.90
	job := review.Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc123", EffectiveConfig: &effective}

	err := pub.PublishForJob(context.Background(), job, review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{
			{
				Severity:        "suggestion",
				File:            "main.go",
				Line:            &line20,
				Title:           "Suggestion skipped",
				Evidence:        "first()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &confidenceHigh,
			},
			{
				Severity:        "warning",
				File:            "main.go",
				Line:            &line21,
				Title:           "Low confidence skipped",
				Evidence:        "second()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &confidenceLow,
			},
			{
				Severity:        "warning",
				File:            "main.go",
				Line:            &line22,
				Title:           "High confidence kept",
				Evidence:        "third()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &confidenceHigh,
			},
			{
				Severity:        "blocker",
				File:            "main.go",
				Line:            &line20,
				Title:           "Limited out",
				Evidence:        "first()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &confidenceHigh,
			},
		},
	})
	if err != nil {
		t.Fatalf("PublishForJob() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 1 {
		t.Fatalf("comments = %+v", fake.createdPullRequestReview.Comments)
	}
	if !strings.Contains(fake.createdPullRequestReview.Comments[0].Body, "High confidence kept") {
		t.Fatalf("created body = %q", fake.createdPullRequestReview.Comments[0].Body)
	}
}

func TestPublisherAppliesJobPathIgnoreToInlineEligibility(t *testing.T) {
	line := 20
	fake := &fakeInlineCommenter{
		files: []review.FileChange{
			{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+main()\n"},
			{Filename: "docs/secret.md", Patch: "@@ -1 +20,2 @@\n+secret\n"},
		},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	effective := review.MergeEffectiveReviewConfig(review.DefaultGlobalReviewConfig(review.LanguageEnglish), nil)
	cfg, err := review.ParseRepositoryConfig([]byte("path_ignore:\n  - docs/**\n"))
	if err != nil {
		t.Fatalf("ParseRepositoryConfig() error = %v", err)
	}
	effective.PathIgnore = cfg.PathIgnore
	job := review.Job{InstallationID: 42, Owner: "octo", Repo: "repo", PullNumber: 7, HeadSHA: "abc123", EffectiveConfig: &effective}

	err = pub.PublishForJob(context.Background(), job, review.ReviewResult{
		Summary:  "summary",
		Findings: []review.Finding{eligibleFinding("docs/secret.md", line, "Ignored", "secret")},
	})
	if err != nil {
		t.Fatalf("PublishForJob() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 0 {
		t.Fatalf("ignored path produced inline comments: %+v", fake.createdPullRequestReview.Comments)
	}
}

func TestPublisherSkipsInlineCommentOutsideDiff(t *testing.T) {
	line := 99
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{{
			Severity:        "warning",
			File:            "main.go",
			Line:            &line,
			Title:           "Bad call",
			Evidence:        "evidence",
			FailureScenario: "failure",
			Suggestion:      "fix",
		}},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 0 || fake.updatedReviewBody != "" {
		t.Fatalf("unexpected inline output: created=%+v updated=%q", fake.createdPullRequestReview, fake.updatedReviewBody)
	}
}

func TestPublisherBatchesMultipleNewInlineCommentsInOneReview(t *testing.T) {
	line20 := 20
	line21 := 21
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,3 @@\n+first()\n+second()\n"}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{
			eligibleFinding("main.go", line20, "First", "first()"),
			eligibleFinding("main.go", line21, "Second", "second()"),
		},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if fake.createdReview.Body != "" {
		t.Fatalf("legacy individual create used: %+v", fake.createdReview)
	}
	if fake.createdPullRequestReview.CommitID != "abc123" || fake.createdPullRequestReview.Event != "COMMENT" || strings.TrimSpace(fake.createdPullRequestReview.Body) == "" {
		t.Fatalf("created pull request review = %+v", fake.createdPullRequestReview)
	}
	if len(fake.createdPullRequestReview.Comments) != 2 {
		t.Fatalf("comments = %+v", fake.createdPullRequestReview.Comments)
	}
}

func TestPublisherSplitsExistingNewAndObsoleteInlineComments(t *testing.T) {
	line20 := 20
	line21 := 21
	current := eligibleFinding("main.go", line20, "Current", "current()")
	obsolete := eligibleFinding("main.go", line21, "Obsolete", "obsolete()")
	existing := eligibleFinding("main.go", line21, "Existing", "existing()")
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,3 @@\n+current()\n+existing()\n"}},
		reviewComments: []ReviewComment{
			{ID: 55, AuthorType: "Bot", Body: InlineMarker + " fingerprint=" + inlineFingerprint(existing) + " -->\nold existing"},
			{ID: 56, AuthorType: "Bot", Body: InlineMarker + " fingerprint=" + inlineFingerprint(obsolete) + " -->\nold obsolete"},
			{ID: 57, AuthorType: "Bot", Body: "other bot comment"},
			{ID: 58, AuthorType: "User", Body: InlineMarker + " fingerprint=" + inlineFingerprint(current) + " -->\nhuman"},
		},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary:  "summary",
		Findings: []review.Finding{current, existing},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 1 || !strings.Contains(fake.createdPullRequestReview.Comments[0].Body, "Current") {
		t.Fatalf("created pull request review = %+v", fake.createdPullRequestReview)
	}
	if got := fake.updatedReviewBodies[55]; !strings.Contains(got, "Existing") {
		t.Fatalf("existing update body = %q", got)
	}
	if got := fake.updatedReviewBodies[56]; !strings.Contains(got, InlineStaleMarker) || !strings.Contains(got, inlineFingerprint(obsolete)) {
		t.Fatalf("stale body = %q", got)
	}
	if _, ok := fake.updatedReviewBodies[57]; ok {
		t.Fatal("updated unrelated bot comment")
	}
	if _, ok := fake.updatedReviewBodies[58]; ok {
		t.Fatal("updated human comment")
	}
}

func TestExistingInlineCommentsRecognizesTrailingAndLeadingMarkers(t *testing.T) {
	fingerprint := "aaaaaaaaaaaaaaaa"
	comments := []ReviewComment{
		{ID: 55, AuthorType: "Bot", Body: "⚠️ **Potential issue:** Bad call\n\n" + InlineMarker + " fingerprint=" + fingerprint + " -->"},
		{ID: 56, AuthorType: "Bot", Body: InlineMarker + " fingerprint=bbbbbbbbbbbbbbbb -->\nold format"},
	}

	got := existingInlineComments(comments)
	if got[fingerprint].ID != 55 {
		t.Fatalf("trailing-marker comment not discovered: %+v", got)
	}
	if got["bbbbbbbbbbbbbbbb"].ID != 56 {
		t.Fatalf("leading-marker comment not discovered: %+v", got)
	}
}

func TestExistingInlineCommentsIgnoresUnmarkedOrInvalidFingerprint(t *testing.T) {
	comments := []ReviewComment{
		{ID: 55, AuthorType: "Bot", Body: "⚠️ **Potential issue:** Bad call\n\nfingerprint=aaaaaaaaaaaaaaaa"},
		{ID: 56, AuthorType: "Bot", Body: InlineMarker + " fingerprint=not-a-fingerprint -->\ninvalid"},
		{ID: 57, AuthorType: "User", Body: InlineMarker + " fingerprint=bbbbbbbbbbbbbbbb -->\nhuman"},
	}

	if got := existingInlineComments(comments); len(got) != 0 {
		t.Fatalf("unexpected existing comments: %+v", got)
	}
}

func TestStaleAndInactiveInlineCommentsPreserveMetadata(t *testing.T) {
	body := "⚠️ **Potential issue:** Bad call\n\n" + InlineMarker + " fingerprint=aaaaaaaaaaaaaaaa -->"
	stale := renderStaleInlineComment(body, "aaaaaaaaaaaaaaaa", "abc123")
	if !strings.Contains(stale, InlineMarker) || !strings.Contains(stale, "fingerprint=aaaaaaaaaaaaaaaa") || !strings.Contains(stale, InlineStaleMarker) {
		t.Fatalf("stale body lost metadata:\n%s", stale)
	}
	inactive := renderInactiveInlineComment(body, "aaaaaaaaaaaaaaaa", review.CleanupStateClosed)
	if !strings.Contains(inactive, InlineMarker) || !strings.Contains(inactive, "fingerprint=aaaaaaaaaaaaaaaa") || !strings.Contains(inactive, InlineStaleMarker) {
		t.Fatalf("inactive body lost metadata:\n%s", inactive)
	}
}

func TestPublisherSkipsEmptyPullRequestReviewWhenOnlyExistingCommentsUpdate(t *testing.T) {
	line := 20
	finding := eligibleFinding("main.go", line, "Existing only", "existing()")
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+existing()\n"}},
		reviewComments: []ReviewComment{{
			ID:         55,
			AuthorType: "Bot",
			Body:       InlineMarker + " fingerprint=" + inlineFingerprint(finding) + " -->\nold",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{finding}})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 0 {
		t.Fatalf("created empty review: %+v", fake.createdPullRequestReview)
	}
	if fake.updatedReviewID != 55 {
		t.Fatalf("updatedReviewID = %d", fake.updatedReviewID)
	}
}

func TestPublisherFallsBackToIndividualCreatesAfterBatchFailureWithoutDuplicatingExisting(t *testing.T) {
	line20 := 20
	line21 := 21
	newFinding := eligibleFinding("main.go", line20, "New", "new()")
	existingFinding := eligibleFinding("main.go", line21, "Existing", "existing()")
	fake := &fakeInlineCommenter{
		files:           []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,3 @@\n+new()\n+existing()\n"}},
		createReviewErr: fmt.Errorf("batch failed"),
		reviewComments: []ReviewComment{{
			ID:         55,
			AuthorType: "Bot",
			Body:       InlineMarker + " fingerprint=" + inlineFingerprint(existingFinding) + " -->\nold",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary:  "summary",
		Findings: []review.Finding{newFinding, existingFinding},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.fallbackCreatedReviews) != 1 || !strings.Contains(fake.fallbackCreatedReviews[0].Body, "New") {
		t.Fatalf("fallbackCreatedReviews = %+v", fake.fallbackCreatedReviews)
	}
	if strings.Contains(fake.fallbackCreatedReviews[0].Body, "Existing") {
		t.Fatalf("fallback duplicated existing body = %q", fake.fallbackCreatedReviews[0].Body)
	}
	if fake.updatedReviewID != 55 {
		t.Fatalf("existing comment not updated: %d", fake.updatedReviewID)
	}
}

func TestPublisherLimitsInlineBatchToTenMappedFindings(t *testing.T) {
	var patch strings.Builder
	patch.WriteString("@@ -1 +20,12 @@\n")
	findings := make([]review.Finding, 0, 12)
	for i := 0; i < 12; i++ {
		line := 20 + i
		fmt.Fprintf(&patch, "+call%d()\n", i)
		findings = append(findings, eligibleFinding("main.go", line, fmt.Sprintf("Finding %d", i), fmt.Sprintf("call%d()", i)))
	}
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: patch.String()}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: findings})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 10 {
		t.Fatalf("created %d comments, want 10", len(fake.createdPullRequestReview.Comments))
	}
}

func TestPublisherIsolatesStaleMarkFailures(t *testing.T) {
	line20 := 20
	line21 := 21
	current := eligibleFinding("main.go", line20, "Current", "current()")
	obsolete := eligibleFinding("main.go", line21, "Obsolete", "obsolete()")
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+current()\n"}},
		reviewComments: []ReviewComment{{
			ID:         56,
			AuthorType: "Bot",
			Body:       InlineMarker + " fingerprint=" + inlineFingerprint(obsolete) + " -->\nold obsolete",
		}},
		updateErrIDs: map[int64]error{56: fmt.Errorf("stale update failed")},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{current}})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 1 || !strings.Contains(fake.createdPullRequestReview.Comments[0].Body, "Current") {
		t.Fatalf("new comments were not batched after stale failure: %+v", fake.createdPullRequestReview)
	}
}

func TestPublisherLogsInlineStats(t *testing.T) {
	line20 := 20
	line21 := 21
	line99 := 99
	lowConfidence := 0.69
	updatedFinding := review.Finding{
		Severity:        "blocker",
		File:            "main.go",
		Line:            &line21,
		Title:           "Updated finding",
		Evidence:        "updatedCall()",
		FailureScenario: "runtime failure",
		Suggestion:      "guard it",
	}
	logger := &fakeLogger{}
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,4 @@\n+newCall()\n+updatedCall()\n+otherCall()\n"}},
		reviewComments: []ReviewComment{{
			ID:         55,
			AuthorType: "Bot",
			Body:       InlineMarker + " fingerprint=" + inlineFingerprint(updatedFinding) + " -->\nold",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true, Logger: logger})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{
		Summary: "summary",
		Findings: []review.Finding{
			{
				Severity:        "blocker",
				File:            "main.go",
				Line:            &line20,
				Title:           "Created finding",
				Evidence:        "newCall()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
			},
			updatedFinding,
			{
				Severity:        "blocker",
				File:            "main.go",
				Line:            &line20,
				Title:           "Low confidence",
				Evidence:        "newCall()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
				Confidence:      &lowConfidence,
			},
			{
				Severity:        "blocker",
				File:            "main.go",
				Line:            &line99,
				Title:           "Unmapped finding",
				Evidence:        "missingCall()",
				FailureScenario: "runtime failure",
				Suggestion:      "guard it",
			},
		},
	})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 1 || fake.updatedReviewID != 55 {
		t.Fatalf("expected created and updated inline comments: created=%+v updatedID=%d", fake.createdPullRequestReview, fake.updatedReviewID)
	}
	logLine := logger.String()
	for _, want := range []string{
		"inline comments completed repo=octo/repo pull=7",
		"findings=4",
		"eligible=3",
		"mapped=2",
		"created=1",
		"updated=1",
		"skipped_quality=1",
		"skipped_unmapped=1",
		"skipped_limit=0",
	} {
		if !strings.Contains(logLine, want) {
			t.Fatalf("log line missing %q:\n%s", want, logLine)
		}
	}
}

func TestPublisherUpdatesExistingInlineComment(t *testing.T) {
	line := 20
	finding := review.Finding{
		Severity:        "blocker",
		File:            "main.go",
		Line:            &line,
		Title:           "Bad call",
		Evidence:        "evidence",
		FailureScenario: "failure",
		Suggestion:      "fix",
	}
	fake := &fakeInlineCommenter{
		files: []review.FileChange{{Filename: "main.go", Patch: "@@ -1 +20,2 @@\n+badCall()\n"}},
		reviewComments: []ReviewComment{{
			ID:         55,
			AuthorType: "Bot",
			Body:       InlineMarker + " fingerprint=" + inlineFingerprint(finding) + " -->\nold",
		}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.PublishForHead(context.Background(), 42, "octo", "repo", 7, "abc123", review.ReviewResult{Summary: "summary", Findings: []review.Finding{finding}})
	if err != nil {
		t.Fatalf("PublishForHead() error = %v", err)
	}
	if fake.updatedReviewID != 55 || !strings.Contains(fake.updatedReviewBody, "Bad call") {
		t.Fatalf("updated id/body = %d %q", fake.updatedReviewID, fake.updatedReviewBody)
	}
	if len(fake.createdPullRequestReview.Comments) != 0 {
		t.Fatalf("created duplicate inline review: %+v", fake.createdPullRequestReview)
	}
}

func TestPublisherCleanupMarksOnlyServiceInlineCommentsInactive(t *testing.T) {
	fake := &fakeInlineCommenter{
		reviewComments: []ReviewComment{
			{ID: 55, AuthorType: "Bot", Body: InlineMarker + " fingerprint=aaaaaaaaaaaaaaaa -->\nold service comment"},
			{ID: 56, AuthorType: "Bot", Body: "other bot comment"},
			{ID: 57, AuthorType: "User", Body: InlineMarker + " fingerprint=bbbbbbbbbbbbbbbb -->\nhuman comment"},
			{ID: 58, AuthorType: "Bot", Body: InlineMarker + " -->\nmissing fingerprint"},
		},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: true})
	err := pub.Cleanup(context.Background(), review.CleanupJob{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        "abc123",
		State:          review.CleanupStateMerged,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if len(fake.createdPullRequestReview.Comments) != 0 || len(fake.fallbackCreatedReviews) != 0 || fake.createdReview.Body != "" {
		t.Fatalf("cleanup created inline output: review=%+v fallback=%+v direct=%+v", fake.createdPullRequestReview, fake.fallbackCreatedReviews, fake.createdReview)
	}
	got := fake.updatedReviewBodies[55]
	if !strings.Contains(got, InlineStaleMarker) || !strings.Contains(got, "inactive because this pull request was merged") || !strings.Contains(got, "aaaaaaaaaaaaaaaa") {
		t.Fatalf("updated service body = %q", got)
	}
	for _, id := range []int64{56, 57, 58} {
		if _, ok := fake.updatedReviewBodies[id]; ok {
			t.Fatalf("updated non-service comment id=%d", id)
		}
	}
}

func TestPublisherCleanupSkipsInlineWhenDisabled(t *testing.T) {
	fake := &fakeInlineCommenter{
		reviewComments: []ReviewComment{{ID: 55, AuthorType: "Bot", Body: InlineMarker + " fingerprint=aaaaaaaaaaaaaaaa -->\nold service comment"}},
	}
	pub := NewPublisherWithOptions(fake, PublisherOptions{InlineCommentsEnabled: false})
	err := pub.Cleanup(context.Background(), review.CleanupJob{
		InstallationID: 42,
		Owner:          "octo",
		Repo:           "repo",
		PullNumber:     7,
		HeadSHA:        "abc123",
		State:          review.CleanupStateClosed,
	})
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	if len(fake.updatedReviewBodies) != 0 {
		t.Fatalf("updated inline comments = %+v, want none", fake.updatedReviewBodies)
	}
}

func eligibleFinding(file string, line int, title, evidence string) review.Finding {
	return review.Finding{
		Severity:        "blocker",
		File:            file,
		Line:            &line,
		Title:           title,
		Evidence:        evidence,
		FailureScenario: "runtime failure",
		Suggestion:      "guard it",
	}
}

type fakeLogger struct {
	lines []string
}

func (l *fakeLogger) Printf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *fakeLogger) String() string {
	return strings.Join(l.lines, "\n")
}

type fakeInlineCommenter struct {
	fakeIssueCommenter
	files                    []review.FileChange
	reviewComments           []ReviewComment
	createdReview            ReviewCommentRequest
	fallbackCreatedReviews   []ReviewCommentRequest
	createdPullRequestReview PullRequestReviewRequest
	createReviewErr          error
	updatedReviewID          int64
	updatedReviewBody        string
	updatedReviewBodies      map[int64]string
	updateErrIDs             map[int64]error
}

func (f *fakeInlineCommenter) FetchPullRequestFiles(context.Context, int64, string, string, int) ([]review.FileChange, error) {
	return f.files, nil
}

func (f *fakeInlineCommenter) ListPullRequestReviewComments(context.Context, int64, string, string, int) ([]ReviewComment, error) {
	return f.reviewComments, nil
}

func (f *fakeInlineCommenter) CreatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req ReviewCommentRequest) error {
	f.createdReview = req
	f.fallbackCreatedReviews = append(f.fallbackCreatedReviews, req)
	return nil
}

func (f *fakeInlineCommenter) UpdatePullRequestReviewComment(ctx context.Context, installationID int64, owner, repo string, commentID int64, body string) error {
	if err := f.updateErrIDs[commentID]; err != nil {
		return err
	}
	f.updatedReviewID = commentID
	f.updatedReviewBody = body
	if f.updatedReviewBodies == nil {
		f.updatedReviewBodies = map[int64]string{}
	}
	f.updatedReviewBodies[commentID] = body
	return nil
}

func (f *fakeInlineCommenter) CreatePullRequestReview(ctx context.Context, installationID int64, owner, repo string, pullNumber int, req PullRequestReviewRequest) (PullRequestReview, error) {
	f.createdPullRequestReview = req
	if f.createReviewErr != nil {
		return PullRequestReview{}, f.createReviewErr
	}
	return PullRequestReview{ID: 71}, nil
}
