package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github-ai-reviewer/internal/reviewbench"
)

func main() {
	fixturePath := flag.String("fixture", "", "path to a review bench fixture JSON file")
	flag.Parse()
	if *fixturePath == "" {
		fmt.Fprintln(os.Stderr, "usage: review-bench -fixture testdata/review-bench/example.json")
		os.Exit(2)
	}
	data, err := os.ReadFile(*fixturePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read fixture: %v\n", err)
		os.Exit(1)
	}
	fixture, err := reviewbench.DecodeFixture(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode fixture: %v\n", err)
		os.Exit(1)
	}
	report, err := reviewbench.Run(context.Background(), fixture)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run bench: %v\n", err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
