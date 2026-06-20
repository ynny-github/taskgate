Feature: AI show subcommand — bare task has null summary (ADR-0003)

  A task with no summary annotation: `summary` is the explicit JSON `null`.

  Contract (null vs omit rule, applies to task / directory / child records):
    - `summary` is ALWAYS present on records describing an entry; `null`
      when no summary was extracted. Never omitted.
    - `body` is OMITTED when there is no body. Never `null`.

  Scenario: bare task emits task envelope with summary set to null
    Given a bare task ".taskgate/ai/bare"
    When I run "taskgate ai show bare"
    Then exit code is 0
    And stderr is empty
    And stdout JSON field "kind" equals "task"
    And stdout JSON field "path" equals ".taskgate/ai/bare"
    And stdout has JSON field "summary" set to null
