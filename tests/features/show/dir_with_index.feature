Feature: Show subcommand — directory with _index shows path, summary, body, then children

  `taskgate show <dir-name>` with an _index prints path, summary, body, then
  one row per immediate child; _index itself is not listed.

  Scenario: directory with _index shows header and child rows
    Given an _index at ".taskgate/human/deploy/_index" with summary "Promote a build to an environment." and body "Idempotent across reruns."
    And an annotated task ".taskgate/human/deploy/canary" with summary "Promote to canary."
    And an annotated task ".taskgate/human/deploy/prod" with summary "Promote to production."
    When I run "taskgate show deploy"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/deploy

        Promote a build to an environment.

      Idempotent across reruns.

      .taskgate/human/deploy/canary	Promote to canary.
      .taskgate/human/deploy/prod	Promote to production.
      """
