Feature: Show subcommand — no truncation with many children

  A directory with many children produces a complete listing with no
  truncation. We seed 50 children (enough to surface any per-child cost or
  accidental truncation) and assert the first and the last basename-lex
  entries are both present.

  Scenario: 50 children all appear in listing without truncation
    Given 50 bare tasks under ".taskgate/human/many"
    When I run "taskgate show many"
    Then exit code is 0
    And stderr is empty
    And stdout contains ".taskgate/human/many/child00"
    And stdout contains ".taskgate/human/many/child49"
