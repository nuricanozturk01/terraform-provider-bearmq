package client

import (
	"context"
	"fmt"
	"net/url"
)

func queuesPath(vhostID string) string {
	return "/api/broker/vhost/" + url.PathEscape(vhostID) + "/queues"
}

// CreateQueue calls POST /api/broker/vhost/{vhostID}/queues.
func (c *Client) CreateQueue(ctx context.Context, vhostID string, req QueueRequest) (*Queue, error) {
	var out Queue
	if err := c.do(ctx, "POST", queuesPath(vhostID), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetQueue calls GET /api/broker/vhost/{vhostID}/queues/{queueID}.
func (c *Client) GetQueue(ctx context.Context, vhostID, queueID string) (*Queue, error) {
	var out Queue
	path := queuesPath(vhostID) + "/" + url.PathEscape(queueID)
	if err := c.do(ctx, "GET", path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteQueue calls DELETE /api/broker/vhost/{vhostID}/queues/{queueID}.
func (c *Client) DeleteQueue(ctx context.Context, vhostID, queueID string) error {
	return c.do(ctx, "DELETE", queuesPath(vhostID)+"/"+url.PathEscape(queueID), nil, nil)
}

// FindQueueByName pages through the vhost's queues for an exact name match.
func (c *Client) FindQueueByName(ctx context.Context, vhostID, name string) (*Queue, error) {
	match, err := paginate(ctx, c, queuesPath(vhostID), func(q Queue) bool { return q.Name == name })
	if err != nil {
		return nil, err
	}
	if match == nil {
		return nil, fmt.Errorf("%w: queue %q in vhost %s", ErrNotFound, name, vhostID)
	}
	return match, nil
}
