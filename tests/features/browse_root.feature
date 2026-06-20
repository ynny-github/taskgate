Feature: Show subcommand — root browse merges human/ and shared/ (FR-001)

  Annotated tasks from human/ and shared/ surface in the merged view with
  their summaries; bucket directories never appear as rows.

  Scenario: FR-001 — annotated tasks from both buckets appear in merged view
    Given an annotated task ".taskgate/human/build" with summary "Build the project for the current platform."
    And an annotated task ".taskgate/shared/lint" with summary "Lint the codebase with project rules."
    When I run "taskgate show"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/build	Build the project for the current platform.
      .taskgate/shared/lint	Lint the codebase with project rules.
      """
