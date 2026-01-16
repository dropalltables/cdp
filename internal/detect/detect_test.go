package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeDetection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"next":"^15"},"scripts":{"build":"next build","start":"next start"}}`), 0644)

	result, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Framework != "Next.js" {
		t.Errorf("expected Next.js, got %s", result.Framework)
	}
	if result.PackageManager != "npm" {
		t.Errorf("expected npm, got %s", result.PackageManager)
	}

	// Test Dockerfile generation
	os.Chdir(dir)
	dockerfile := result.GenerateDockerfile()
	if !strings.Contains(dockerfile, "Coolpack") {
		t.Errorf("expected coolpack-generated Dockerfile, got:\n%s", dockerfile)
	}
	t.Logf("Generated Dockerfile:\n%s", dockerfile[:200])
}

func TestGoDetection(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\ngo 1.22"), 0644)

	result, err := Detect(dir)
	if err != nil {
		t.Fatal(err)
	}

	if result.Framework != "Go" {
		t.Errorf("expected Go, got %s", result.Framework)
	}
	if result.Kind != "go" {
		t.Errorf("expected kind go, got %s", result.Kind)
	}

	dockerfile := result.GenerateDockerfile()
	if !strings.Contains(dockerfile, "golang") {
		t.Errorf("expected golang Dockerfile, got:\n%s", dockerfile)
	}
}
