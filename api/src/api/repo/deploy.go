package repo

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type DeployState string

const (
	StatePending    DeployState = "pending"
	StateBuilding   DeployState = "building"
	StateRunning    DeployState = "running"
	StateFailed     DeployState = "failed"
	StateRolledBack DeployState = "rolled_back"
)

// valid transitions: pending→building, building→running|failed, running→rolled_back
var validTransitions = map[DeployState][]DeployState{
	StatePending:  {StateBuilding},
	StateBuilding: {StateRunning, StateFailed},
	StateRunning:  {StateBuilding, StateRolledBack}, // rebuild = new building state
}

type Deploy struct {
	ID            string      `json:"id"`
	State         DeployState `json:"state"`
	ImageTag      string      `json:"image_tag"`
	ContainerName string      `json:"container_name"`
	Port          int         `json:"port"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	Error         string      `json:"error,omitempty"`
}

type DeployRepo struct {
	mu      sync.RWMutex
	deploys map[string]*Deploy
	order   []string

	// ring buffers: deploy ID → log lines
	logs  map[string]*RingBuffer
	logMu sync.RWMutex

	// SSE subscribers: deploy ID → slice of line channels
	subs   map[string][]chan string
	subsMu sync.Mutex
}

func NewDeployRepo() *DeployRepo {
	return &DeployRepo{
		deploys: make(map[string]*Deploy),
		logs:    make(map[string]*RingBuffer),
		subs:    make(map[string][]chan string),
	}
}

func (r *DeployRepo) Create() *Deploy {
	d := &Deploy{
		ID:        uuid.New().String(),
		State:     StatePending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	r.mu.Lock()
	r.deploys[d.ID] = d
	r.order = append(r.order, d.ID)
	r.mu.Unlock()

	r.logMu.Lock()
	r.logs[d.ID] = NewRingBuffer(500)
	r.logMu.Unlock()

	return d
}

func (r *DeployRepo) Get(id string) (*Deploy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.deploys[id]
	return d, ok
}

func (r *DeployRepo) List() []*Deploy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Deploy, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.deploys[id])
	}
	return out
}

func (r *DeployRepo) Transition(id string, next DeployState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	d, ok := r.deploys[id]
	if !ok {
		return fmt.Errorf("deploy %s not found", id)
	}

	allowed := validTransitions[d.State]
	for _, s := range allowed {
		if s == next {
			d.State = next
			d.UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return fmt.Errorf("invalid transition %s → %s", d.State, next)
}

func (r *DeployRepo) SetError(id, msg string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.deploys[id]; ok {
		d.Error = msg
		d.State = StateFailed
		d.UpdatedAt = time.Now().UTC()
	}
}

func (r *DeployRepo) UpdateMeta(id, imageTag, containerName string, port int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d, ok := r.deploys[id]; ok {
		d.ImageTag = imageTag
		d.ContainerName = containerName
		d.Port = port
		d.UpdatedAt = time.Now().UTC()
	}
}

// Latest returns the most recently created deploy, or nil.
func (r *DeployRepo) Latest() *Deploy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.order) == 0 {
		return nil
	}
	return r.deploys[r.order[len(r.order)-1]]
}

// Previous returns the second-to-last deploy, or nil.
func (r *DeployRepo) Previous() *Deploy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.order) < 2 {
		return nil
	}
	return r.deploys[r.order[len(r.order)-2]]
}

func (r *DeployRepo) AppendLog(id, line string) {
	r.logMu.Lock()
	if buf, ok := r.logs[id]; ok {
		buf.Push(line)
	}
	r.logMu.Unlock()

	// fan-out to SSE subscribers
	r.subsMu.Lock()
	for _, ch := range r.subs[id] {
		select {
		case ch <- line:
		default: // slow consumer — drop
		}
	}
	r.subsMu.Unlock()
}

func (r *DeployRepo) GetLogs(id string) []string {
	r.logMu.RLock()
	defer r.logMu.RUnlock()
	if buf, ok := r.logs[id]; ok {
		return buf.Snapshot()
	}
	return nil
}

func (r *DeployRepo) Subscribe(id string) (replay []string, ch <-chan string, unsub func()) {
	r.logMu.RLock()
	var snap []string
	if buf, ok := r.logs[id]; ok {
		snap = buf.Snapshot()
	}
	r.logMu.RUnlock()

	c := make(chan string, 64)
	r.subsMu.Lock()
	r.subs[id] = append(r.subs[id], c)
	r.subsMu.Unlock()

	unsub = func() {
		r.subsMu.Lock()
		defer r.subsMu.Unlock()
		list := r.subs[id]
		for i, sub := range list {
			if sub == c {
				r.subs[id] = append(list[:i], list[i+1:]...)
				close(c)
				break
			}
		}
	}
	return snap, c, unsub
}

type RingBuffer struct {
	mu   sync.Mutex
	buf  []string
	size int
	head int
	full bool
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{buf: make([]string, size), size: size}
}

func (rb *RingBuffer) Push(line string) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.buf[rb.head] = line
	rb.head = (rb.head + 1) % rb.size
	if rb.head == 0 {
		rb.full = true
	}
}

func (rb *RingBuffer) Snapshot() []string {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if !rb.full {
		out := make([]string, rb.head)
		copy(out, rb.buf[:rb.head])
		return out
	}
	out := make([]string, rb.size)
	copy(out, rb.buf[rb.head:])
	copy(out[rb.size-rb.head:], rb.buf[:rb.head])
	return out
}
