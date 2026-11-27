// Package matching ranks approved connected table sets without database writes.
package matching

import "sort"

type Table struct {
	ID, Zone  string
	Capacity  int
	Available bool
}
type Candidate struct {
	IDs   []string
	Zone  string
	Seats int
}

func Connected(ids []string, edges [][2]string) bool {
	if len(ids) == 0 {
		return false
	}
	seen := map[string]bool{ids[0]: true}
	for changed := true; changed; {
		changed = false
		for _, id := range ids {
			if seen[id] {
				continue
			}
			for _, edge := range edges {
				if (edge[0] == id && seen[edge[1]]) || (edge[1] == id && seen[edge[0]]) {
					seen[id] = true
					changed = true
					break
				}
			}
		}
	}
	return len(seen) == len(ids)
}

func Recommend(tables []Table, edges [][2]string, party int) []Candidate {
	result := []Candidate{}
	if party < 1 || party > 16 {
		return result
	}
	available := []Table{}
	for _, table := range tables {
		if table.Available {
			available = append(available, table)
		}
	}
	sort.Slice(available, func(i, j int) bool { return available[i].ID < available[j].ID })
	var visit func(int, []string, string, int)
	visit = func(start int, ids []string, zone string, seats int) {
		if len(ids) > 0 && seats >= party && Connected(ids, edges) {
			result = append(result, Candidate{append([]string{}, ids...), zone, seats})
		}
		if len(ids) == 3 {
			return
		}
		for i := start; i < len(available); i++ {
			table := available[i]
			if len(ids) > 0 && table.Zone != zone {
				continue
			}
			visit(i+1, append(ids, table.ID), table.Zone, seats+table.Capacity)
		}
	}
	visit(0, nil, "", 0)
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if len(a.IDs) != len(b.IDs) {
			return len(a.IDs) < len(b.IDs)
		}
		if a.Seats != b.Seats {
			return a.Seats < b.Seats
		}
		for k := range a.IDs {
			if a.IDs[k] != b.IDs[k] {
				return a.IDs[k] < b.IDs[k]
			}
		}
		return false
	})
	return result
}
