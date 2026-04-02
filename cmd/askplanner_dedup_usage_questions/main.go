package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lab/askplanner/internal/config"
	"lab/askplanner/internal/usage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatalf("load config: %v", err)
	}

	defaultPath := cfg.UsageQuestionsPath
	var src string
	var out string
	flag.StringVar(&src, "source", defaultPath, "source usage_questions.jsonl path")
	flag.StringVar(&out, "output", "", "output path; defaults to in-place rewrite")
	flag.Usage = func() {
		name := filepath.Base(os.Args[0])
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [--source <usage_questions.jsonl>] [--output <deduped.jsonl>]\n", name)
		flag.PrintDefaults()
	}
	flag.Parse()

	src = strings.TrimSpace(src)
	out = strings.TrimSpace(out)
	if src == "" {
		fatalf("source path is empty")
	}
	if out == "" {
		out = src
	}

	summary, err := usage.DedupQuestionEventsFile(src, out)
	if err != nil {
		fatalf("dedup usage questions: %v", err)
	}

	fmt.Printf("Dedup completed.\n")
	fmt.Printf("Source: %s\n", src)
	fmt.Printf("Output: %s\n", out)
	fmt.Printf("Lines read: %d\n", summary.LinesRead)
	fmt.Printf("Lines written: %d\n", summary.LinesWritten)
	fmt.Printf("Duplicates removed: %d\n", summary.DuplicatesRemoved)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
