package quiz

import (
	"fmt"
	"testing"
)

func quizOf(n int) Quiz {
	q := Quiz{StartMonth: 1, EndMonth: 2, Location: "US", Notes: "n"}
	for i := 0; i < n; i++ {
		q.Species = append(q.Species, Species{CommonName: fmt.Sprintf("Bird %03d", i)})
	}
	return q
}

func sizes(parts []Quiz) []int {
	out := make([]int, len(parts))
	for i, p := range parts {
		out[i] = len(p.Species)
	}
	return out
}

func TestSplitLeavesSmallQuizzesAlone(t *testing.T) {
	for _, n := range []int{0, 1, 40} {
		if parts := Split(quizOf(n), 40, 30); len(parts) != 1 || len(parts[0].Species) != n {
			t.Errorf("n=%d parts=%v", n, sizes(parts))
		}
	}
}

func TestSplitBalancesPartsUnderMax(t *testing.T) {
	cases := map[int][]int{
		41:  {21, 20},
		60:  {30, 30},
		61:  {21, 20, 20},
		100: {25, 25, 25, 25},
	}
	for n, want := range cases {
		got := sizes(Split(quizOf(n), 40, 30))
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("n=%d got %v want %v", n, got, want)
		}
	}
}

func TestSplitKeepsOrderHeaderAndAllSpecies(t *testing.T) {
	parts := Split(quizOf(65), 40, 30)
	next := 0
	for _, p := range parts {
		if p.Location != "US" || p.Notes != "n" || p.StartMonth != 1 || p.EndMonth != 2 {
			t.Fatalf("header lost: %+v", p)
		}
		for _, s := range p.Species {
			if want := fmt.Sprintf("Bird %03d", next); s.CommonName != want {
				t.Fatalf("got %s want %s", s.CommonName, want)
			}
			next++
		}
	}
	if next != 65 {
		t.Fatalf("species covered = %d", next)
	}
}
