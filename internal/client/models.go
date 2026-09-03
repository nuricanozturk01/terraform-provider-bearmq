package client

// Wire models. Field tags match the BearMQ REST contract exactly (see
// com.bearmq.common.broker.dto.* and com.bearmq.api.broker.dtos.read.*).

// VHost is the response of POST/GET /api/broker/vhost.
// Mirrors com.bearmq.common.vhost.dto.VirtualHostCreatedInfo (create) and
// VirtualHostInfo (get/list — password absent).
type VHost struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Domain    string `json:"domain"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	Status    string `json:"status"`
}

// CreateVHostRequest is the (optional) body of POST /api/broker/vhost.
type CreateVHostRequest struct {
	Name string `json:"name,omitempty"`
}

// UpdateVHostStatusRequest is the body of PATCH /api/broker/vhost/{id}/status.
type UpdateVHostStatusRequest struct {
	Status string `json:"status"`
}

// QueueRequest is the body of POST /api/broker/vhost/{id}/queues.
// Mirrors com.bearmq.common.broker.dto.QueueRequest.
type QueueRequest struct {
	Name       string         `json:"name"`
	Durable    bool           `json:"durable"`
	Exclusive  bool           `json:"exclusive"`
	AutoDelete bool           `json:"auto_delete"`
	Arguments  map[string]any `json:"arguments,omitempty"`
	DLQName    string         `json:"dlq_name,omitempty"`
}

// Queue is the response of the queue create/get endpoints.
// Mirrors com.bearmq.api.broker.dtos.read.QueueSummaryDto.
type Queue struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ActualName      string `json:"actualName"`
	Durable         bool   `json:"durable"`
	Exclusive       bool   `json:"exclusive"`
	AutoDelete      bool   `json:"autoDelete"`
	Status          string `json:"status"`
	DLQName         string `json:"dlqName"`
	OverflowPolicy  string `json:"overflowPolicy"`
	MaxMessageCount int64  `json:"maxMessageCount"`
}

// ExchangeRequest is the body of POST /api/broker/vhost/{id}/exchanges.
// Mirrors com.bearmq.common.broker.dto.ExchangeRequest.
type ExchangeRequest struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Durable  bool           `json:"durable"`
	Internal bool           `json:"internal"`
	Delayed  bool           `json:"delayed"`
	Args     map[string]any `json:"args,omitempty"`
}

// Exchange is the response of the exchange create/get endpoints.
// Mirrors com.bearmq.api.broker.dtos.read.ExchangeSummaryDto.
type Exchange struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ActualName string `json:"actualName"`
	Type       string `json:"type"`
	Durable    bool   `json:"durable"`
	Internal   bool   `json:"internal"`
	Status     string `json:"status"`
}

// BindRequest is the body of POST /api/broker/vhost/{id}/bindings.
// Mirrors com.bearmq.common.broker.dto.BindRequest.
type BindRequest struct {
	Source          string         `json:"source"`
	Destination     string         `json:"destination"`
	DestinationType string         `json:"destination_type"`
	RoutingKey      string         `json:"routing_key"`
	Arguments       map[string]any `json:"arguments,omitempty"`
}

// Binding is the response of the binding create/get endpoints.
// Mirrors com.bearmq.api.broker.dtos.read.BindingSummaryDto.
type Binding struct {
	ID                 string `json:"id"`
	SourceExchangeName string `json:"sourceExchangeName"`
	DestinationType    string `json:"destinationType"`
	DestinationName    string `json:"destinationName"`
	RoutingKey         string `json:"routingKey"`
	Status             string `json:"status"`
}

// pageResponse mirrors com.bearmq.common.broker.dto.PageResponse.
type pageResponse[T any] struct {
	Content       []T   `json:"content"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
	Size          int   `json:"size"`
	Number        int   `json:"number"`
}
