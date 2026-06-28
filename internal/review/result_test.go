package review

import (
	"errors"
	"testing"
)

func TestParseReviewResultAcceptsValidFencedJSON(t *testing.T) {
	line := 17
	confidence := 0.8
	raw := "```json\n" + `{
		"summary": "Looks focused.",
		"risk_score": 25,
		"findings": [{
			"severity": "warning",
			"category": "correctness",
			"file": "main.go",
			"line": 17,
			"title": "Nil response can panic",
			"evidence": "handler dereferences resp before checking err",
			"failure_scenario": "network failure returns nil resp",
			"suggestion": "check err before using resp",
			"confidence": 0.8
		}],
		"missing_tests": ["network error path"],
		"limitations": ["Only diff context was available."]
	}` + "\n```"

	got, err := ParseReviewResult(raw)
	if err != nil {
		t.Fatalf("ParseReviewResult() error = %v", err)
	}
	if got.Summary != "Looks focused." || got.RiskScore == nil || *got.RiskScore != 25 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if len(got.Findings) != 1 {
		t.Fatalf("findings len = %d", len(got.Findings))
	}
	f := got.Findings[0]
	if f.Line == nil || *f.Line != line || f.Confidence == nil || *f.Confidence != confidence {
		t.Fatalf("finding line/confidence = %+v", f)
	}
}

func TestParseReviewResultAllowsOptionalFindingLine(t *testing.T) {
	raw := `{"summary":"See finding.","findings":[{"severity":"question","title":"Clarify behavior","evidence":"No caller context is present","failure_scenario":"Reviewer cannot determine intended behavior","suggestion":"Document expected behavior"}]}`

	got, err := ParseReviewResult(raw)
	if err != nil {
		t.Fatalf("ParseReviewResult() error = %v", err)
	}
	if got.Findings[0].Line != nil {
		t.Fatalf("line = %v, want nil", *got.Findings[0].Line)
	}
}

func TestParseReviewResultRejectsNoUsefulContent(t *testing.T) {
	_, err := ParseReviewResult(`{"summary":" ","findings":[],"missing_tests":[],"limitations":[]}`)
	if !errors.Is(err, ErrNoUsefulContent) {
		t.Fatalf("error = %v, want ErrNoUsefulContent", err)
	}
}

func TestParseReviewResultRejectsInvalidSeverity(t *testing.T) {
	_, err := ParseReviewResult(`{"summary":"See finding.","findings":[{"severity":"critical","title":"Bad","evidence":"e","failure_scenario":"f","suggestion":"s"}]}`)
	if !errors.Is(err, ErrInvalidSeverity) {
		t.Fatalf("error = %v, want ErrInvalidSeverity", err)
	}
}

func TestParseReviewResultRejectsInvalidRiskScore(t *testing.T) {
	_, err := ParseReviewResult(`{"summary":"Looks focused.","risk_score":101}`)
	if !errors.Is(err, ErrInvalidRiskScore) {
		t.Fatalf("error = %v, want ErrInvalidRiskScore", err)
	}
}

func TestParseReviewResultRejectsInvalidConfidence(t *testing.T) {
	_, err := ParseReviewResult(`{"summary":"See finding.","findings":[{"severity":"warning","title":"Bad","evidence":"e","failure_scenario":"f","suggestion":"s","confidence":1.5}]}`)
	if !errors.Is(err, ErrInvalidConfidence) {
		t.Fatalf("error = %v, want ErrInvalidConfidence", err)
	}
}

func TestParseReviewResultRejectsMissingFindingFields(t *testing.T) {
	_, err := ParseReviewResult(`{"summary":"See finding.","findings":[{"severity":"warning","title":"Bad","evidence":"e","failure_scenario":"f"}]}`)
	if !errors.Is(err, ErrInvalidFinding) {
		t.Fatalf("error = %v, want ErrInvalidFinding", err)
	}
}
