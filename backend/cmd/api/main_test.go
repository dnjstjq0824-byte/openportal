package main

import "testing"

func TestValidateQ(t *testing.T) {
	if validateQ("한국어 검색") != nil {
		t.Fatal("expected valid Korean query")
	}
	if validateQ("") == nil {
		t.Fatal("empty query should be rejected")
	}
}
func TestPageParam(t *testing.T) {
	n, err := pageParam("")
	if err != nil || n != 1 {
		t.Fatal("default page")
	}
	_, err = pageParam("101")
	if err == nil {
		t.Fatal("pagination bound")
	}
}
