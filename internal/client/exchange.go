package client

import (
	"context"
	"fmt"
	"net/url"
)

func exchangesPath(vhostID string) string {
	return "/api/broker/vhost/" + url.PathEscape(vhostID) + "/exchanges"
}

// CreateExchange calls POST /api/broker/vhost/{vhostID}/exchanges.
func (c *Client) CreateExchange(ctx context.Context, vhostID string, req ExchangeRequest) (*Exchange, error) {
	var out Exchange
	if err := c.do(ctx, "POST", exchangesPath(vhostID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetExchange calls GET /api/broker/vhost/{vhostID}/exchanges/{exchangeID}.
func (c *Client) GetExchange(ctx context.Context, vhostID, exchangeID string) (*Exchange, error) {
	var out Exchange
	path := exchangesPath(vhostID) + "/" + url.PathEscape(exchangeID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteExchange calls DELETE /api/broker/vhost/{vhostID}/exchanges/{exchangeID}.
func (c *Client) DeleteExchange(ctx context.Context, vhostID, exchangeID string) error {
	return c.do(ctx, "DELETE", exchangesPath(vhostID)+"/"+url.PathEscape(exchangeID), nil, nil)
}

// FindExchangeByName pages through the vhost's exchanges for an exact name match.
func (c *Client) FindExchangeByName(ctx context.Context, vhostID, name string) (*Exchange, error) {
	match, err := paginate(ctx, c, exchangesPath(vhostID), func(e Exchange) bool { return e.Name == name })
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("%w: exchange %q in vhost %s", ErrNotFound, name, vhostID)
	}
	return match, nil
}
