// taskgate/cmd/snapshot.go
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newSnapshotCmd() *cobra.Command {
	snapshotCmd := &cobra.Command{
		Use:          "snapshot",
		Short:        "Manage approved script snapshots",
		SilenceUsage: true,
	}
	snapshotCmd.AddCommand(newSnapshotInstallCmd())
	snapshotCmd.AddCommand(newSnapshotPathCmd())
	return snapshotCmd
}

func newSnapshotInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "install",
		Short:         "Copy ai/ and shared/ scripts to the snapshot directory",
		Args:          cobra.NoArgs,
		RunE:          snapshotInstall,
		SilenceErrors: true,
	}
}

func snapshotInstall(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("cannot determine working directory: %w", err)
	}

	dirFn := snapshotDirFn
	if snapshotDirOverride != nil {
		dirFn = snapshotDirOverride
	}
	dir, err := dirFn(cwd)
	if err != nil {
		return err
	}

	taskgateDir := filepath.Join(cwd, ".taskgate")
	aiScripts, err := listScripts(filepath.Join(taskgateDir, "ai"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot read .taskgate/ai/: %w", err)
	}
	sharedScripts, err := listScripts(filepath.Join(taskgateDir, "shared"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot read .taskgate/shared/: %w", err)
	}

	aiSet := make(map[string]bool, len(aiScripts))
	for _, name := range aiScripts {
		aiSet[name] = true
	}
	for _, name := range sharedScripts {
		if aiSet[name] {
			return fmt.Errorf("task %q exists in both ai/ and shared/", name)
		}
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create snapshot directory: %w", err)
	}

	for subdir, scripts := range map[string][]string{"ai": aiScripts, "shared": sharedScripts} {
		for _, name := range scripts {
			src := filepath.Join(taskgateDir, subdir, name)
			dst := filepath.Join(dir, name)
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("cannot copy %q: %w", name, err)
			}
		}
	}

	total := len(aiScripts) + len(sharedScripts)
	fmt.Fprintf(cmd.OutOrStdout(), "installed %d script(s) to %s\n", total, dir)
	return nil
}

func listScripts(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

func copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func resolveSnapshotDir(args []string) (string, error) {
	var workdir string
	if len(args) == 1 {
		workdir = args[0]
	} else {
		var err error
		workdir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine working directory: %w", err)
		}
	}

	dirFn := snapshotDirFn
	if snapshotDirOverride != nil {
		dirFn = snapshotDirOverride
	}
	return dirFn(workdir)
}

func newSnapshotPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "path [path]",
		Short:         "Print the snapshot directory for a project",
		Args:          cobra.MaximumNArgs(1),
		RunE:          snapshotPath,
		SilenceErrors: true,
	}
}

func snapshotPath(cmd *cobra.Command, args []string) error {
	dir, err := resolveSnapshotDir(args)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), dir)
	return nil
}
