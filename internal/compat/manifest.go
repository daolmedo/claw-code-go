package compat

import (
	"fmt"
	"os"
	"path/filepath"
)

// SourceManifest holds a listing of files from the upstream TypeScript source.
type SourceManifest struct {
	Files   []string `json:"files"`
	Version string   `json:"version"`
}

// LoadManifest walks the given source directory and returns a manifest of all files found.
// It does not parse TypeScript — it only lists the directory structure.
func LoadManifest(srcDir string) (*SourceManifest, error) {
	info, err := os.Stat(srcDir)
	if err != nil {
		return nil, fmt.Errorf("stat source dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source dir is not a directory: %s", srcDir)
	}

	manifest := &SourceManifest{
		Version: "unknown",
	}

	// Try to read a version file if present
	for _, versionFile := range []string{"package.json", "version.txt", "VERSION"} {
		vPath := filepath.Join(srcDir, versionFile)
		if data, err := os.ReadFile(vPath); err == nil {
			if versionFile == "version.txt" || versionFile == "VERSION" {
				manifest.Version = string(data)
			} else {
				// Rough extraction: just store that we found package.json
				manifest.Version = "see " + vPath
			}
			_ = data
			break
		}
	}

	err = filepath.Walk(srcDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !fi.IsDir() {
			manifest.Files = append(manifest.Files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk source dir: %w", err)
	}

	return manifest, nil
}
