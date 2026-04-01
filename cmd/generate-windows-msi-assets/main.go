package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/liuhaotian/xhs-local-helper/internal/windowsbundle"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: generate-windows-msi-assets <repo-root> <installer-dir> <version>")
		os.Exit(1)
	}

	repoRoot := os.Args[1]
	installerDir := os.Args[2]
	version := os.Args[3]

	if err := os.MkdirAll(installerDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create installer dir: %v\n", err)
		os.Exit(1)
	}

	layout := windowsbundle.RepoLayout(repoRoot)
	source, err := windowsbundle.RenderMSIWixSource(layout, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render wix source: %v\n", err)
		os.Exit(1)
	}

	path := filepath.Join(installerDir, "Product.wxs")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write Product.wxs: %v\n", err)
		os.Exit(1)
	}
}
