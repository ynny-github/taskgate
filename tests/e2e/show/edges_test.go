package show_test

import (
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate show: unreadable file does not abort listing", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("unreadable task", func() {
		It("is skipped; other entries still appear", func() {
			if runtime.GOOS == "windows" {
				Skip("POSIX-only")
			}
			ws.WriteAnnotatedTask(".taskgate/human/locked", "Locked.", "")
			ws.MakeUnreadable(".taskgate/human/locked")
			ws.WriteAnnotatedTask(".taskgate/shared/lint", "Lint.", "")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/shared/lint"))
			Expect(out.Stdout).To(ContainSubstring("Lint."))
		})
	})
})

var _ = Describe("taskgate show: whitespace-only summary treated as empty", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("whitespace-only summary", func() {
		It("renders as path-only entry", func() {
			ws.WriteAnnotatedTask(".taskgate/human/build", "   ", "")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(".taskgate/human/build\n"))
		})
	})
})

var _ = Describe("taskgate show: leading comments before YAML envelope are skipped", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("shellcheck pragma and copyright header before annotation", func() {
		It("do not interfere", func() {
			ws.WriteLeadingCommentsTask(".taskgate/human/build", "Build the project.")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stderr).To(BeEmpty())
			Expect(out.Stdout).To(Equal(".taskgate/human/build\tBuild the project.\n"))
		})
	})
})

var _ = Describe("taskgate show: symlinks escaping .taskgate/ are listed but not read", func() {
	var ws *testutil.Workspace

	BeforeEach(func() {
		ws = testutil.New(GinkgoT().TempDir(), TaskgateBinary)
	})

	Context("symlink escape", func() {
		It("appears in listing but target content is not read", func() {
			if runtime.GOOS == "windows" {
				Skip("POSIX-only")
			}
			ws.Symlink(".taskgate/human/escapee", "../../outside")
			ws.WriteAnnotatedTask("outside", "Secret outside summary.", "")
			out := ws.Run("show")
			Expect(out.ExitCode).To(Equal(0))
			Expect(out.Stdout).To(ContainSubstring(".taskgate/human/escapee"))
			Expect(out.Stdout).NotTo(ContainSubstring("Secret outside summary"))
			Expect(out.Stderr).To(ContainSubstring(".taskgate/human/escapee"))
		})
	})
})
