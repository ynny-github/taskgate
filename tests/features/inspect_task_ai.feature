Feature: Show subcommand — AI task envelope shape (FR-006)

  taskgate ai show <task-name> MUST emit a single `task` envelope on stdout
  carrying path, summary, body, and audience. The schema details live in
  ADR-0003 (docs/adr/0003-ai-output-wire-format.md).

  Scenario: task envelope has the expected fields
    Given an annotated task ".taskgate/ai/analyze" with summary "Analyze the codebase." and body "Reads CONFIG from environment."
    When I run "taskgate ai show analyze"
    Then exit code is 0
    And stderr is empty
    And stdout JSON field "kind" equals "task"
    And stdout JSON field "path" equals ".taskgate/ai/analyze"
    And stdout JSON field "summary" equals "Analyze the codebase."
    And stdout JSON field "body" equals "Reads CONFIG from environment."
    And stdout JSON field "audience" equals "ai"
