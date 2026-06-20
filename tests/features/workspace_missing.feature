Feature: Show subcommand — missing workspace exits 5 (FR-016)

  Running show outside a workspace (no .taskgate/ in cwd or ancestors).

  Contract (exit 5):
    - Exit code 5. Stable.
    - stderr: `.taskgate/ not found`.

  Scenario: FR-016 — no .taskgate/ directory exits 5
    When I run "taskgate show"
    Then exit code is 5
    And stdout is empty
    And stderr contains ".taskgate/"
