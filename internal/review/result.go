package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrMalformedResult   = errors.New("malformed_result")
	ErrNoUsefulContent   = errors.New("no_useful_content")
	ErrInvalidSeverity   = errors.New("invalid_severity")
	ErrInvalidRiskScore  = errors.New("invalid_risk_score")
	ErrInvalidConfidence = errors.New("invalid_confidence")
	ErrInvalidFinding    = errors.New("invalid_finding")
)

type ReviewResult struct {
	Summary      string    `json:"summary"`
	RiskScore    *int      `json:"risk_score,omitempty"`
	Findings     []Finding `json:"findings"`
	MissingTests []string  `json:"missing_tests"`
	Limitations  []string  `json:"limitations"`
}

type Finding struct {
	Severity        string   `json:"severity"`
	Category        string   `json:"category,omitempty"`
	File            string   `json:"file,omitempty"`
	Line            *int     `json:"line,omitempty"`
	Title           string   `json:"title"`
	Evidence        string   `json:"evidence"`
	FailureScenario string   `json:"failure_scenario"`
	Suggestion      string   `json:"suggestion"`
	Confidence      *float64 `json:"confidence,omitempty"`
}

func ParseReviewResult(raw string) (ReviewResult, error) {
	text := extractJSON(strings.TrimSpace(raw))
	if text == "" {
		return ReviewResult{}, ErrMalformedResult
	}
	var result ReviewResult
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return ReviewResult{}, fmt.Errorf("%w: %v", ErrMalformedResult, err)
	}
	if decoder.More() {
		return ReviewResult{}, ErrMalformedResult
	}
	if err := result.NormalizeValidate(); err != nil {
		return ReviewResult{}, err
	}
	return result, nil
}

func (r *ReviewResult) NormalizeValidate() error {
	r.Summary = strings.TrimSpace(r.Summary)
	r.MissingTests = normalizeStringList(r.MissingTests)
	r.Limitations = normalizeStringList(r.Limitations)
	if r.RiskScore != nil && (*r.RiskScore < 0 || *r.RiskScore > 100) {
		return ErrInvalidRiskScore
	}
	for i := range r.Findings {
		if err := r.Findings[i].normalizeValidate(); err != nil {
			return err
		}
	}
	if !r.HasUsefulContent() {
		return ErrNoUsefulContent
	}
	return nil
}

func (r ReviewResult) HasUsefulContent() bool {
	return strings.TrimSpace(r.Summary) != "" || len(r.Findings) > 0 || len(r.MissingTests) > 0 || len(r.Limitations) > 0
}

func (f *Finding) normalizeValidate() error {
	f.Severity = strings.ToLower(strings.TrimSpace(f.Severity))
	if !allowedSeverity(f.Severity) {
		return ErrInvalidSeverity
	}
	f.Category = strings.TrimSpace(f.Category)
	f.File = strings.TrimSpace(f.File)
	f.Title = strings.TrimSpace(f.Title)
	f.Evidence = strings.TrimSpace(f.Evidence)
	f.FailureScenario = strings.TrimSpace(f.FailureScenario)
	f.Suggestion = strings.TrimSpace(f.Suggestion)
	if f.Title == "" || f.Evidence == "" || f.FailureScenario == "" || f.Suggestion == "" {
		return ErrInvalidFinding
	}
	if f.Line != nil && *f.Line <= 0 {
		return ErrInvalidFinding
	}
	if f.Confidence != nil && (*f.Confidence < 0 || *f.Confidence > 1) {
		return ErrInvalidConfidence
	}
	return nil
}

func allowedSeverity(severity string) bool {
	switch severity {
	case "blocker", "warning", "suggestion", "question":
		return true
	default:
		return false
	}
}

func normalizeStringList(values []string) []string {
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func extractJSON(text string) string {
	if strings.HasPrefix(text, "```") && strings.HasSuffix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			first := strings.TrimSpace(lines[0])
			if first == "```" || strings.EqualFold(first, "```json") {
				return strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
			}
		}
	}
	return text
}
