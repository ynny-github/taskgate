package show_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate show: FR-001 — root browse merges human/ and shared/", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("annotated tasks from both buckets", func() {
		It("appear in merged view with summaries", func() {
			ws.WriteAnnotatedTask(".taskgate/human/build", "Build the project for the current platform.", "")
			ws.WriteAnnotatedTask(".taskgate/shared/lint", "Lint the codebase with project rules.", "")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(
				".taskgate/human/build\tBuild the project for the current platform.\n" +
					".taskgate/shared/lint\tLint the codebase with project rules.\n",
			))
		})
	})
})

var _ = Describe("taskgate show: unannotated tasks still appear in root browse", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("bare task with no annotation", func() {
		It("appears with path only, no error", func() {
			ws.WriteBareTask(".taskgate/shared/bare")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(".taskgate/shared/bare\n"))
		})
	})
})

var _ = Describe("taskgate ai show: FR-001 — ai browse merges shared/ and ai/, excludes human/", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
		ws.WriteAnnotatedTask(".taskgate/human/build", "Build.", "")
		ws.WriteAnnotatedTask(".taskgate/shared/lint", "Lint.", "")
		ws.WriteAnnotatedTask(".taskgate/ai/analyze", "Analyze.", "")
	})

	Context("ai show with no argument", func() {
		It("merges shared and ai buckets, excludes human", func() {
			out := ws.Run("ai", "show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())

			var envelope map[string]interface{}
			Expect(json.Unmarshal([]byte(out.Stdout), &envelope)).To(Succeed())
			Expect(envelope["kind"]).To(Equal("listing"))
			Expect(envelope["audience"]).To(Equal("ai"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/ai/analyze"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/lint"))
			Expect(out.Stdout).NotTo(ContainSubstring(".taskgate/human/build"))
		})
	})
})
