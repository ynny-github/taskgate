Feature: Show subcommand — collisions are a hard error (FR-013)

  When the same logical name exists in both the audience bucket and the
  shared bucket, taskgate show MUST refuse to render the conflicting region
  with exit 4 and stderr listing every conflicting real path.

  Scenario: no-argument browse
    Given an annotated task ".taskgate/human/build" with summary "human variant"
    And an annotated task ".taskgate/shared/build" with summary "shared variant"
    When I run "taskgate show"
    Then exit code is 4
    And stdout is empty
    And stderr contains ".taskgate/human/build"
    And stderr contains ".taskgate/shared/build"

  Scenario: explicit name reference
    Given an annotated task ".taskgate/human/build" with summary "human variant"
    And an annotated task ".taskgate/shared/build" with summary "shared variant"
    When I run "taskgate show build"
    Then exit code is 4
    And stdout is empty
    And stderr contains ".taskgate/human/build"
    And stderr contains ".taskgate/shared/build"
