package show_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate show: directory lists its immediate children", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory with task children", func() {
		It("shows path header then a one-level tree, no summary/body", func() {
			ws.WriteAnnotatedTask(".taskgate/human/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod", "Promote to production.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(testutil.MatchGolden("dir_children"))
		})
	})
})

var _ = Describe("taskgate show: directory listing is one level deep", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("nested sub-directory", func() {
		It("appears as a single row, not expanded", func() {
			ws.WriteAnnotatedTask(".taskgate/human/deploy/prod/run", "Run a prod deploy.", "")
			out := ws.Run("show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(testutil.Lines(
				".taskgate/human/deploy",
				"",
				"  prod/",
				"",
			)))
		})
	})
})

var _ = Describe("taskgate show: non-executable files are hidden", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("a directory with an executable and a non-executable file", func() {
		It("lists only the executable one", func() {
			ws.WriteAnnotatedTask(".taskgate/human/tools/run", "Runnable.", "")
			ws.WriteFile(".taskgate/human/tools/notes.txt", "just notes\n", false)
			out := ws.Run("show", "tools")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stdout).To(ContainSubstring("run"))
			Expect(out.Stdout).NotTo(ContainSubstring("notes.txt"))
		})
	})

	Context("a named non-executable file", func() {
		It("is not found", func() {
			ws.WriteFile(".taskgate/human/notes.txt", "just notes\n", false)
			out := ws.Run("show", "notes.txt")
			Expect(out.ExitCode).To(Equal(3))
			Expect(out.Stderr).To(ContainSubstring("not found"))
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
			Expect(out.Stdout).To(ContainSubstring("child00"))
			Expect(out.Stdout).To(ContainSubstring("child49"))
		})
	})
})

var _ = Describe("taskgate ai show: directory envelope shape", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("directory envelope", func() {
		It("has child entries and no summary/body", func() {
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/canary", "Promote to canary.", "")
			ws.WriteAnnotatedTask(".taskgate/shared/deploy/prod", "Promote to production.", "")
			out := ws.Run("ai", "show", "deploy")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())

			var envelope map[string]interface{}
			Expect(json.Unmarshal([]byte(out.Stdout), &envelope)).To(Succeed())
			Expect(envelope["kind"]).To(Equal("directory"))
			Expect(envelope["path"]).To(Equal(".taskgate/shared/deploy"))
			Expect(envelope).NotTo(HaveKey("summary"))
			Expect(envelope).NotTo(HaveKey("body"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/deploy/canary"))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/deploy/prod"))
			Expect(envelope["name"]).To(Equal("deploy"))
			Expect(out.Stdout).To(ContainSubstring(`"name":"deploy/canary"`))
			Expect(out.Stdout).To(ContainSubstring(`"name":"deploy/prod"`))
		})
	})
})
