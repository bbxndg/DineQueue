package matching

import "testing"

func TestConnectedRanking(t *testing.T) {
	tables := []Table{{"a", "z", 6, true}, {"b", "z", 4, true}, {"c", "z", 8, true}}
	edges := [][2]string{{"a", "b"}}
	got := Recommend(tables, edges, 10)
	if len(got) != 1 || got[0].Seats != 10 || len(got[0].IDs) != 2 {
		t.Fatalf("party ten: %+v", got)
	}
	if len(Recommend(tables, nil, 10)) != 0 {
		t.Fatal("disconnected set accepted")
	}
	tables[1].Available = false
	if len(Recommend(tables, edges, 10)) != 0 {
		t.Fatal("busy table accepted")
	}
}
func TestChainAndDeterminism(t *testing.T) {
	tables := []Table{{"c", "z", 2, true}, {"b", "z", 4, true}, {"a", "z", 6, true}}
	got := Recommend(tables, [][2]string{{"a", "b"}, {"b", "c"}}, 12)
	if len(got) != 1 || len(got[0].IDs) != 3 {
		t.Fatalf("chain: %+v", got)
	}
	got = Recommend(tables, [][2]string{{"a", "b"}, {"b", "c"}}, 4)
	if got[0].IDs[0] != "b" {
		t.Fatalf("ranking: %+v", got)
	}
}

func TestBoundariesAndScope(t *testing.T) {
	tables := []Table{{"a", "z", 8, true}, {"b", "z", 8, true}, {"c", "other", 8, true}}
	for _, n := range []int{1, 16} {
		if len(Recommend(tables, [][2]string{{"a", "b"}}, n)) == 0 {
			t.Fatalf("party %d", n)
		}
	}
	for _, n := range []int{0, 17} {
		if len(Recommend(tables, nil, n)) != 0 {
			t.Fatalf("invalid party %d", n)
		}
	}
	if len(Recommend(tables, [][2]string{{"a", "c"}}, 16)) != 0 {
		t.Fatal("cross zone")
	}
	if Connected([]string{"a", "c"}, [][2]string{{"a", "b"}, {"b", "c"}}) {
		t.Fatal("unselected middle table")
	}
}
