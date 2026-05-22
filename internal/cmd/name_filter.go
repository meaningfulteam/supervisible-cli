package cmd

import (
	"sort"
	"strings"
)

// filterByName returns the subset of items whose getName(item) contains needle
// case-insensitively. Returns items unchanged when needle is empty.
func filterByName[T any](items []T, needle string, getName func(T) string) []T {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return items
	}
	out := make([]T, 0, len(items))
	for _, it := range items {
		name := strings.ToLower(getName(it))
		if strings.Contains(name, needle) {
			out = append(out, it)
		}
	}
	return out
}

// emitNameMissWarning emits a single stderr warning when --name returned zero
// matches. Includes did-you-mean candidates if any look close. Caller supplies
// the Aux writer to keep this package-free of output dependencies.
func emitNameMissWarning[T any](aux func(format string, args ...any), entity string, items []T, needle string, getName func(T) string) {
	hints := suggestNames(items, needle, getName, 5)
	if len(hints) > 0 {
		aux("warning: no %s match --name %q. Did you mean: %s?", entity, needle, strings.Join(hints, ", "))
		return
	}
	aux("warning: no %s match --name %q.", entity, needle)
}

// suggestNames returns up to max candidate names that look close to needle.
// Scores combine substring containment, shared prefix length, and per-character
// overlap. Returns empty when nothing is close enough. Stable order: highest
// score first, ties broken by lexical order so tests don't flake on map order.
func suggestNames[T any](items []T, needle string, getName func(T) string, max int) []string {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" || len(items) == 0 {
		return nil
	}

	type scored struct {
		name  string
		score int
	}
	scoredAll := make([]scored, 0, len(items))
	for _, it := range items {
		raw := strings.TrimSpace(getName(it))
		if raw == "" {
			continue
		}
		low := strings.ToLower(raw)
		s := 0
		if strings.Contains(low, needle) {
			s += 100
		}
		if strings.HasPrefix(low, needle) {
			s += 50
		}
		s += sharedPrefixLen(low, needle) * 10
		s += charOverlap(low, needle)
		if s <= 0 {
			continue
		}
		scoredAll = append(scoredAll, scored{name: raw, score: s})
	}

	if len(scoredAll) == 0 {
		return nil
	}
	sort.SliceStable(scoredAll, func(i, j int) bool {
		if scoredAll[i].score != scoredAll[j].score {
			return scoredAll[i].score > scoredAll[j].score
		}
		return scoredAll[i].name < scoredAll[j].name
	})

	if max <= 0 || max > len(scoredAll) {
		max = len(scoredAll)
	}
	out := make([]string, 0, max)
	seen := make(map[string]struct{}, max)
	for _, s := range scoredAll {
		if _, dup := seen[s.name]; dup {
			continue
		}
		seen[s.name] = struct{}{}
		out = append(out, s.name)
		if len(out) >= max {
			break
		}
	}
	return out
}

func sharedPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// charOverlap counts how many of needle's characters appear (in any position)
// in candidate. Cheap proxy for "looks vaguely similar".
func charOverlap(candidate, needle string) int {
	overlap := 0
	for _, r := range needle {
		if strings.ContainsRune(candidate, r) {
			overlap++
		}
	}
	return overlap
}
