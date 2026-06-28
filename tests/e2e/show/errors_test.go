package show_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

// collision content (moved from collision_test.go)
var _ = Describe("taskgate show: FR-013 — collision is a hard error", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
		ws.WriteAnnotatedTask(".taskgate/human/build", "human variant", "")
		ws.WriteAnnotatedTask(".taskgate/shared/build", "shared variant", "")
	})

	Context("no argument", func() {
		It("exits 4, stdout empty, stderr lists both paths", func() {
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(4))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring(".taskgate/human/build"))
			Expect(out.Stderr).To(ContainSubstring(".taskgate/shared/build"))
		})
	})

	Context("explicit name", func() {
		It("exits 4, stdout empty, stderr lists both paths", func() {
			out := ws.Run("show", "build")
			Expect(out.ExitCode).To(Equal(4))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring(".taskgate/human/build"))
			Expect(out.Stderr).To(ContainSubstring(".taskgate/shared/build"))
		})
	})
})

var _ = Describe("taskgate show: FR-014 — not-found name exits 3", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
		ws.WriteAnnotatedTask(".taskgate/human/build", "Build.", "")
	})

	Context("unknown task name", func() {
		It("exits 3 with scope in stderr", func() {
			out := ws.Run("show", "no-such-task")
			Expect(out.ExitCode).To(Equal(3))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring("not found"))
			Expect(out.Stderr).To(ContainSubstring(".taskgate/human"))
			Expect(out.Stderr).To(ContainSubstring(".taskgate/shared"))
		})
	})
})

var _ = Describe("taskgate show: FR-015 — filesystem-shaped arguments are rejected", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
		ws.WriteAnnotatedTask(".taskgate/human/build", "Build.", "")
	})

	Context("taskgate-prefixed path", func() {
		It("is rejected with exit 2", func() {
			out := ws.Run("show", ".taskgate/human/build")
			Expect(out.ExitCode).To(Equal(2))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring("run-style"))
		})
	})

	Context("absolute path", func() {
		It("is rejected with exit 2", func() {
			out := ws.Run("show", "/abs/path")
			Expect(out.ExitCode).To(Equal(2))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring("run-style"))
		})
	})

	Context("dot-slash path", func() {
		It("is rejected with exit 2", func() {
			out := ws.Run("show", "./build")
			Expect(out.ExitCode).To(Equal(2))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring("run-style"))
		})
	})

	Context("empty string argument", func() {
		It("is rejected with exit 2", func() {
			out := ws.Run("show", "")
			Expect(out.ExitCode).To(Equal(2))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring("run-style"))
		})
	})
})

var _ = Describe("taskgate show: FR-016 — missing workspace exits 5", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		// No .taskgate/ directory — workspace is empty
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("no .taskgate/ directory", func() {
		It("exits 5", func() {
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(5))
			Expect(out.Stdout).To(BeEmpty())
			Expect(out.Stderr).To(ContainSubstring(".taskgate/"))
		})
	})
})

var _ = Describe("taskgate show: legacy list subcommand removed", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("taskgate list", func() {
		It("is unknown", func() {
			out := ws.Run("list")
			Expect(out.ExitCode).To(Equal(1))
			Expect(out.Stderr).To(ContainSubstring("unknown command"))
		})
	})

	Context("taskgate ai list", func() {
		It("is unknown", func() {
			out := ws.Run("ai", "list")
			Expect(out.ExitCode).To(Equal(1))
			Expect(out.Stderr).To(ContainSubstring("unknown command"))
		})
	})
})
