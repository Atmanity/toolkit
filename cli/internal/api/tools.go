package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// App tools: functions a full conversation's avatar may call. A tool is the
// OpenAI function-tool shape plus an optional `x-atmee` object saying whether
// the avatar waits for the result (`mode` `notify` or `await`, and
// `timeoutSeconds`). Every call reaches the visitor's LiveKit client as the
// RPC `atmee.tool`.

// Tools is a session's tool list as the caller wrote it: each element is sent
// unchanged, and the API judges it (400 invalid_tools names the rule).
type Tools []json.RawMessage

// ParseTools checks only the shape a session start needs: a JSON array of
// objects, each with a string name. Limits, names and values are the API's
// to check.
func ParseTools(raw []byte) (Tools, error) {
	var list []json.RawMessage
	if err := json.Unmarshal(bytes.TrimSpace(raw), &list); err != nil || list == nil {
		if err == nil {
			err = errors.New("it is null")
		}
		return nil, fmt.Errorf("not a JSON array of tools: %v", err)
	}
	if len(list) == 0 {
		return nil, errors.New("the array is empty")
	}
	for i, el := range list {
		var t struct {
			Name *string `json:"name"`
		}
		if bytes.TrimSpace(el)[0] != '{' || json.Unmarshal(el, &t) != nil || t.Name == nil {
			return nil, fmt.Errorf("tool %d is not an object with a string name", i+1)
		}
	}
	return list, nil
}
