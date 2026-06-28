package show_test

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate show: task inspection prints path, summary, and body", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("task with summary and body", func() {
		It("shows all three sections", func() {
			ws.WriteAnnotatedTask(
				".taskgate/human/build",
				"Build the project.",
				testutil.Lines(
					"Reads VERSION from the environment.",
					"Exits non-zero on build failure.",
				),
			)
			out := ws.Run("show", "build")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(testutil.MatchGolden("inspect_task_with_body"))
		})
	})
})

var _ = Describe("taskgate show: task with no body omits body section entirely", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("task with summary only", func() {
		It("shows path and summary, no body section", func() {
			ws.WriteAnnotatedTask(".taskgate/human/build", "Build the project.", "")
			out := ws.Run("show", "build")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(testutil.MatchGolden("inspect_task_no_body"))
		})
	})
})

var _ = Describe("taskgate ai show: FR-006 — AI task envelope shape", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("task envelope", func() {
		It("has the expected fields", func() {
			ws.WriteAnnotatedTask(".taskgate/ai/analyze", "Analyze the codebase.", "Reads CONFIG from environment.")
			out := ws.Run("ai", "show", "analyze")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())

			var envelope map[string]interface{}
			Expect(json.Unmarshal([]byte(out.Stdout), &envelope)).To(Succeed())
			Expect(envelope["kind"]).To(Equal("task"))
			Expect(envelope["path"]).To(Equal(".taskgate/ai/analyze"))
			Expect(envelope["audience"]).To(Equal("ai"))

			// summary and body may have trailing newline from YAML literal-block; strip it
			summary, _ := envelope["summary"].(string)
			body, _ := envelope["body"].(string)
			Expect(strings.TrimRight(summary, "\n")).To(Equal("Analyze the codebase."))
			Expect(strings.TrimRight(body, "\n")).To(Equal("Reads CONFIG from environment."))
		})
	})
})

var _ = Describe("taskgate ai show: ADR-0003 — bare task has null summary", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("bare task with no annotation", func() {
		It("emits task envelope with summary set to null", func() {
			ws.WriteBareTask(".taskgate/ai/bare")
			out := ws.Run("ai", "show", "bare")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())

			var envelope map[string]interface{}
			Expect(json.Unmarshal([]byte(out.Stdout), &envelope)).To(Succeed())
			Expect(envelope["kind"]).To(Equal("task"))
			Expect(envelope["path"]).To(Equal(".taskgate/ai/bare"))
			Expect(envelope).To(HaveKey("summary"))
			Expect(envelope["summary"]).To(BeNil())
		})
	})
})
