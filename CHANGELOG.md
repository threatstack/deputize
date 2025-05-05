# Changelog
Deputize has had a few different iterations - we started maintaining a changelog at version 4.

## 4.1.3
* Deps: Bumped all first-line deps to latest, swapped `github.com/xanzy/go-gitlab` for the official `gitlab.com/gitlab-org/api/client-go`
* LDAP: Fixed a bug where if there were 0 members in the LDAP group the code to update the group never ran.

## 4.1.2
* Deps: Bumped all first-line deps to latest; 

## 4.1.1
* Deps: Bumped all first-line deps to latest; bumped indirect protobuf to address CVE-2024-24786.
* Slack: Fixed the bug where Deputize would post the on call message == to the amount of channels in the `Channels` config option.
* Pagerduty: Used the query option in the PD API so that we don't have to pull down the full list of schedules, saves me from dealing with pagination.

## 4.1.0
* Support for scoped OAuth tokens using AWS secret manager, take a look at the PDRotator [README](cmd/pdrotator/README.md).

## 4.0.0
* Refactored the code to work in AWS Lambda
