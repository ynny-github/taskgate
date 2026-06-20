Feature: Show subcommand — unreadable file does not abort listing

  A task file with read permission denied does not abort the listing.
  Remaining entries are still emitted.

  @unix-only
  Scenario: unreadable task is skipped; other entries still appear
    Given an unreadable task at ".taskgate/human/locked" with summary "Locked."
    And an annotated task ".taskgate/shared/lint" with summary "Lint."
    When I run "taskgate show"
    Then exit code is 0
    And stdout contains ".taskgate/shared/lint"
    And stdout contains "Lint."
