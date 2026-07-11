package run_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ynny-github/taskgate/tests/e2e/testutil"
)

var _ = Describe("taskgate run: before/after dependency lifecycle", func() {
	var ws *testutil.Workspace

	BeforeEach(func() { ws = testutil.New(GinkgoT().TempDir(), RunBinary) })

	It("runs before deps, then the target, then after deps (immediate order)", func() {
		ws.WriteDependentTask(".taskgate/human/build", "build", nil, []string{"clean"})
		ws.WriteDependentTask(".taskgate/human/clean", "clean", nil, nil)
		ws.WriteDependentTask(".taskgate/human/notify", "notify", nil, nil)
		ws.WriteDependentTask(".taskgate/human/deploy", "deploy", []string{"build"}, []string{"notify"})

		out := ws.Run("run", "deploy")
		Expect(out.ExitCode).To(Equal(0))
		Expect(strings.Fields(ws.ReadFile("order.txt"))).To(Equal([]string{"build", "clean", "deploy", "notify"}))
	})

	It("runs a diamond-shared dependency exactly once", func() {
		ws.WriteDependentTask(".taskgate/human/d", "d", nil, nil)
		ws.WriteDependentTask(".taskgate/human/b", "b", []string{"d"}, nil)
		ws.WriteDependentTask(".taskgate/human/c", "c", []string{"d"}, nil)
		ws.WriteDependentTask(".taskgate/human/a", "a", []string{"b", "c"}, nil)

		out := ws.Run("run", "a")
		Expect(out.ExitCode).To(Equal(0))
		count := 0
		for _, f := range strings.Fields(ws.ReadFile("order.txt")) {
			if f == "d" {
				count++
			}
		}
		Expect(count).To(Equal(1))
	})

	It("aborts the target when a before dependency fails", func() {
		ws.WriteFile(".taskgate/human/build",
			"#!/bin/sh\necho build >> \""+ws.Root+"/order.txt\"\nexit 5\n", true)
		ws.WriteDependentTask(".taskgate/human/deploy", "deploy", []string{"build"}, []string{"notify"})
		ws.WriteDependentTask(".taskgate/human/notify", "notify", nil, nil)

		out := ws.Run("run", "deploy")
		Expect(out.ExitCode).To(Equal(5))
		Expect(strings.Fields(ws.ReadFile("order.txt"))).To(Equal([]string{"build"}))
	})

	It("errors on a dependency cycle without running anything", func() {
		ws.WriteDependentTask(".taskgate/human/a", "a", []string{"b"}, nil)
		ws.WriteDependentTask(".taskgate/human/b", "b", []string{"a"}, nil)

		out := ws.Run("run", "a")
		Expect(out.ExitCode).NotTo(Equal(0))
		Expect(out.Stderr).To(ContainSubstring("cycle"))
		Expect(ws.ReadFile("order.txt")).To(BeEmpty())
	})
})
