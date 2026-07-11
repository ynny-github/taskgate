package run_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var RunBinary string

func TestRun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Run Dependency Suite")
}

var _ = BeforeSuite(func() {
	tmpDir, err := os.MkdirTemp("", "taskgate-bin-")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(func() { os.RemoveAll(tmpDir) })

	cwd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	repoRoot := filepath.Join(cwd, "..", "..", "..") // tests/e2e/run -> repo root

	binary := filepath.Join(tmpDir, "taskgate")
	cmd := exec.Command("go", "build", "-o", binary, "./taskgate")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "go build failed: %s", output)

	RunBinary = binary
})
