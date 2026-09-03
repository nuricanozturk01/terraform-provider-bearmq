package client

import (
	"context"
	"fmt"
	"net/url"
)

// CreateVHost calls POST /api/broker/vhost. name may be empty, in which case the
// server generates one.
func (c *Client) CreateVHost(ctx context.Context, name string) (*VHost, error) {
	var body *CreateVHostRequest
	if name != "" {
		body = &CreateVHostRequest{Name: name}
	}
	var out VHost
	if err := c.do(ctx, "POST", "/api/broker/vhost", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVHost calls GET /api/broker/vhost/{id}. Returns ErrNotFound if absent.
func (c *Client) GetVHost(ctx context.Context, id string) (*VHost, error) {
	var out VHost
	if err := c.do(ctx, "GET", "/api/broker/vhost/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetVHostStatus calls PATCH /api/broker/vhost/{id}/status (ACTIVE | PAUSED).
func (c *Client) SetVHostStatus(ctx context.Context, id, status string) (*VHost, error) {
	var out VHost
	err := c.do(ctx, "PATCH", "/api/broker/vhost/"+url.PathEscape(id)+"/status",
		UpdateVHostStatusRequest{Status: status}, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteVHost calls DELETE /api/broker/vhost/{id}.
func (c *Client) DeleteVHost(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", "/api/broker/vhost/"+url.PathEscape(id), nil, nil)
}

// FindVHostByName pages through GET /api/broker/vhost looking for an exact name
// match. Returns ErrNotFound when no vhost has that name.
func (c *Client) FindVHostByName(ctx context.Context, name string) (*VHost, error) {
	match, err := paginate(ctx, c, "/api/broker/vhost", func(v VHost) bool { return v.Name == name })
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("%w: vhost %q", ErrNotFound, name)
	}
	return match, nil
}

// paginate walks every page of a PageResponse<T> collection endpoint and returns
// the first element for which pred is true, or nil if none match.
func paginate[T any](ctx context.Context, c *Client, path string, pred func(T) bool) (*T, error) {
	const pageSize = 100
	for page := 0; ; page++ {
		var resp pageResponse[T]
		q := fmt.Sprintf("%s?page=%d&size=%d", path, page, pageSize)
		if err := c.do(ctx, "GET", q, nil, &resp); err != nil {
			return nil, err
		}
		for i := range resp.Content {
			if pred(resp.Content[i]) {
				return &resp.Content[i], nil
			}
		}
		if page+1 >= resp.TotalPages || len(resp.Content) == 0 {
			return nil, nil
		}
	}
}
