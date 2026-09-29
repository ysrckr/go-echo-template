package migrate

import (
	"io/fs"
	"strings"
	"testing"
)

// TestEmbeddedMigrationsArePresent guards the go:embed path. If the directory
// moves or the pattern stops matching, the binary still compiles and simply
// migrates nothing — a silent failure worth a test.
func TestEmbeddedMigrationsArePresent(t *testing.T) {
	root, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("sub: %v", err)
	}

	// goose globs "*.sql" at the root of the FS it is given.
	files, err := fs.Glob(root, "*.sql")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no migrations embedded: check the go:embed pattern and fs.Sub root")
	}

	for _, name := range files {
		content, err := fs.ReadFile(root, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		// goose ignores a .sql file that carries no annotations.
		if !strings.Contains(string(content), "-- +goose Up") {
			t.Errorf("%s is missing a '-- +goose Up' annotation", name)
		}
	}
}
