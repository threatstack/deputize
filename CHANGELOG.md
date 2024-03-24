# Changelog
Deputize has had a few different iterations - we started maintaining a changelog at version 4.

## 4.1.1
* Slack: Fixed the bug where Deputize would post the on call message == to the amount of channels in the `Channels` config option.
* Pagerduty: Used the query option in the PD API so that we don't have to pull down the full list of schedules, saves me from dealing with pagination.

## 4.1.0
* Support for scoped OAuth tokens using AWS secret manager, take a look at the PDRotator [README](cmd/pdrotator/README.md).

## 4.0.0
* Refactored the code to work in AWS Lambda
