// deputize - Update an LDAP group with info from the PagerDuty API
// main.go: CLI initialization
//
// Copyright 2017 Threat Stack, Inc. All rights reserved.
// Author: Patrick T. Cable II <pat.cable@threatstack.com>

package main

import "os"

func main() {
	os.Exit(Run(os.Args[1:]))
}
