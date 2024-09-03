package client

import (
	"reflect"
	"slices"
)

// isEquivalent return whether two slices are equivalent using reflect.DeepEqual
//
// Equivalent in this context means they contain the same elements, not necessarily in the same order.
// In essence, this is set equality using DeepEqual.
func isEquivalent[T any](s1, s2 []T) bool {
	if len(s1) != len(s2) {
		return false
	}
	for _, v := range s1 {
		if !slices.ContainsFunc(s2, func(e T) bool { return reflect.DeepEqual(v, e) }) {
			return false
		}
	}
	return true
}
