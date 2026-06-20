Feature: Show subcommand — whitespace-only summary treated as empty

  A summary annotation that is whitespace-only is treated as empty (same
  handling as no summary). The task still appears at root, path-only.

  Scenario: whitespace-only summary renders as path-only entry
    Given an annotated task ".taskgate/human/build" with summary "   "
    When I run "taskgate show"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/build
      """
