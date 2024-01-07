// mod_gitlab.go - Gitlab sink code
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

package main

import (
	"fmt"
	"log"

	"github.com/xanzy/go-gitlab"
)

func updateGitlab(cfg deputizeGitlabConfig, pdOnCallEmails []string, gitlabAuthToken string) error {
	log.Printf("Beginning Gitlab Update.\n")
	var newOnCallApproverGitlabUserIDs []int

	client, err := gitlab.NewClient(gitlabAuthToken, gitlab.WithBaseURL(cfg.Server+"api/v4"))
	if err != nil {
		return fmt.Errorf("could not initialize client: %s", err)
	}
	// Lets get user ids for On Call people
	for _, email := range pdOnCallEmails {
		userOptions := &gitlab.ListUsersOptions{Search: gitlab.Ptr(email)}
		users, _, err := client.Users.ListUsers(userOptions)
		if err != nil {
			log.Printf("Warning: Got %s back from Gitlab API\n", err)
		}
		if len(users) == 1 {
			// We expect only one user returned based on an email. We error out otherwise
			log.Printf("User found! username is %s for email %s\n", users[0].Username, email)
			newOnCallApproverGitlabUserIDs = append(newOnCallApproverGitlabUserIDs, users[0].ID)
		} else if len(users) == 0 {
			log.Printf("No user found for email %s\n", email)
		} else {
			// Lets output some helpful information if we don't get 1 user
			for _, user := range users {
				log.Printf("Found the following users associated with \"%s\": %s\n", email, user.Username)
			}
			return fmt.Errorf("found more than one user with an email of %s: %d users found", email, len(users))
		}
	}

	if len(newOnCallApproverGitlabUserIDs) == 0 {
		// If no users are in the new approver list, leave the group alone
		log.Printf("No new Approvers, not updating Gitlab group: %s", cfg.Group)
	} else {
		// Add OnCall approvers to approver group
		log.Printf("Updating Gitlab group: %s", cfg.Group)

		// Remove existing members of the group, if they exist
		log.Printf("Removing old approvers from Gitlab group: %s", cfg.Group)

		// Get the existing members of the group
		approverGroupMembers, _, err := client.Groups.ListGroupMembers(cfg.Group, &gitlab.ListGroupMembersOptions{})
		if err != nil {
			return fmt.Errorf("gitlab could not get group members: %s", err.Error())
		}
		if len(approverGroupMembers) > 0 {
			// Remove existing members
			for _, member := range approverGroupMembers {
				// Don't remove group owner/maintainers
				if member.AccessLevel < 40 {
					log.Printf("Removing user %s", member.Username)
					_, err := client.GroupMembers.RemoveGroupMember(cfg.Group, member.ID, &gitlab.RemoveGroupMemberOptions{})
					if err != nil {
						return fmt.Errorf("gitlab could not remove group member: %s", err)
					}
				}
			}
		}

		// Add new members to the group
		log.Printf("Adding new approvers to Gitlab group: %s", cfg.Group)
		for _, newApproverUserID := range newOnCallApproverGitlabUserIDs {
			log.Printf("Adding user id %d", newApproverUserID)
			addGroupMemberOpts := &gitlab.AddGroupMemberOptions{
				UserID:      gitlab.Ptr(newApproverUserID),
				AccessLevel: gitlab.Ptr(gitlab.DeveloperPermissions),
			}
			_, _, err := client.GroupMembers.AddGroupMember(cfg.Group, addGroupMemberOpts)
			if err != nil {
				return fmt.Errorf("gitlab could not add group member: %s", err)
			}
		}
	}
	log.Printf("Gitlab Update Complete.\n")
	return nil
}
