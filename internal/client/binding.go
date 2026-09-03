package client

import (
	"context"
	"net/url"
)

func bindingsPath(vhostID string) string {
	return "/api/broker/vhost/" + url.PathEscape(vhostID) + "/bindings"
}

// CreateBinding calls POST /api/broker/vhost/{vhostID}/bindings.
func (c *Client) CreateBinding(ctx context.Context, vhostID string, req BindRequest) (*Binding, error) {
	var out Binding
	if err := c.do(ctx, "POST", bindingsPath(vhostID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBinding calls GET /api/broker/vhost/{vhostID}/bindings/{bindingID}.
func (c *Client) GetBinding(ctx context.Context, vhostID, bindingID string) (*Binding, error) {
	var out Binding
	path := bindingsPath(vhostID) + "/" + url.PathEscape(bindingID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteBinding calls DELETE /api/broker/vhost/{vhostID}/bindings/{bindingID}.
func (c *Client) DeleteBinding(ctx context.Context, vhostID, bindingID string) error {
	return c.do(ctx, "DELETE", bindingsPath(vhostID)+"/"+url.PathEscape(bindingID), nil, nil)
}
