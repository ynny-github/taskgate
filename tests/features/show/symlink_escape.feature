Feature: Show subcommand — symlinks escaping .taskgate/ are listed but not read

  A symlink under .taskgate/ pointing outside .taskgate/ surfaces as an
  entry but its target is not read; the off-workspace summary text never
  appears in output.

  @unix-only
  Scenario: symlink escape appears in listing but target content is not read
    Given a symlink at ".taskgate/human/escapee" pointing to "../../outside"
    And an annotated task "outside" with summary "Secret outside summary."
    When I run "taskgate show"
    Then exit code is 0
    And stdout contains ".taskgate/human/escapee"
    And stdout does not contain "Secret outside summary"
    And stderr contains ".taskgate/human/escapee"
