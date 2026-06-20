Feature: Show subcommand — directory listing is not recursive

  Directory target lists nested sub-dirs as single rows (no recursion).
  Drilling deeper requires `taskgate show deploy/prod`.

  Scenario: nested sub-directory appears as a single row, not expanded
    Given an _index at ".taskgate/human/deploy/_index" with summary "Promote a build."
    And an _index at ".taskgate/human/deploy/prod/_index" with summary "Prod target."
    And an annotated task ".taskgate/human/deploy/prod/run" with summary "Run a prod deploy."
    When I run "taskgate show deploy"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/deploy

        Promote a build.

      .taskgate/human/deploy/prod/	Prod target.
      """
