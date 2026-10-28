package goinsta

import (
	"context"
	"errors"
	"testing"
)

func countingPager(pages ...[]int) (*Pager[int], *int) {
	calls := 0
	return newPager(func(context.Context) ([]int, bool, error) {
		p := pages[calls]
		calls++
		return p, calls < len(pages), nil
	}), &calls
}

func TestPagerAll(t *testing.T) {
	p, calls := countingPager([]int{1, 2}, []int{3}, []int{4, 5})
	var got []int
	for v, err := range p.All(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, v)
	}
	if len(got) != 5 || got[4] != 5 || *calls != 3 {
		t.Errorf("got %v after %d calls", got, *calls)
	}
	if err := p.Next(t.Context()); !errors.Is(err, ErrNoMore) {
		t.Errorf("past the end: %v", err)
	}
}

func TestPagerCollectStopsEarly(t *testing.T) {
	p, calls := countingPager([]int{1, 2}, []int{3, 4}, []int{5})
	got, err := p.Collect(t.Context(), 3)
	if err != nil || len(got) != 3 || *calls != 2 {
		t.Errorf("got %v err %v after %d calls", got, err, *calls)
	}
}

func TestPagerYieldsError(t *testing.T) {
	boom := errors.New("boom")
	p := newPager(func(context.Context) ([]int, bool, error) { return nil, false, boom })
	seen := 0
	for _, err := range p.All(t.Context()) {
		seen++
		if !errors.Is(err, boom) {
			t.Errorf("err = %v", err)
		}
	}
	if seen != 1 {
		t.Errorf("yielded %d times, want the error once", seen)
	}
}
