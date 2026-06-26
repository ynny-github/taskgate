package show_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TaskgateBinary is built once per suite by BeforeSuite and reused across specs.
var TaskgateBinary string

func TestShow(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Show Subcommand Suite")
}

var _ = BeforeSuite(func() {
	tmpDir, err := os.MkdirTemp("", "taskgate-bin-")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { os.RemoveAll(tmpDir) })

	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	// tests/e2e/show -> repo root (3 levels up)
	repoRoot := filepath.Join(cwd, "..", "..", "..")

	binary := filepath.Join(tmpDir, "taskgate")
	cmd := exec.Command("go", "build", "-o", binary, "./taskgate")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "go build failed: %s", output)

	TaskgateBinary = binary
})
