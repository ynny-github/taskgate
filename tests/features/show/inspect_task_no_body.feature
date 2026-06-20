Feature: Show subcommand — task with no body omits body section entirely

  When a task has a summary but no body, the body section is omitted
  entirely (no blank section, no placeholder).

  Scenario: task with summary only shows path and summary, no body section
    Given an annotated task ".taskgate/human/build" with summary "Build the project."
    When I run "taskgate show build"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/build

        Build the project.
      """
