package airun_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate ai run: dependency lifecycle from snapshot", func() {
	var ws *testutil.Workspace
	var stateHome string

	BeforeEach(func() {
		root := GinkgoT().TempDir()
		ws = testutil.New(root, AIRunBinary)
		stateHome = filepath.Join(root, "state")
		Expect(os.MkdirAll(stateHome, 0o755)).To(Succeed())
	})

	runWithState := func(args ...string) testutil.Result {
		// testutil.Run does not set env; shell out here with XDG_STATE_HOME.
		return ws.RunEnv([]string{"XDG_STATE_HOME=" + stateHome}, args...)
	}

	It("runs a before dependency from the installed snapshot", func() {
		ws.WriteDependentTask(".taskgate/ai/build", "build", nil, nil)
		ws.WriteDependentTask(".taskgate/ai/deploy", "deploy", []string{"build"}, nil)

		Expect(runWithState("snapshot", "install").ExitCode).To(Equal(0))
		out := runWithState("ai", "run", "deploy")
		Expect(out.ExitCode).To(Equal(0))
		Expect(strings.Fields(ws.ReadFile("order.txt"))).To(Equal([]string{"build", "deploy"}))
	})

	It("blocks when a dependency's snapshot is stale", func() {
		ws.WriteDependentTask(".taskgate/ai/build", "build", nil, nil)
		ws.WriteDependentTask(".taskgate/ai/deploy", "deploy", []string{"build"}, nil)
		Expect(runWithState("snapshot", "install").ExitCode).To(Equal(0))

		// Mutate the source so build's snapshot is now out of date.
		ws.WriteDependentTask(".taskgate/ai/build", "build2", nil, nil)

		out := runWithState("ai", "run", "deploy")
		Expect(out.ExitCode).NotTo(Equal(0))
		Expect(out.Stderr).To(ContainSubstring("out of date"))
	})
})
