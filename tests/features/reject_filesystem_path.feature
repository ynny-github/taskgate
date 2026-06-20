Feature: Show subcommand — filesystem-shaped arguments are rejected (FR-015)

  Filesystem-shaped inputs and the empty string are rejected at argument
  validation, before any name resolution.

  Contract (exit 2):
    - Exit code 2. Stable.
    - Rejected forms: any input whose first byte is `/`; any input that
      contains the segment `.taskgate/`; any input starting with `./` or
      `../`; the empty string.
    - stderr explains: show accepts only run-style names (bare or
      slash-separated), not filesystem paths.

  Background:
    Given an annotated task ".taskgate/human/build" with summary "Build."

  Scenario: FR-015 — taskgate-prefixed path is rejected with exit 2
    When I run "taskgate show .taskgate/human/build"
    Then exit code is 2
    And stdout is empty
    And stderr contains "run-style"

  Scenario: FR-015 — absolute path is rejected with exit 2
    When I run "taskgate show /abs/path"
    Then exit code is 2
    And stdout is empty
    And stderr contains "run-style"

  Scenario: FR-015 — dot-slash path is rejected with exit 2
    When I run "taskgate show ./build"
    Then exit code is 2
    And stdout is empty
    And stderr contains "run-style"

  Scenario: FR-015 — empty string argument is rejected with exit 2
    When I run "taskgate show ''"
    Then exit code is 2
    And stdout is empty
    And stderr contains "run-style"
