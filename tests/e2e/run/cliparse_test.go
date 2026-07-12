package run_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate run: CLI parser", func() {
	var ws *testutil.Workspace
	BeforeEach(func() { ws = testutil.New(GinkgoT().TempDir(), RunBinary) })

	const deploy = `#!/bin/sh
# ---
# summary: Deploy to an environment.
# args:
#   - name: env
#     choices: [staging, prod]
#     required: true
#   - name: files
#     variadic: true
# flags:
#   - name: --dry-run
#     short: -n
#     type: bool
#   - name: --tag
#     default: latest
# ---
echo "env=$taskgate_env tag=$taskgate_tag dry=$taskgate_dry_run n=$taskgate_files_count f1=$taskgate_files_1"
`

	It("injects taskgate_* env and empties argv", func() {
		ws.WriteFile(".taskgate/human/deploy", deploy, true)
		out := ws.Run("run", "deploy", "prod", "a.txt", "-n")
		Expect(out.ExitCode).To(Equal(0))
		Expect(out.Stdout).To(ContainSubstring("env=prod tag=latest dry=true n=1 f1=a.txt"))
	})

	It("rejects a bad choice with exit 2 and a usage line", func() {
		ws.WriteFile(".taskgate/human/deploy", deploy, true)
		out := ws.Run("run", "deploy", "dev")
		Expect(out.ExitCode).To(Equal(2))
		Expect(out.Stderr).To(ContainSubstring("must be one of staging, prod"))
		Expect(out.Stderr).To(ContainSubstring("Usage: taskgate run deploy"))
	})

	It("prints --help and exits 0 without running", func() {
		ws.WriteFile(".taskgate/human/deploy", deploy, true)
		out := ws.Run("run", "deploy", "--help")
		Expect(out.ExitCode).To(Equal(0))
		Expect(out.Stdout).To(ContainSubstring("Usage: taskgate run deploy"))
		Expect(out.Stdout).To(ContainSubstring("Deploy to an environment."))
	})

	It("passes args through unchanged when no spec is declared", func() {
		ws.WriteFile(".taskgate/human/raw", "#!/bin/sh\necho \"got: $1 $2\"\n", true)
		out := ws.Run("run", "raw", "one", "two")
		Expect(out.ExitCode).To(Equal(0))
		Expect(strings.TrimSpace(out.Stdout)).To(Equal("got: one two"))
	})
})
