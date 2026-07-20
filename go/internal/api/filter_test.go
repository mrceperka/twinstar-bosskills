package api

import (
	"net/url"
	"strings"
	"testing"
)

func TestQuery_Encode_Defaults(t *testing.T) {
	q := Query{}
	got := q.Encode()
	v, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if v.Get("realm") != "Helios" {
		t.Errorf("realm: %s", v.Get("realm"))
	}
	if v.Get("page") != "0" {
		t.Errorf("page: %s", v.Get("page"))
	}
	if v.Get("pageSize") != "100" {
		t.Errorf("pageSize: %s", v.Get("pageSize"))
	}
}

func TestQuery_Encode_FiltersAreJSON(t *testing.T) {
	q := Query{
		Realm: "Proudmoore",
		Filters: []Filter{
			{Column: "entry", Operator: OpEquals, Value: 71358},
		},
	}
	got := q.Encode()
	if !strings.Contains(got, "filters=") {
		t.Fatal("missing filters param")
	}
	v, _ := url.ParseQuery(got)
	if !strings.Contains(v.Get("filters"), `"column":"entry"`) ||
		!strings.Contains(v.Get("filters"), `"operator":"equals"`) {
		t.Errorf("filters payload: %s", v.Get("filters"))
	}
}

func TestQuery_Encode_RealmCapitalised(t *testing.T) {
	q := Query{Realm: "helios"}
	v, _ := url.ParseQuery(q.Encode())
	if v.Get("realm") != "Helios" {
		t.Errorf("realm capitalisation: %s", v.Get("realm"))
	}
}

func TestQuery_Encode_OptionalIntsRespectNil(t *testing.T) {
	q := Query{}
	v, _ := url.ParseQuery(q.Encode())
	if v.Has("mode") {
		t.Error("mode should be absent when Difficulty nil")
	}
	if v.Has("talent_spec") {
		t.Error("talent_spec should be absent when TalentSpec nil")
	}

	d := 0
	s := 1
	q = Query{Difficulty: &d, TalentSpec: &s}
	v, _ = url.ParseQuery(q.Encode())
	if v.Get("mode") != "0" {
		t.Errorf("mode: %s", v.Get("mode"))
	}
	if v.Get("talent_spec") != "1" {
		t.Errorf("talent_spec: %s", v.Get("talent_spec"))
	}
}
