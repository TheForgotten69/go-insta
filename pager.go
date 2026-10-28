package goinsta

import (
	"context"
	"errors"
	"iter"
)

// Pager walks a paged endpoint. Next loads the following page into Items and
// returns ErrNoMore past the last one; All ranges over the items of every
// remaining page:
//
//	for post, err := range user.Posts().All(ctx) { ... }
//
// A Pager is not safe for concurrent use.
type Pager[T any] struct {
	Items []T

	done  bool
	fetch func(ctx context.Context) (items []T, more bool, err error)
}

func newPager[T any](fetch func(ctx context.Context) ([]T, bool, error)) *Pager[T] {
	return &Pager[T]{fetch: fetch}
}

func (p *Pager[T]) More() bool { return !p.done }

func (p *Pager[T]) Next(ctx context.Context) error {
	if p.done {
		return ErrNoMore
	}
	items, more, err := p.fetch(ctx)
	if err != nil {
		return err
	}
	p.Items, p.done = items, !more
	return nil
}

// All loads pages from the current position on and yields their items. It
// stops at the end of the feed, or after yielding a non-nil error.
func (p *Pager[T]) All(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for {
			if err := p.Next(ctx); err != nil {
				if !errors.Is(err, ErrNoMore) {
					var zero T
					yield(zero, err)
				}
				return
			}
			for _, it := range p.Items {
				if !yield(it, nil) {
					return
				}
			}
		}
	}
}

// Collect loads up to limit items (all when limit <= 0) from the current
// position. Stopping at limit leaves the rest of the last page unread in Items.
func (p *Pager[T]) Collect(ctx context.Context, limit int) ([]T, error) {
	var out []T
	for it, err := range p.All(ctx) {
		if err != nil {
			return out, err
		}
		out = append(out, it)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
