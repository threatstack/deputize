// deputize - Update an LDAP group with info from the PagerDuty API
// commands.go: CLI initialization
//
// Copyright 2017 Threat Stack, Inc. All rights reserved.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package main

import (
	"github.com/mitchellh/cli"
	"github.com/threatstack/deputize/command"
)

// Commands is the factory generator for various command options in Deputize
func Commands(meta *command.Meta) map[string]cli.CommandFactory {
	return map[string]cli.CommandFactory{
		"oncall": func() (cli.Command, error) {
			return &command.OncallCommand{
				Meta: *meta,
			}, nil
		},

		"version": func() (cli.Command, error) {
			return &command.VersionCommand{
				Meta:     *meta,
				Version:  Version,
				Revision: GitCommit,
				Name:     Name,
			}, nil
		},
	}
}
