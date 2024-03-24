// mod_pagerduty.go - PagerDuty source code
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/PagerDuty/go-pagerduty"
)

func getPagerdutyInfo(ctx context.Context, withOAuth bool, authToken string, schedules []string) ([]string, error) {
	var newOnCallEmails []string
	var pdClient *pagerduty.Client

	if withOAuth {
		pdClient = pagerduty.NewOAuthClient(authToken)
	} else {
		pdClient = pagerduty.NewClient(authToken)
	}

	var allRawSchedulesPD [][]pagerduty.Schedule

	for _, sch := range schedules {
		lsSchedulesOpts := pagerduty.ListSchedulesOptions{
			Total: true,
			Query: sch,
		}
		reqSchedule, err := pdClient.ListSchedulesWithContext(ctx, lsSchedulesOpts)
		if err != nil {
			return []string{}, err
		}

		allRawSchedulesPD = append(allRawSchedulesPD, reqSchedule.Schedules)
	}

	allSchedulesPD := slices.Concat(allRawSchedulesPD...)

	for _, p := range allSchedulesPD {
		if contains(schedules, p.Name) {
			var onCallOpts pagerduty.ListOnCallUsersOptions
			var currentTime = time.Now()
			onCallOpts.Since = currentTime.Format("2006-01-02T15:04:05Z07:00")
			hours, _ := time.ParseDuration("1s")
			onCallOpts.Until = currentTime.Add(hours).Format("2006-01-02T15:04:05Z07:00")
			if oncall, err := pdClient.ListOnCallUsersWithContext(ctx, p.APIObject.ID, onCallOpts); err != nil {
				return []string{}, fmt.Errorf("unable to ListOnCallUsers: %s", err)
			} else {
				for _, person := range oncall {
					newOnCallEmails = append(newOnCallEmails, person.Email)
				}
			}
		}
	}

	return newOnCallEmails, nil
}
