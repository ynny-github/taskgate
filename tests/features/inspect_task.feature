Feature: Show subcommand — task inspection prints path, summary, and body

  `taskgate show <name>` for a task prints the resolved path, summary, and
  body in that order.

  Scenario: task with summary and body shows all three sections
    Given an annotated task ".taskgate/human/build" with summary "Build the project." and body "Reads VERSION from the environment.\nExits non-zero on build failure."
    When I run "taskgate show build"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/build

        Build the project.

      Reads VERSION from the environment.
      Exits non-zero on build failure.
      """
