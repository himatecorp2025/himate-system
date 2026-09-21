package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChunkedEncryptionRoundTripAndTamperDetection(t *testing.T) {
	dir:=t.TempDir()
	source:=filepath.Join(dir,"source.bin")
	encrypted:=filepath.Join(dir,"encrypted.hmbk")
	restored:=filepath.Join(dir,"restored.bin")
	payload:=bytes.Repeat([]byte("HIMATE-backup-test-"),150000)
	if err:=os.WriteFile(source,payload,0600);err!=nil{t.Fatal(err)}
	key:=[]byte("0123456789abcdef0123456789abcdef")
	hash,size,err:=encryptFile(source,encrypted,key)
	if err!=nil{t.Fatal(err)}
	if hash==""||size<=int64(len(payload)){t.Fatalf("invalid encrypted metadata hash=%q size=%d",hash,size)}
	reader,err:=os.Open(encrypted);if err!=nil{t.Fatal(err)}
	if err:=decryptStreamToFile(reader,restored,key);err!=nil{reader.Close();t.Fatal(err)}
	reader.Close()
	got,err:=os.ReadFile(restored);if err!=nil{t.Fatal(err)}
	if !bytes.Equal(got,payload){t.Fatal("decrypted artifact differs from source")}

	raw,err:=os.ReadFile(encrypted);if err!=nil{t.Fatal(err)}
	raw[len(raw)-1]^=0xff
	tampered:=filepath.Join(dir,"tampered.hmbk")
	if err:=os.WriteFile(tampered,raw,0600);err!=nil{t.Fatal(err)}
	reader,err=os.Open(tampered);if err!=nil{t.Fatal(err)}
	err=decryptStreamToFile(reader,filepath.Join(dir,"bad.bin"),key);reader.Close()
	if err==nil{t.Fatal("tampered encrypted artifact unexpectedly decrypted")}
}

func TestLocalOffsiteRejectsTraversalAndRoundTrips(t *testing.T) {
	root:=t.TempDir()
	provider:=&localOffsite{root:root}
	source:=filepath.Join(t.TempDir(),"artifact")
	if err:=os.WriteFile(source,[]byte("encrypted"),0600);err!=nil{t.Fatal(err)}
	if err:=provider.Put(context.Background(),"ptr_1/bkp_1.hmbk",source);err!=nil{t.Fatal(err)}
	reader,err:=provider.Open(context.Background(),"ptr_1/bkp_1.hmbk");if err!=nil{t.Fatal(err)}
	got:=new(bytes.Buffer);_,_=got.ReadFrom(reader);reader.Close()
	if got.String()!="encrypted"{t.Fatalf("unexpected offsite content %q",got.String())}
	if _,err:=provider.Open(context.Background(),"../escape");err==nil{t.Fatal("traversal key accepted")}
}

func TestSanitizeConfigRemovesSecretsRecursively(t *testing.T) {
	input:=map[string]any{
		"partner_id":"ptr_1",
		"token":"secret",
		"nested":map[string]any{"api_key":"hidden","safe":"visible"},
	}
	out:=sanitizeConfig(input).(map[string]any)
	if _,ok:=out["token"];ok{t.Fatal("token remained in config snapshot")}
	nested:=out["nested"].(map[string]any)
	if _,ok:=nested["api_key"];ok{t.Fatal("nested API key remained in config snapshot")}
	if nested["safe"]!="visible"{t.Fatal("safe config value was removed")}
}

func TestS3SigningProducesScopedAuthorization(t *testing.T) {
	endpoint,err:=validateS3Endpoint("https://objects.example.test")
	if err!=nil{t.Fatal(err)}
	p:=&s3Offsite{
		endpoint:endpoint,bucket:"himate-backups",region:"us-east-1",
		accessKey:"AKIDEXAMPLE",secretKey:"secret",client:nil,
	}
	empty:="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	req,err:=p.signedRequest(context.Background(),"GET","ptr_1/bkp_1.hmbk",empty,nil)
	if err!=nil{t.Fatal(err)}
	if req.URL.Scheme!="https"||!strings.Contains(req.URL.Path,"himate-backups/ptr_1/bkp_1.hmbk"){
		t.Fatalf("unexpected object URL %s",req.URL.String())
	}
	auth:=req.Header.Get("Authorization")
	if !strings.HasPrefix(auth,"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/")||!strings.Contains(auth,"/us-east-1/s3/aws4_request"){
		t.Fatalf("unexpected SigV4 authorization %q",auth)
	}
	if req.Header.Get("x-amz-content-sha256")!=empty{t.Fatal("payload hash missing")}
}
