// helpers.go - small useful functions for deputize.
// Copyright 2024 F5 Inc.
// Licensed under the BSD 3-clause license; see LICENSE.md for more information.

package main

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
		if encountered[elements[v]] {
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
