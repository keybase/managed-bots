package base

import (
	"context"
	"errors"

	showtrends "github.com/keybase/showtrends-sdk/go"
)

type StatsBackend interface {
	Count(name string) error
	CountMult(name string, count int) error
	Value(name string, value float64) error
	Shutdown() error
}

type StatsBackendType int

const (
	DummyStatsBackendType StatsBackendType = iota
)

type ShowtrendsBackend struct {
	client *showtrends.Client
}

func NewShowtrendsBackend(addr string) *ShowtrendsBackend {
	return &ShowtrendsBackend{
		client: showtrends.NewClient(addr, "managed-bots", showtrends.DefaultBatchInterval),
	}
}

func (b *ShowtrendsBackend) Count(name string) error {
	b.client.CountOne(name)
	return nil
}

func (b *ShowtrendsBackend) CountMult(name string, count int) error {
	b.client.Count(name, float64(count))
	return nil
}

func (b *ShowtrendsBackend) Value(name string, value float64) error {
	b.client.Value(name, value)
	return nil
}

func (b *ShowtrendsBackend) Shutdown() error {
	return b.client.Close(context.Background())
}

var _ StatsBackend = (*ShowtrendsBackend)(nil)

type DummyStatsBackend struct {
	*DebugOutput
}

func NewDummyStatsBackend(debugConfig *ChatDebugOutputConfig) *DummyStatsBackend {
	return &DummyStatsBackend{
		DebugOutput: NewDebugOutput("DummyStatsBackend", debugConfig),
	}
}

func (d *DummyStatsBackend) Count(name string) error {
	d.Debug("Count: %s", name)
	return nil
}

func (d *DummyStatsBackend) CountMult(name string, count int) error {
	d.Debug("CountMulti: %s - %d", name, count)
	return nil
}

func (d *DummyStatsBackend) Value(name string, value float64) error {
	d.Debug("Value: %s - %.2f", name, value)
	return nil
}

func (d *DummyStatsBackend) Shutdown() error { return nil }

var _ StatsBackend = (*DummyStatsBackend)(nil)

func NewStatsBackend(btype StatsBackendType, config any) (StatsBackend, error) {
	switch btype {
	case DummyStatsBackendType:
		cfg, ok := config.(*ChatDebugOutputConfig)
		if !ok {
			return nil, errors.New("invalid DummyStatsBackend config")
		}
		return NewDummyStatsBackend(cfg), nil
	default:
		return nil, errors.New("unknown stats registry type")
	}
}

type StatsRegistry struct {
	*DebugOutput
	backend StatsBackend
	prefix  string
}

func (r *StatsRegistry) makeFname(name string) string {
	return r.prefix + name
}

func (r *StatsRegistry) SetPrefix(prefix string) *StatsRegistry {
	prefix = r.prefix + prefix + " - "
	return newStatsRegistryWithPrefix(r.Config(), r.backend, prefix)
}

func (r *StatsRegistry) ResetPrefix() *StatsRegistry {
	return NewStatsRegistryWithBackend(r.Config(), r.backend)
}

func (r *StatsRegistry) Count(name string) {
	if err := r.backend.Count(r.makeFname(name)); err != nil {
		r.Errorf("failed to post stat: err: %s name: %s", err, name)
	}
}

func (r *StatsRegistry) CountMult(name string, count int) {
	if err := r.backend.CountMult(r.makeFname(name), count); err != nil {
		r.Errorf("failed to post stat: err: %s name: %s", err, name)
	}
}

func (r *StatsRegistry) ValueInt(name string, value int) {
	r.Value(name, float64(value))
}

func (r *StatsRegistry) Value(name string, value float64) {
	if err := r.backend.Value(r.makeFname(name), value); err != nil {
		r.Errorf("failed to post stat: err: %s name: %s", err, name)
	}
}

func (r *StatsRegistry) Shutdown() (err error) {
	defer r.Trace(&err, "Shutdown")()
	return r.backend.Shutdown()
}

func NewStatsRegistryWithBackend(debugConfig *ChatDebugOutputConfig, backend StatsBackend) *StatsRegistry {
	return &StatsRegistry{
		DebugOutput: NewDebugOutput("StatsRegistry", debugConfig),
		backend:     backend,
	}
}

func newStatsRegistryWithPrefix(debugConfig *ChatDebugOutputConfig, backend StatsBackend, prefix string) *StatsRegistry {
	return &StatsRegistry{
		DebugOutput: NewDebugOutput("StatsRegistry - "+prefix, debugConfig),
		backend:     backend,
		prefix:      prefix,
	}
}

func NewStatsRegistry(debugConfig *ChatDebugOutputConfig, showtrendsAddr string) (reg *StatsRegistry, err error) {
	if showtrendsAddr != "" {
		return NewStatsRegistryWithBackend(debugConfig, NewShowtrendsBackend(showtrendsAddr)), nil
	}
	backend, err := NewStatsBackend(DummyStatsBackendType, debugConfig)
	if err != nil {
		return nil, err
	}
	return NewStatsRegistryWithBackend(debugConfig, backend), nil
}
