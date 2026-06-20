"""Shared fixtures and step definitions for the taskgate E2E suite."""
from __future__ import annotations

import json
import os
import re
import shlex
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

import pytest
from pytest_bdd import given, parsers, then, when


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@dataclass
class RunResult:
    stdout: str
    stderr: str
    returncode: int


class Workspace:
    """A scratch directory that holds a .taskgate/ tree for one scenario."""

    def __init__(self, root: Path) -> None:
        self.root = root

    def write_file(self, relpath: str, content: str, *, executable: bool = False) -> None:
        path = self.root / relpath
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)
        if executable:
            path.chmod(0o755)

    def write_annotated_task(
        self,
        relpath: str,
        *,
        summary: str | None = None,
        body: str | None = None,
    ) -> None:
        """Write a sh task with the same YAML front-matter the .txtar files use."""
        lines = ["#!/bin/sh", "# ---"]
        if summary is not None:
            lines.append(f"# summary: {summary}")
        if body is not None:
            lines.append("# body: |")
            for body_line in body.splitlines():
                lines.append(f"#   {body_line}")
        lines.append("# ---")
        lines.append("echo hi")
        lines.append("")
        self.write_file(relpath, "\n".join(lines), executable=True)

    def write_bare_task(self, relpath: str) -> None:
        """Write a shebang-only sh script with no annotation."""
        self.write_file(relpath, "#!/bin/sh\necho hi\n", executable=True)

    def write_index(
        self,
        relpath: str,
        *,
        summary: str | None = None,
        body: str | None = None,
        prefix: str = "# ",
    ) -> None:
        """Write a non-executable _index file with YAML annotation envelope."""
        lines = [f"{prefix}---"]
        if summary is not None:
            lines.append(f"{prefix}summary: {summary}")
        if body is not None:
            lines.append(f"{prefix}body: |")
            for body_line in body.splitlines():
                lines.append(f"{prefix}  {body_line}")
        lines.append(f"{prefix}---")
        lines.append("")
        self.write_file(relpath, "\n".join(lines))

    def write_runnable_index(
        self,
        relpath: str,
        *,
        summary: str | None = None,
        body: str | None = None,
    ) -> None:
        """Write an executable _index with shebang + annotation."""
        lines = ["#!/bin/sh", "# ---"]
        if summary is not None:
            lines.append(f"# summary: {summary}")
        if body is not None:
            lines.append("# body: |")
            for body_line in body.splitlines():
                lines.append(f"#   {body_line}")
        lines.append("# ---")
        lines.append('echo "_index can also run"')
        lines.append("")
        self.write_file(relpath, "\n".join(lines), executable=True)


@dataclass
class Taskgate:
    binary: Path
    workspace: Workspace

    def run(self, *args: str) -> RunResult:
        proc = subprocess.run(
            [str(self.binary), *args],
            cwd=str(self.workspace.root),
            capture_output=True,
            text=True,
            timeout=10,
        )
        return RunResult(stdout=proc.stdout, stderr=proc.stderr, returncode=proc.returncode)


@pytest.fixture(scope="session")
def taskgate_binary(tmp_path_factory) -> Path:
    """Build the taskgate binary once per test session."""
    out_dir = tmp_path_factory.mktemp("bin")
    out_path = out_dir / "taskgate"
    repo_root = Path(__file__).resolve().parent.parent
    subprocess.run(
        ["go", "build", "-o", str(out_path), "./taskgate"],
        cwd=str(repo_root),
        check=True,
    )
    return out_path


@pytest.fixture
def workspace(tmp_path) -> Workspace:
    return Workspace(tmp_path)


@pytest.fixture
def taskgate(taskgate_binary: Path, workspace: Workspace) -> Taskgate:
    return Taskgate(binary=taskgate_binary, workspace=workspace)


# ---------------------------------------------------------------------------
# Tag hooks
# ---------------------------------------------------------------------------


def pytest_bdd_apply_tag(tag, function):
    if tag == "unix-only":
        marker = pytest.mark.skipif(sys.platform == "win32", reason="POSIX-only")
        marker(function)
        return True
    return None


# ---------------------------------------------------------------------------
# Step definitions
# ---------------------------------------------------------------------------


# --- Given -----------------------------------------------------------------


@given(parsers.parse('an annotated task "{path}" with summary "{summary}"'))
def given_task_summary(workspace: Workspace, path: str, summary: str) -> None:
    workspace.write_annotated_task(path, summary=summary)


@given(parsers.parse(
    'an annotated task "{path}" with summary "{summary}" and body "{body}"'
))
def given_task_summary_body(
    workspace: Workspace, path: str, summary: str, body: str
) -> None:
    # Gherkin step strings don't expand \n — convert literal \n to newline.
    workspace.write_annotated_task(path, summary=summary, body=body.replace("\\n", "\n"))


@given(parsers.parse('a bare task "{path}"'))
def given_bare_task(workspace: Workspace, path: str) -> None:
    workspace.write_bare_task(path)


@given(parsers.parse('an _index at "{path}" with summary "{summary}"'))
def given_index_summary(workspace: Workspace, path: str, summary: str) -> None:
    workspace.write_index(path, summary=summary)


@given(parsers.parse(
    'an _index at "{path}" with summary "{summary}" and body "{body}"'
))
def given_index_summary_body(
    workspace: Workspace, path: str, summary: str, body: str
) -> None:
    workspace.write_index(path, summary=summary, body=body)


@given(parsers.parse('a malformed _index at "{path}"'))
def given_malformed_index(workspace: Workspace, path: str) -> None:
    workspace.write_file(path, "# ---\n# summary: [unclosed_array\n# ---\n")


