package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

func TestAttemptIDDeterministic(t *testing.T) {
	a:=attemptID("invoice:inv_123")
	b:=attemptID("invoice:inv_123")
	if a!=b || a=="" { t.Fatalf("unexpected attempt ids %q %q",a,b) }
	if a==attemptID("invoice:inv_124") { t.Fatal("different idempotency keys must not collide in deterministic contract") }
}

func TestStripeSignatureVerification(t *testing.T) {
	secret:="whsec_start234_test"
	body:=[]byte(`{"id":"evt_234","type":"payment_intent.succeeded"}`)
	now:=time.Unix(1770000000,0).UTC()
	ts:=now.Unix()
	mac:=hmac.New(sha256.New,[]byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts,10)+"."+string(body)))
	header:="t="+strconv.FormatInt(ts,10)+",v1="+hex.EncodeToString(mac.Sum(nil))
	if err:=verifyStripeSignature(body,header,secret,now,5*time.Minute);err!=nil{t.Fatalf("valid signature rejected: %v",err)}
	if err:=verifyStripeSignature([]byte("tampered"),header,secret,now,5*time.Minute);err==nil{t.Fatal("tampered body accepted")}
	if err:=verifyStripeSignature(body,header,secret,now.Add(6*time.Minute),5*time.Minute);err==nil{t.Fatal("stale signature accepted")}
}
