package s3x

import (
	"context"
	"testing"

	"s3scalpel/internal/model"
)

func TestInvalidateOnlyDropsOneConnection(t *testing.T) {
	m := NewManager()
	ctx := context.Background()

	a := model.Connection{ID: "a", Endpoint: "https://a.example", AccessKey: "k", SecretKey: "s"}
	b := model.Connection{ID: "b", Endpoint: "https://b.example", AccessKey: "k", SecretKey: "s"}
	for _, c := range []model.Connection{a, b} {
		if _, err := m.Client(ctx, c); err != nil {
			t.Fatalf("build client for %s: %v", c.ID, err)
		}
	}
	if len(m.clients) != 2 {
		t.Fatalf("cached %d clients, want 2", len(m.clients))
	}

	m.Invalidate("a")
	if len(m.clients) != 1 {
		t.Fatalf("after invalidating a, cached %d clients, want 1", len(m.clients))
	}
	if _, ok := m.clients[fingerprint(b)]; !ok {
		t.Error("connection b's client should have survived invalidation of a")
	}

	m.Invalidate("")
	if len(m.clients) != 0 || len(m.byConn) != 0 {
		t.Errorf("empty id should clear the whole cache, got %d clients / %d index entries", len(m.clients), len(m.byConn))
	}
}

func TestEditedConnectionGetsFreshClient(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	c := model.Connection{ID: "a", Endpoint: "https://a.example", AccessKey: "k", SecretKey: "s"}

	first, err := m.Client(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	same, err := m.Client(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if first != same {
		t.Error("an unchanged connection should reuse its cached client")
	}

	c.SecretKey = "rotated"
	rotated, err := m.Client(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if rotated == first {
		t.Error("a rotated secret must produce a new client")
	}
}