@given(parsers.parse(
    'a runnable _index at "{path}" with summary "{summary}" and body "{body}"'
))
def given_runnable_index(
    workspace: Workspace, path: str, summary: str, body: str
) -> None:
    workspace.write_runnable_index(path, summary=summary, body=body)


@given(parsers.parse(
    'a task "{path}" with leading comments and summary "{summary}"'
))
def given_task_leading_comments(
    workspace: Workspace, path: str, summary: str
) -> None:
    content = (
        "#!/bin/sh\n"
        "# shellcheck disable=SC2086\n"
        "# Copyright (c) 2026 Example Corp.\n"
        "# ---\n"
        f"# summary: {summary}\n"
        "# ---\n"
        "echo build\n"
    )
    workspace.write_file(path, content, executable=True)


@given(parsers.parse(
    'an unreadable task at "{path}" with summary "{summary}"'
))
def given_unreadable_task(workspace: Workspace, path: str, summary: str) -> None:
    workspace.write_annotated_task(path, summary=summary)
    full_path = workspace.root / path
    full_path.chmod(0o000)


@given(parsers.parse('a symlink at "{linkpath}" pointing to "{target}"'))
def given_symlink(workspace: Workspace, linkpath: str, target: str) -> None:
    link = workspace.root / linkpath
    link.parent.mkdir(parents=True, exist_ok=True)
    os.symlink(target, link)


@given(parsers.parse('{count:d} bare tasks under "{dirpath}"'))
def given_many_bare_tasks(workspace: Workspace, count: int, dirpath: str) -> None:
    for i in range(count):
        name = f"child{i:02d}"
        workspace.write_annotated_task(
            f"{dirpath}/{name}",
            summary=f"{i:02d}.",
        )


# --- When ------------------------------------------------------------------


@when(parsers.parse('I run "{cmd}"'), target_fixture="result")
def when_run(taskgate: Taskgate, cmd: str) -> RunResult:
    args = shlex.split(cmd)
    if args and args[0] == "taskgate":
        args = args[1:]
    return taskgate.run(*args)


# --- Then ------------------------------------------------------------------


@then(parsers.parse("exit code is {code:d}"))
def then_exit_code(result: RunResult, code: int) -> None:
    assert result.returncode == code, (
        f"exit={result.returncode}, stdout={result.stdout!r}, "
        f"stderr={result.stderr!r}"
    )


@then("stdout is empty")
def then_stdout_empty(result: RunResult) -> None:
    assert result.stdout == "", f"stdout was: {result.stdout!r}"


@then("stderr is empty")
def then_stderr_empty(result: RunResult) -> None:
    assert result.stderr == "", f"stderr was: {result.stderr!r}"


@then(parsers.parse('stderr contains "{text}"'))
def then_stderr_contains(result: RunResult, text: str) -> None:
    assert text in result.stderr, f"stderr was: {result.stderr!r}"


@then(parsers.parse('stdout contains "{text}"'))
def then_stdout_contains(result: RunResult, text: str) -> None:
    assert text in result.stdout, f"stdout was: {result.stdout!r}"


@then(parsers.parse('stdout does not contain "{text}"'))
def then_stdout_not_contains(result: RunResult, text: str) -> None:
    assert text not in result.stdout, f"stdout unexpectedly contained {text!r}: {result.stdout!r}"


@then(parsers.parse('stderr does not contain "{text}"'))
def then_stderr_not_contains(result: RunResult, text: str) -> None:
    assert text not in result.stderr, f"stderr unexpectedly contained {text!r}: {result.stderr!r}"


@then(parsers.parse('stdout matches regex "{pattern}"'))
def then_stdout_matches_regex(result: RunResult, pattern: str) -> None:
    assert re.search(pattern, result.stdout), (
        f"stdout did not match {pattern!r}: {result.stdout!r}"
    )


@then(parsers.parse('stderr matches regex "{pattern}"'))
def then_stderr_matches_regex(result: RunResult, pattern: str) -> None:
    assert re.search(pattern, result.stderr), (
        f"stderr did not match {pattern!r}: {result.stderr!r}"
    )


@then(parsers.parse('stdout JSON field "{field}" equals "{value}"'))
def then_stdout_json_field_equals(result: RunResult, field: str, value: str) -> None:
    envelope = json.loads(result.stdout)
    actual = envelope.get(field)
    # YAML literal-block (`body: |`) adds a trailing newline; normalize for comparison.
    if isinstance(actual, str):
        actual = actual.rstrip("\n")
    assert actual == value, (
        f'field "{field}": got {actual!r}, want {value!r} '
        f"(full envelope: {envelope!r})"
    )


@then(parsers.parse('stdout has JSON field "{field}" set to null'))
def then_stdout_json_field_null(result: RunResult, field: str) -> None:
    envelope = json.loads(result.stdout)
    assert field in envelope, f'field "{field}" absent from envelope: {envelope!r}'
    assert envelope[field] is None, (
        f'field "{field}": got {envelope[field]!r}, want null '
        f"(full envelope: {envelope!r})"
    )


@then(parsers.parse("stdout equals:"))
def then_stdout_equals(result: RunResult, docstring: str) -> None:
    expected = docstring + "\n"
    assert result.stdout == expected, (
        f"stdout mismatch:\n  got:  {result.stdout!r}\n  want: {expected!r}"
    )


@then(parsers.parse("stderr equals:"))
def then_stderr_equals(result: RunResult, docstring: str) -> None:
    expected = docstring + "\n"
    assert result.stderr == expected, (
        f"stderr mismatch:\n  got:  {result.stderr!r}\n  want: {expected!r}"
    )
