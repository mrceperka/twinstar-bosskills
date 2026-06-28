package router

import (
	"reflect"
	"testing"
)

func TestForEachRealmPrefix(t *testing.T) {
	var got []string
	ForEachRealmPrefix(func(prefix string) {
		got = append(got, prefix)
	})
	want := []string{
		"/Helios", "/helios",
		"/Athena", "/athena",
		"/Apollo", "/apollo",
		"/Proudmoore", "/proudmoore",
		"/MoPPvE", "/moppve",
		"/Kronos", "/kronos",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}
