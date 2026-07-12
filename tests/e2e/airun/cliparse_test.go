package airun_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate ai run: CLI parser", func() {
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

	const deploy = `#!/bin/sh
# ---
# summary: Deploy.
# args:
#   - name: env
#     choices: [staging, prod]
#     required: true
# ---
echo "env=$taskgate_env"
`

	It("injects env under ai run", func() {
		ws.WriteFile(".taskgate/ai/deploy", deploy, true)
		Expect(runWithState("snapshot", "install").ExitCode).To(Equal(0))
		out := runWithState("ai", "run", "deploy", "prod")
		Expect(out.ExitCode).To(Equal(0))
		Expect(out.Stdout).To(ContainSubstring("env=prod"))
	})

	It("rejects a bad choice with exit 2", func() {
		ws.WriteFile(".taskgate/ai/deploy", deploy, true)
		Expect(runWithState("snapshot", "install").ExitCode).To(Equal(0))
		out := runWithState("ai", "run", "deploy", "dev")
		Expect(out.ExitCode).To(Equal(2))
		Expect(out.Stderr).To(ContainSubstring("must be one of staging, prod"))
	})
})
