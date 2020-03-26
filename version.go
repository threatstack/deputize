// deputize - Update an LDAP group with info from the PagerDuty API
// version.go: CLI initialization
//
// Copyright 2017 Threat Stack, Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package main

// Name of the application
const Name string = "deputize"

// Version is the current version
const Version string = "2.0.0"

// GitCommit describes latest commit hash.
// This value is extracted by git command when building.
// To set this from outside, use go build -ldflags "-X main.GitCommit \"$(COMMIT)\""
var GitCommit string
