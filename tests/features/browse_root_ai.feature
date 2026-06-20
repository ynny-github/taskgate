Feature: AI show subcommand — root browse merges shared/ and ai/ (FR-001, ADR-0003)

  `taskgate ai show` with no argument merges shared/ and ai/ (not human/) and
  emits a `listing` envelope on stdout. The contract for listing shape:
    {"kind":"listing","audience":"ai","entries":[{"path":string,"kind":"task"|"directory","summary":string|null}]}
  Compact JSON, terminated by exactly one trailing newline.

  Scenario: FR-001 — ai browse merges shared and ai buckets, excludes human
    Given an annotated task ".taskgate/human/build" with summary "Build."
    And an annotated task ".taskgate/shared/lint" with summary "Lint."
    And an annotated task ".taskgate/ai/analyze" with summary "Analyze."
    When I run "taskgate ai show"
    Then exit code is 0
    And stderr is empty
    And stdout JSON field "kind" equals "listing"
    And stdout JSON field "audience" equals "ai"
    And stdout contains ".taskgate/ai/analyze"
    And stdout contains ".taskgate/shared/lint"
    And stdout does not contain ".taskgate/human/build"
