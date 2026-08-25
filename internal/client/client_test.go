package client

import (
	"encoding/json"
	"strings"
	"testing"
)

// Guards the resource-type actions key: the API binds "actions", not
// "default_actions". A regression here silently creates types with no actions.
func TestResourceTypeActionsKey(t *testing.T) {
	b, err := json.Marshal(ResourceType{Name: "document", DefaultActions: []Action{{Name: "read"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"actions":[{"name":"read"`) {
		t.Errorf("actions must serialize under \"actions\": %s", s)
	}
	if strings.Contains(s, "default_actions") {
		t.Errorf("must not emit default_actions: %s", s)
	}
}

// Guards the policy conditions shape: a typed object with *_attrs arrays of
// {key, op, value}, not a flat array. A regression here 400s any conditional
// policy at apply.
func TestPolicyConditionsShape(t *testing.T) {
	p := Policy{
		Name:   "p",
		Effect: "ALLOW",
		Conditions: &PolicyConditions{
			SubjectAttrs: []AttrCheck{{Key: "dept", Op: "eq", Value: json.RawMessage(`"finance"`)}},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"conditions":{`, `"subject_attrs":[`, `"key":"dept"`, `"op":"eq"`, `"value":"finance"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
	if strings.Contains(s, `"conditions":[`) {
		t.Errorf("conditions must be an object, not an array: %s", s)
	}
}

// A policy with no conditions must omit the key entirely (pointer + omitempty),
// so unconditional policies are unaffected.
func TestPolicyConditionsOmittedWhenNil(t *testing.T) {
	b, _ := json.Marshal(Policy{Name: "p", Effect: "ALLOW"})
	if strings.Contains(string(b), "conditions") {
		t.Errorf("nil conditions must be omitted: %s", b)
	}
}
