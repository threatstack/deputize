// deputize - Update an LDAP group with info from the PagerDuty API
// helpers.go: helper functions used in the oncall command
//
// Copyright 2017-2022 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE for more information.

package oncall

import (
	"log"

	"gopkg.in/ldap.v2"
)

// This is only good for things you know will return only one result. Be warned.
func search(l *ldap.Conn, basedn string, search string, attributes []string) (s *ldap.SearchResult) {
	searchRequest := ldap.NewSearchRequest(
		basedn,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		search,
		attributes,
		nil,
	)
	sr, err := l.Search(searchRequest)
	if err != nil {
		log.Fatal(err)
	}

	if len(sr.Entries) != 1 {
		log.Fatal("User does not exist or too many entries returned")
	}

	return sr
}

func contains(str []string, search string) bool {
	for _, a := range str {
		if a == search {
			return true
		}
	}
	return false
}

func removeDuplicates(elements []string) []string {
	// Use map to record duplicates as we find them.
	encountered := map[string]bool{}
	result := []string{}

	for v := range elements {
		if encountered[elements[v]] == true {
			// Do not add duplicate.
		} else {
			// Record this element as an encountered element.
			encountered[elements[v]] = true
			// Append to result slice.
			result = append(result, elements[v])
		}
	}
	// Return the new slice.
	return result
}
