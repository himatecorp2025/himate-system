package main

import "testing"

func TestHealthStringValue(t *testing.T){
	if stringValue(nil)!=""{t.Fatal("nil must become empty string")}
	if stringValue("OK")!="OK"{t.Fatal("string conversion failed")}
}
