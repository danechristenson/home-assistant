package main

import "testing"

func TestSplitRole(t *testing.T) {
	cases := []struct {
		in         []string
		connector  bool
		name, role string
	}{
		{[]string{"Steve"}, true, "Steve", ""},
		{[]string{"Cool", "Kid", "99"}, true, "Cool Kid 99", ""},
		{[]string{"Steve", "as", "operator"}, true, "Steve", "operator"},
		{[]string{"Steve", "as", "a", "user"}, true, "Steve", "member"},
		{[]string{"Best", "Player"}, true, "Best Player", ""}, // "add": a gamertag ending in a role word
		{[]string{"Best", "Player", "as", "op"}, true, "Best Player", "operator"},
		{[]string{"Steve", "a", "user"}, false, "Steve", "member"},
		{[]string{"Cool", "Kid", "an", "op"}, false, "Cool Kid", "operator"},
		{[]string{"Steve", "to", "visitor"}, false, "Steve", "visitor"},
		{[]string{"Steve", "member"}, false, "Steve", "member"}, // "make Steve member"
		{[]string{"member"}, false, "member", ""},
		{[]string{"as", "operator"}, false, "as operator", ""},
	}
	for _, c := range cases {
		if name, role := splitRole(c.in, c.connector); name != c.name || role != c.role {
			t.Errorf("%q (connector %v): got (%q, %q), want (%q, %q)", c.in, c.connector, name, role, c.name, c.role)
		}
	}
}
