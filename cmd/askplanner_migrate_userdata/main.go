package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"lab/askplanner/internal/migration"
)

func main() {
	var src string
	flag.StringVar(&src, "source", ".askplanner", "source askplanner data directory")
	flag.Usage = func() {
		_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [--source <askplanner_dir>] <destination_dir>\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	dst := flag.Arg(0)

	summary, err := migration.CopyAskplannerUserData(src, dst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate askplanner user data: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Migration completed.\n")
	fmt.Printf("Source: %s\n", src)
	fmt.Printf("Destination: %s\n", dst)
	fmt.Printf("Directories created: %d\n", summary.DirectoriesCreated)
	fmt.Printf("Files copied: %d\n", summary.FilesCopied)
	fmt.Printf("Symlinks created: %d\n", summary.SymlinksCreated)
	fmt.Printf("Bytes copied: %d\n", summary.BytesCopied)
}
