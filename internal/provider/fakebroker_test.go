package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nuricanozturk01/terraform-provider-bearmq/internal/client"
)

// fakeBroker is an in-memory stand-in for the BearMQ control-plane REST API,
// implementing exactly the endpoints this provider calls. It lets the acceptance
// tests run a real Terraform lifecycle (plan/apply/import/destroy) with no
// database or broker process.
type fakeBroker struct {
	mu        sync.Mutex
	seq       int
	vhosts    map[string]*client.VHost
	queues    map[string]map[string]*client.Queue
	exchanges map[string]map[string]*client.Exchange
	bindings  map[string]map[string]*client.Binding
}

func newFakeBroker(t *testing.T) (*httptest.Server, *fakeBroker) {
	t.Helper()
	fb := &fakeBroker{
		vhosts:    map[string]*client.VHost{},
		queues:    map[string]map[string]*client.Queue{},
		exchanges: map[string]map[string]*client.Exchange{},
		bindings:  map[string]map[string]*client.Binding{},
	}
	srv := httptest.NewServer(fb.routes())
	t.Cleanup(srv.Close)
	return srv, fb
}

func (fb *fakeBroker) id(prefix string) string {
	fb.seq++
	return fmt.Sprintf("%s_%03d", prefix, fb.seq)
}

func (fb *fakeBroker) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/broker/vhost", fb.createVHost)
	mux.HandleFunc("GET /api/broker/vhost", fb.listVHosts)
	mux.HandleFunc("GET /api/broker/vhost/{vid}", fb.getVHost)
	mux.HandleFunc("PATCH /api/broker/vhost/{vid}/status", fb.setVHostStatus)
	mux.HandleFunc("DELETE /api/broker/vhost/{vid}", fb.deleteVHost)

	mux.HandleFunc("POST /api/broker/vhost/{vid}/queues", fb.createQueue)
	mux.HandleFunc("GET /api/broker/vhost/{vid}/queues", fb.listQueues)
	mux.HandleFunc("GET /api/broker/vhost/{vid}/queues/{qid}", fb.getQueue)
	mux.HandleFunc("DELETE /api/broker/vhost/{vid}/queues/{qid}", fb.deleteQueue)

	mux.HandleFunc("POST /api/broker/vhost/{vid}/exchanges", fb.createExchange)
	mux.HandleFunc("GET /api/broker/vhost/{vid}/exchanges", fb.listExchanges)
	mux.HandleFunc("GET /api/broker/vhost/{vid}/exchanges/{eid}", fb.getExchange)
	mux.HandleFunc("DELETE /api/broker/vhost/{vid}/exchanges/{eid}", fb.deleteExchange)

	mux.HandleFunc("POST /api/broker/vhost/{vid}/bindings", fb.createBinding)
	mux.HandleFunc("GET /api/broker/vhost/{vid}/bindings/{bid}", fb.getBinding)
	mux.HandleFunc("DELETE /api/broker/vhost/{vid}/bindings/{bid}", fb.deleteBinding)

	return fb.requireAPIKey(mux)
}

func (fb *fakeBroker) requireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") == "" && r.Header.Get("Authorization") == "" {
			writeErr(w, http.StatusUnauthorized, "missing credentials")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- vhost ---

func (fb *fakeBroker) createVHost(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()

	var body client.CreateVHostRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	name := body.Name
	if name == "" {
		name = fb.id("vh-auto")
	}
	id := fb.id("vh")
	vh := &client.VHost{
		ID: id, Name: name, Username: "u_" + id, Password: "secret_" + id,
		Domain: name + ".localhost", URL: "amqp://u_" + id + "@localhost/" + name,
		CreatedAt: time.Now().UTC().Format(time.RFC3339), Status: statusActive,
	}
	fb.vhosts[id] = vh
	fb.queues[id] = map[string]*client.Queue{}
	fb.exchanges[id] = map[string]*client.Exchange{}
	fb.bindings[id] = map[string]*client.Binding{}
	writeJSON(w, http.StatusOK, vh)
}

func (fb *fakeBroker) listVHosts(w http.ResponseWriter, _ *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	out := make([]client.VHost, 0, len(fb.vhosts))
	for _, v := range fb.vhosts {
		out = append(out, *v)
	}
	writePage(w, out)
}

func (fb *fakeBroker) getVHost(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vh, ok := fb.vhosts[r.PathValue("vid")]
	if !ok {
		writeErr(w, http.StatusNotFound, "virtual host not found")
		return
	}
	writeJSON(w, http.StatusOK, vh)
}

func (fb *fakeBroker) setVHostStatus(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vh, ok := fb.vhosts[r.PathValue("vid")]
	if !ok {
		writeErr(w, http.StatusNotFound, "virtual host not found")
		return
	}
	var body client.UpdateVHostStatusRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	vh.Status = body.Status
	writeJSON(w, http.StatusOK, vh)
}

func (fb *fakeBroker) deleteVHost(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	id := r.PathValue("vid")
	if _, ok := fb.vhosts[id]; !ok {
		writeErr(w, http.StatusNotFound, "virtual host not found")
		return
	}
	delete(fb.vhosts, id)
	w.WriteHeader(http.StatusNoContent)
}

// --- queue ---

func (fb *fakeBroker) createQueue(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vid := r.PathValue("vid")
	if _, ok := fb.vhosts[vid]; !ok {
		writeErr(w, http.StatusBadRequest, "virtual host not found")
		return
	}
	var body client.QueueRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	for _, q := range fb.queues[vid] {
		if q.Name == body.Name {
			writeErr(w, http.StatusConflict, "queue already exists")
			return
		}
	}
	id := fb.id("q")
	q := &client.Queue{
		ID: id, Name: body.Name, ActualName: "queue-" + id,
		Durable: body.Durable, Exclusive: body.Exclusive, AutoDelete: body.AutoDelete,
		Status: statusActive, DLQName: body.DLQName, OverflowPolicy: "DEAD_LETTER_QUEUE",
		MaxMessageCount: 1024,
	}
	fb.queues[vid][id] = q
	writeJSON(w, http.StatusCreated, q)
}

