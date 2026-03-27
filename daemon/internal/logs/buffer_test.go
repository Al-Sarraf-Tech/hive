package logs

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingBufferPushAndLast(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push 5 entries into a capacity-3 buffer
	for i := 0; i < 5; i++ {
		rb.Push(Entry{Line: fmt.Sprintf("line-%d", i)})
	}

	got := rb.Last(10) // ask for more than capacity
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	// Should have the 3 most recent, oldest first
	if got[0].Line != "line-2" {
		t.Errorf("got[0] = %q, want %q", got[0].Line, "line-2")
	}
	if got[1].Line != "line-3" {
		t.Errorf("got[1] = %q, want %q", got[1].Line, "line-3")
	}
	if got[2].Line != "line-4" {
		t.Errorf("got[2] = %q, want %q", got[2].Line, "line-4")
	}
}

func TestRingBufferLastPartial(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Push(Entry{Line: "a"})
	rb.Push(Entry{Line: "b"})

	got := rb.Last(1)
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	if got[0].Line != "b" {
		t.Errorf("got %q, want %q", got[0].Line, "b")
	}
}

func TestRingBufferLastEmpty(t *testing.T) {
	rb := NewRingBuffer(5)
	got := rb.Last(10)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestRingBufferForService(t *testing.T) {
	rb := NewRingBuffer(100)

	for i := 0; i < 10; i++ {
		svc := "web"
		if i%2 == 0 {
			svc = "api"
		}
		rb.Push(Entry{ServiceName: svc, Line: fmt.Sprintf("line-%d", i)})
	}

	got := rb.ForService("api", 100)
	if len(got) != 5 {
		t.Fatalf("expected 5 api entries, got %d", len(got))
	}
	// Oldest first
	if got[0].Line != "line-0" {
		t.Errorf("got[0] = %q, want %q", got[0].Line, "line-0")
	}
	if got[4].Line != "line-8" {
		t.Errorf("got[4] = %q, want %q", got[4].Line, "line-8")
	}

	// Limit
	got = rb.ForService("api", 2)
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Line != "line-6" {
		t.Errorf("got[0] = %q, want %q", got[0].Line, "line-6")
	}
}

func TestRingBufferConcurrency(t *testing.T) {
	rb := NewRingBuffer(1000)
	now := time.Now()

	var wg sync.WaitGroup
	// 10 writers, 10 readers
	for w := 0; w < 10; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				rb.Push(Entry{
					ServiceName: fmt.Sprintf("svc-%d", id),
					Line:        fmt.Sprintf("w%d-line-%d", id, i),
					Timestamp:   now,
				})
			}
		}(w)
	}
	for r := 0; r < 10; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = rb.Last(50)
				_ = rb.ForService("svc-0", 10)
			}
		}()
	}
	wg.Wait()

	// Just verify we didn't panic/race
	got := rb.Last(1000)
	if len(got) != 1000 {
		t.Errorf("expected 1000 entries, got %d", len(got))
	}
}
