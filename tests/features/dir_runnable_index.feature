Feature: Show subcommand — runnable _index supplies annotation and is not double-listed

  A runnable _index (with shebang, executable-shaped) supplies the
  directory's annotation and is NOT double-listed as a child.

  Scenario: runnable _index provides annotation without appearing as a child entry
    Given a runnable _index at ".taskgate/human/deploy/_index" with summary "Promote a build." and body "Idempotent."
    And an annotated task ".taskgate/human/deploy/prod" with summary "Promote to production."
    When I run "taskgate show deploy"
    Then exit code is 0
    And stderr is empty
    And stdout equals:
      """
      .taskgate/human/deploy

        Promote a build.

      Idempotent.

      .taskgate/human/deploy/prod	Promote to production.
      """
