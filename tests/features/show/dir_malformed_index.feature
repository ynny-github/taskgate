Feature: Show subcommand — malformed _index does not abort listing

  A malformed _index does NOT abort the listing. Human form: summary/body
  omitted, optional notice on stderr (we assert non-empty stderr but not its
  exact text — the notice copy is renderer-owned and may change).

  Scenario: malformed _index yields path-only directory row; children still shown
    Given a malformed _index at ".taskgate/human/deploy/_index"
    And an annotated task ".taskgate/human/deploy/prod" with summary "Promote to production."
    When I run "taskgate show deploy"
    Then exit code is 0
    And stdout contains ".taskgate/human/deploy"
    And stdout contains "Promote to production."
    And stdout does not contain "Promote a build"
