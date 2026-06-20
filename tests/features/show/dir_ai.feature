Feature: AI show subcommand — directory envelope shape (FR-007, ADR-0003)

  `taskgate ai show <dir-name>` emits a single `directory` envelope.
  Contract (directory shape):
    {"kind":"directory","path":string,"summary":string|null,"body":string,"audience":"human"|"ai",
     "entries":[{"path":string,"kind":"task"|"directory","summary":string|null}]}
  Compact JSON, terminated by exactly one trailing newline. Sort: directories
  first, then tasks; basename case-sensitive.

  Scenario: FR-007 — directory envelope contains summary and child entries
    Given an _index at ".taskgate/shared/deploy/_index" with summary "Promote a build."
    And an annotated task ".taskgate/shared/deploy/canary" with summary "Promote to canary."
    And an annotated task ".taskgate/shared/deploy/prod" with summary "Promote to production."
    When I run "taskgate ai show deploy"
    Then exit code is 0
    And stderr is empty
    And stdout JSON field "kind" equals "directory"
    And stdout JSON field "path" equals ".taskgate/shared/deploy"
    And stdout JSON field "summary" equals "Promote a build."
    And stdout contains ".taskgate/shared/deploy/canary"
    And stdout contains ".taskgate/shared/deploy/prod"
