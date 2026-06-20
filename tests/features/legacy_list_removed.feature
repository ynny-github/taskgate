Feature: Legacy list subcommand removed — no alias or deprecation shim

  `taskgate list` and `taskgate ai list` were removed in favour of
  `taskgate show` / `taskgate ai show`. Invoking the old names must surface
  cobra's "unknown command" error and exit non-zero.

  Contract: no alias, no deprecation shim. Callers must migrate.

  Scenario: taskgate list is unknown
    When I run "taskgate list"
    Then exit code is 1
    And stderr contains "unknown command"

  Scenario: taskgate ai list is unknown
    When I run "taskgate ai list"
    Then exit code is 1
    And stderr contains "unknown command"