func (fb *fakeBroker) listQueues(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	out := make([]client.Queue, 0)
	for _, q := range fb.queues[r.PathValue("vid")] {
		out = append(out, *q)
	}
	writePage(w, out)
}

func (fb *fakeBroker) getQueue(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	q, ok := fb.queues[r.PathValue("vid")][r.PathValue("qid")]
	if !ok {
		writeErr(w, http.StatusNotFound, "queue not found")
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (fb *fakeBroker) deleteQueue(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vid, qid := r.PathValue("vid"), r.PathValue("qid")
	if _, ok := fb.queues[vid][qid]; !ok {
		writeErr(w, http.StatusNotFound, "queue not found")
		return
	}
	delete(fb.queues[vid], qid)
	w.WriteHeader(http.StatusNoContent)
}

// --- exchange ---

func (fb *fakeBroker) createExchange(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vid := r.PathValue("vid")
	if _, ok := fb.vhosts[vid]; !ok {
		writeErr(w, http.StatusBadRequest, "virtual host not found")
		return
	}
	var body client.ExchangeRequest
	_ = json.NewDecoder(r.Body).Decode(&body)
	if !isKnownExchangeType(body.Type) {
		writeErr(w, http.StatusBadRequest, "unsupported exchange type")
		return
	}
	for _, e := range fb.exchanges[vid] {
		if e.Name == body.Name {
			writeErr(w, http.StatusConflict, "exchange already exists")
			return
		}
	}
	id := fb.id("ex")
	e := &client.Exchange{
		ID: id, Name: body.Name, ActualName: "exchange-" + id,
		Type: strings.ToUpper(body.Type), Durable: body.Durable, Internal: body.Internal,
		Status: statusActive,
	}
	fb.exchanges[vid][id] = e
	writeJSON(w, http.StatusCreated, e)
}

func (fb *fakeBroker) listExchanges(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	out := make([]client.Exchange, 0)
	for _, e := range fb.exchanges[r.PathValue("vid")] {
		out = append(out, *e)
	}
	writePage(w, out)
}

func (fb *fakeBroker) getExchange(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	e, ok := fb.exchanges[r.PathValue("vid")][r.PathValue("eid")]
	if !ok {
		writeErr(w, http.StatusNotFound, "exchange not found")
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (fb *fakeBroker) deleteExchange(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vid, eid := r.PathValue("vid"), r.PathValue("eid")
	if _, ok := fb.exchanges[vid][eid]; !ok {
		writeErr(w, http.StatusNotFound, "exchange not found")
		return
	}
	delete(fb.exchanges[vid], eid)
	w.WriteHeader(http.StatusNoContent)
}

// --- binding ---

func (fb *fakeBroker) createBinding(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vid := r.PathValue("vid")
	if _, ok := fb.vhosts[vid]; !ok {
		writeErr(w, http.StatusBadRequest, "virtual host not found")
		return
	}
	var body client.BindRequest
	_ = json.NewDecoder(r.Body).Decode(&body)

	if !fb.exchangeNameExists(vid, body.Source) {
		writeErr(w, http.StatusNotFound, "source exchange not found: "+body.Source)
		return
	}
	destOK := fb.queueNameExists(vid, body.Destination)
	if strings.EqualFold(body.DestinationType, "EXCHANGE") {
		destOK = fb.exchangeNameExists(vid, body.Destination)
	}
	if !destOK {
		writeErr(w, http.StatusNotFound, "destination not found: "+body.Destination)
		return
	}

	id := fb.id("b")
	b := &client.Binding{
		ID: id, SourceExchangeName: body.Source, DestinationName: body.Destination,
		DestinationType: strings.ToUpper(body.DestinationType), RoutingKey: body.RoutingKey,
		Status: statusActive,
	}
	fb.bindings[vid][id] = b
	writeJSON(w, http.StatusCreated, b)
}

func (fb *fakeBroker) getBinding(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	b, ok := fb.bindings[r.PathValue("vid")][r.PathValue("bid")]
	if !ok {
		writeErr(w, http.StatusNotFound, "binding not found")
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (fb *fakeBroker) deleteBinding(w http.ResponseWriter, r *http.Request) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	vid, bid := r.PathValue("vid"), r.PathValue("bid")
	if _, ok := fb.bindings[vid][bid]; !ok {
		writeErr(w, http.StatusNotFound, "binding not found")
		return
	}
	delete(fb.bindings[vid], bid)
	w.WriteHeader(http.StatusNoContent)
}

func (fb *fakeBroker) exchangeNameExists(vid, name string) bool {
	for _, e := range fb.exchanges[vid] {
		if e.Name == name {
			return true
		}
	}
	return false
}

func (fb *fakeBroker) queueNameExists(vid, name string) bool {
	for _, q := range fb.queues[vid] {
		if q.Name == name {
			return true
		}
	}
	return false
}

// --- wire helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writePage[T any](w http.ResponseWriter, content []T) {
	writeJSON(w, http.StatusOK, map[string]any{
		"content":       content,
		"totalElements": len(content),
		"totalPages":    1,
		"size":          len(content),
		"number":        0,
	})
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"status": status, "error": http.StatusText(status), "message": msg, "path": "/",
	})
}

func isKnownExchangeType(t string) bool {
	switch strings.ToUpper(t) {
	case "DIRECT", "FANOUT", "TOPIC", "HEADERS":
		return true
	default:
		return false
	}
}
