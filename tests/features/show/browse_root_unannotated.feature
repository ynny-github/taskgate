Feature: Show subcommand — unannotated tasks still appear in root browse

  A task without an annotation block still appears at root with its path
  only; no error is raised.

  Scenario: unannotated task appears with path only, no error
    Given a bare task ".taskgate/shared/bare"
    When I run "taskgate show"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/shared/bare
      """
