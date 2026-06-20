Feature: Show subcommand — leading comments before YAML envelope are skipped

  Non-annotation comments before the YAML envelope (shellcheck pragmas,
  copyright headers) are skipped; the parser finds the envelope and
  extracts the summary intact.

  Scenario: shellcheck pragma and copyright header before annotation do not interfere
    Given a task ".taskgate/human/build" with leading comments and summary "Build the project."
    When I run "taskgate show"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/build	Build the project.
      """
