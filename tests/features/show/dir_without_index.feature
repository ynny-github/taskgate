Feature: Show subcommand — directory without _index shows path only then children

  Directory with no _index prints path only (no summary, no body), then the children rows.

  Scenario: directory without _index shows path header and child rows
    Given an annotated task ".taskgate/human/deploy/canary" with summary "Promote to canary."
    And an annotated task ".taskgate/human/deploy/prod" with summary "Promote to production."
    When I run "taskgate show deploy"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/deploy

      .taskgate/human/deploy/canary	Promote to canary.
      .taskgate/human/deploy/prod	Promote to production.
      """
