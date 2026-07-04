package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github-ai-reviewer/internal/reviewbench"
)

func main() {
	fixturePath := flag.String("fixture", "", "path to a review bench fixture JSON file")
	fixturePatterns := flag.String("fixtures", "", "comma-separated fixture paths or glob patterns")
	flag.Parse()
	paths, err := fixturePaths(*fixturePath, *fixturePatterns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(2)
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: review-bench -fixture testdata/review-bench/example.json")
		fmt.Fprintln(os.Stderr, "   or: review-bench -fixtures 'testdata/review-bench/*.json,/tmp/review-fixture.json'")
		os.Exit(2)
	}
	fixtures := make([]reviewbench.Fixture, 0, len(paths))
	for _, path := range paths {
		fixture, err := readFixture(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		fixtures = append(fixtures, fixture)
	}
	var output any
	if len(fixtures) == 1 {
		report, err := reviewbench.Run(context.Background(), fixtures[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "run bench: %v\n", err)
			os.Exit(1)
		}
		output = report
	} else {
		report, err := reviewbench.RunSuite(context.Background(), fixtures)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run bench suite: %v\n", err)
			os.Exit(1)
		}
		output = report
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}

func readFixture(path string) (reviewbench.Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return reviewbench.Fixture{}, fmt.Errorf("read fixture %s: %w", path, err)
	}
	fixture, err := reviewbench.DecodeFixture(data)
	if err != nil {
		return reviewbench.Fixture{}, fmt.Errorf("decode fixture %s: %w", path, err)
	}
	return fixture, nil
}

func fixturePaths(single, multiple string) ([]string, error) {
	if strings.TrimSpace(single) != "" && strings.TrimSpace(multiple) != "" {
		return nil, fmt.Errorf("use either -fixture or -fixtures, not both")
	}
	if strings.TrimSpace(single) != "" {
		return []string{single}, nil
	}
	seen := map[string]struct{}{}
	paths := []string{}
	for _, item := range strings.Split(multiple, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		matches, err := filepath.Glob(item)
		if err != nil {
			return nil, fmt.Errorf("invalid fixture glob %q: %w", item, err)
		}
		if len(matches) == 0 {
			matches = []string{item}
		}
		for _, match := range matches {
			if _, ok := seen[match]; ok {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)
	return paths, nil
}
