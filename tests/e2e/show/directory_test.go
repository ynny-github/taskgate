package show_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate show: directory with _index shows path, summary, body, then children", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory with _index", func() {
		It("shows header and child rows", func() {
			ws.WriteIndex(".taskgate/human/deploy/_index", "Promote a build to an environment.", "Idempotent across reruns.", "# ")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod", "Promote to production.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines(
				".taskgate/human/deploy",
				"",
				"  Promote a build to an environment.",
				"",
				"Idempotent across reruns.",
				"",
				testutil.Cols(".taskgate/human/deploy/canary", "Promote to canary."),
				testutil.Cols(".taskgate/human/deploy/prod", "Promote to production."),
				"",
			)))
		})
	})
})

var _ = Describe("taskgate show: directory without _index shows path only then children", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory without _index", func() {
		It("shows path header and child rows", func() {
			ws.WriteAnnotatedTask(".taskgate/human/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod", "Promote to production.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines(
				".taskgate/human/deploy",
				"",
				testutil.Cols(".taskgate/human/deploy/canary", "Promote to canary."),
				testutil.Cols(".taskgate/human/deploy/prod", "Promote to production."),
				"",
			)))
		})
	})
})

var _ = Describe("taskgate show: directory listing is not recursive", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("nested sub-directory", func() {
		It("appears as a single row, not expanded", func() {
			ws.WriteIndex(".taskgate/human/deploy/_index", "Promote a build.", "", "# ")
			ws.WriteIndex(".taskgate/human/deploy/prod/_index", "Prod target.", "", "# ")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod/run", "Run a prod deploy.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines(
				".taskgate/human/deploy",
				"",
				"  Promote a build.",
				"",
				testutil.Cols(".taskgate/human/deploy/prod/", "Prod target."),
				"",
			)))
		})
	})
})

var _ = Describe("taskgate show: malformed _index does not abort listing", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("malformed _index", func() {
		It("yields path-only directory row; children still shown", func() {
			ws.WriteMalformedIndex(".taskgate/human/deploy/_index")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod", "Promote to production.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/human/deploy"))
			Expect(out.Stdout).To(ContainSubstring("Promote to production."))
			Expect(out.Stdout).NotTo(ContainSubstring("Promote a build"))
		})
	})
})

var _ = Describe("taskgate show: runnable _index supplies annotation and is not double-listed", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("runnable _index", func() {
		It("provides annotation without appearing as a child entry", func() {
			ws.WriteRunnableIndex(".taskgate/human/deploy/_index", "Promote a build.", "Idempotent.")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod", "Promote to production.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines(
				".taskgate/human/deploy",
				"",
				"  Promote a build.",
				"",
				"Idempotent.",
				"",
				testutil.Cols(".taskgate/human/deploy/prod", "Promote to production."),
				"",
			)))
		})
	})
})

var _ = Describe("taskgate show: no truncation with many children", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("50 children", func() {
		It("all appear in listing without truncation", func() {
			ws.WriteManyBareTasks(".taskgate/human/many", 50)
			out := ws.Run("show", "many")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(ContainSubstring(".taskgate/human/many/child00"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/human/many/child49"))
		})
	})
})

var _ = Describe("taskgate ai show: FR-007 — directory envelope shape", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory envelope", func() {
		It("contains summary and child entries", func() {
			ws.WriteIndex(".taskgate/shared/deploy/_index", "Promote a build.", "", "# ")
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/prod", "Promote to production.", "")
			out := ws.Run("ai", "show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())

			var envelope map[string]interface{}
			Expect(json.Unmarshal([]byte(out.Stdout), &envelope)).To(Succeed())
			Expect(envelope["kind"]).To(Equal("directory"))
			Expect(envelope["path"]).To(Equal(".taskgate/shared/deploy"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/deploy/canary"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/deploy/prod"))
		})
	})
})
