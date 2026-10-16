package ui

import (
	"reflect"
	"testing"
)

func TestFuzzyRankPrefersSubstring(t *testing.T) {
	items := []string{
		"abc-shop",
		"db-cluster-shop",
		"team-shop-xyz-shop-a1b2c",
		"team-shop-foo-shop-d3e4f",
		"deployment-test-shop",
		"foo-shop",
	}
	hay := func(s string) string { return s + "  " + s }
	want := []string{"foo-shop", "team-shop-foo-shop-d3e4f"}
	for _, q := range []string{"foo", "foo-sh", "foo-shop"} {
		if got := fuzzyRank(items, q, hay); !reflect.DeepEqual(got, want) {
			t.Fatalf("q=%q: got %v, want %v", q, got, want)
		}
	}
}

func TestFuzzyRankFallsBackToSubsequence(t *testing.T) {
	items := []string{"kube-system", "kube-public", "default"}
	got := fuzzyRank(items, "ksys", func(s string) string { return s })
	if !reflect.DeepEqual(got, []string{"kube-system"}) {
		t.Fatalf("got %v", got)
	}
}

func TestFuzzyRankEmptyQueryKeepsOrder(t *testing.T) {
	items := []string{"b", "a", "c"}
	got := fuzzyRank(items, "", func(s string) string { return s })
	if !reflect.DeepEqual(got, items) {
		t.Fatalf("got %v", got)
	}
}

func TestSelectorNamespaceFilter(t *testing.T) {
	s := newSelector(Theme{})
	s.openLoading(selNamespace, "Switch namespace", "namespace")
	items := []selItem{{title: "all namespaces", desc: "*", id: "*"}}
	for _, n := range []string{"abc-shop", "team-shop-foo-shop-d3e4f", "db-shop", "foo-shop"} {
		items = append(items, selItem{title: n, id: n})
	}
	s.setItems(items)
	s.input.SetValue("foo-shop")
	s.refilter()

	var got []string
	for _, i := range s.match {
		got = append(got, s.items[i].title)
	}
	want := []string{"foo-shop", "team-shop-foo-shop-d3e4f"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
