Feature: Show subcommand — not-found name exits 3 (FR-014)

  A name that does not resolve to any entry in the merged view.

  Contract (exit 3):
    - Exit code 3. Stable.
    - stderr names the audience scope that was searched (audience bucket
      and shared bucket).

  Scenario: FR-014 — unknown task name exits 3 with scope in stderr
    Given an annotated task ".taskgate/human/build" with summary "Build."
    When I run "taskgate show no-such-task"
    Then exit code is 3
    And stdout is empty
    And stderr contains "not found"
    And stderr contains ".taskgate/human"
    And stderr contains ".taskgate/shared"
