package automation

import (
	"bytes"
	"net/http"
	"testing"
	"time"
)

func TestServiceIdentitySignature(t *testing.T) {
	now:=time.Date(2026,9,24,17,0,0,0,time.UTC)
	body:=[]byte(`{"event":"workflow.qc.passed.v1"}`)
	req,_:=http.NewRequest(http.MethodPost,"http://automation/internal/v1/automation/events",bytes.NewReader(body))
	secret:="01234567890123456789012345678901"
	if err:=SignRequest(req,body,"workshop",secret,now);err!=nil{t.Fatal(err)}
	got,err:=VerifyRequest(req,body,func(id string)(string,bool){return secret,id=="workshop"},now,5*time.Minute)
	if err!=nil||got!="workshop"{t.Fatalf("verification failed: %s %v",got,err)}
}

func TestServiceIdentityRejectsTamperAndReplay(t *testing.T) {
	now:=time.Date(2026,9,24,17,0,0,0,time.UTC)
	body:=[]byte("payload")
	req,_:=http.NewRequest(http.MethodPost,"http://automation/internal/v1/automation/events",bytes.NewReader(body))
	secret:="01234567890123456789012345678901"
	if err:=SignRequest(req,body,"finance",secret,now);err!=nil{t.Fatal(err)}
	if _,err:=VerifyRequest(req,[]byte("changed"),func(id string)(string,bool){return secret,true},now,5*time.Minute);err==nil{t.Fatal("tampered body must fail")}
	if _,err:=VerifyRequest(req,body,func(id string)(string,bool){return secret,true},now.Add(6*time.Minute),5*time.Minute);err==nil{t.Fatal("expired signature must fail")}
}
