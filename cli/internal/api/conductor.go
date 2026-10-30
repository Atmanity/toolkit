package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// MaxConductorBodyBytes is the conductor route's request body cap.
const MaxConductorBodyBytes = 512 * 1024

// Conductor is a v2 avatar's graph conductor as the API returns it. The
// graph stays raw JSON: it is printed and saved exactly as the API sent it
// (the generated types would drop null fields and anything they don't name).
type Conductor struct {
	AvatarID    string          `json:"avatarId"`
	Graph       json.RawMessage `json:"graph"`
	SpeakAfterS *float64        `json:"speakAfterS,omitempty"`
	Nodes       int             `json:"nodes"`
	Edges       int             `json:"edges"`
}

// ConductorBody builds a PUT /conductor body from a file's JSON: a full
// request body (an object with a top-level "graph") or a bare graph (any
// other object). speakAfterS, when set, replaces the body's own. The body's
// other values are sent as written; the API checks them.
func ConductorBody(raw []byte, speakAfterS *float64) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return nil, fmt.Errorf("the graph file must hold a JSON object: a graph ({\"start\", \"nodes\"}) or a request body ({\"graph\", \"speakAfterS\"})")
	}
	body := top
	if _, full := top["graph"]; !full {
		body = map[string]json.RawMessage{"graph": json.RawMessage(bytes.TrimSpace(raw))}
	}
	if speakAfterS != nil {
		b, err := json.Marshal(*speakAfterS)
		if err != nil {
			return nil, fmt.Errorf("speakAfterS: %v", err)
		}
		body["speakAfterS"] = b
	}
	out, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	if len(out) > MaxConductorBodyBytes {
		return nil, fmt.Errorf("the conductor request is %d bytes; the API takes at most %d (512 KiB)", len(out), MaxConductorBodyBytes)
	}
	return out, nil
}

// SetConductor replaces the avatar's conductor with body (see ConductorBody).
func (c *Client) SetConductor(ctx context.Context, avatarID string, body []byte) (*Conductor, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.SetAvatarConductorWithBodyWithResponse(ctx, id, "application/json", bytes.NewReader(body))
	if r == nil {
		return nil, err
	}
	return decode[Conductor](r.HTTPResponse, r.Body, err)
}

// GetConductor reads the conductor as saved (404 conductor_not_set when none is).
func (c *Client) GetConductor(ctx context.Context, avatarID string) (*Conductor, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarConductorWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Conductor](r.HTTPResponse, r.Body, err)
}

// DeleteConductor removes the conductor; removing one that is not set is no error.
func (c *Client) DeleteConductor(ctx context.Context, avatarID string) error {
	id, err := ParseID(avatarID)
	if err != nil {
		return err
	}
	r, err := c.gen.DeleteAvatarConductorWithResponse(ctx, id)
	if r == nil {
		return err
	}
	_, err = decode[struct{}](r.HTTPResponse, r.Body, err)
	return err
}
