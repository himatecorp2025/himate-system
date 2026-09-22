package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"himate.local/services/internal/common"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type offsiteProvider interface {
	Name() string
	Put(context.Context,string,string) error
	Open(context.Context,string)(io.ReadCloser,error)
	Delete(context.Context,string) error
}

type localOffsite struct{root,name string}

func (p *localOffsite)Name()string{if strings.TrimSpace(p.name)!=""{return p.name};return "local"}

func cleanObjectKey(key string)(string,error){
	key=strings.Trim(strings.TrimSpace(key),"/")
	if key==""||strings.Contains(key,"..")||strings.ContainsRune(key,'\\'){return "",fmt.Errorf("invalid backup object key")}
	for _,part:=range strings.Split(key,"/"){if part==""||part=="."||part==".."{return "",fmt.Errorf("invalid backup object key")}}
	return key,nil
}

func (p *localOffsite)path(key string)(string,error){
	key,err:=cleanObjectKey(key);if err!=nil{return "",err}
	root:=filepath.Clean(p.root);target:=filepath.Clean(filepath.Join(root,filepath.FromSlash(key)))
	if target==root||!strings.HasPrefix(target,root+string(os.PathSeparator)){return "",fmt.Errorf("backup object escapes offsite root")}
	return target,nil
}

func (p *localOffsite)Put(ctx context.Context,key,source string)error{
	select{case<-ctx.Done():return ctx.Err();default:}
	target,err:=p.path(key);if err!=nil{return err}
	if err:=os.MkdirAll(filepath.Dir(target),0700);err!=nil{return err}
	src,err:=os.Open(source);if err!=nil{return err};defer src.Close()
	tmp,err:=os.CreateTemp(filepath.Dir(target),".backup-*");if err!=nil{return err}
	tmpName:=tmp.Name();defer os.Remove(tmpName)
	if _,err=io.Copy(tmp,src);err!=nil{_ = tmp.Close();return err}
	if err=tmp.Sync();err!=nil{_ = tmp.Close();return err}
	if err=tmp.Close();err!=nil{return err}
	if err=os.Chmod(tmpName,0600);err!=nil{return err}
	return os.Rename(tmpName,target)
}

func (p *localOffsite)Open(ctx context.Context,key string)(io.ReadCloser,error){
	select{case<-ctx.Done():return nil,ctx.Err();default:}
	target,err:=p.path(key);if err!=nil{return nil,err};return os.Open(target)
}

func (p *localOffsite)Delete(ctx context.Context,key string)error{
	select{case<-ctx.Done():return ctx.Err();default:}
	target,err:=p.path(key);if err!=nil{return err}
	if err:=os.Remove(target);err!=nil&&!os.IsNotExist(err){return err};return nil
}

type s3Offsite struct{
	endpoint *url.URL
	bucket,region,accessKey,secretKey,sessionToken string
	client *http.Client
}

func (p *s3Offsite)Name()string{return "s3"}

func awsURLEscapePath(value string)string{
	parts:=strings.Split(strings.Trim(value,"/"),"/")
	for i,part:=range parts{parts[i]=url.PathEscape(part)}
	return "/"+strings.Join(parts,"/")
}

func hmacSHA256(key []byte,value string)[]byte{
	mac:=hmac.New(sha256.New,key);_,_=mac.Write([]byte(value));return mac.Sum(nil)
}

func (p *s3Offsite)objectURL(key string)(*url.URL,error){
	key,err:=cleanObjectKey(key);if err!=nil{return nil,err}
	u:=*p.endpoint
	base:=strings.TrimRight(u.EscapedPath(),"/")
	u.RawPath=base+awsURLEscapePath(p.bucket+"/"+key)
	decoded,err:=url.PathUnescape(u.RawPath);if err!=nil{return nil,err}
	u.Path=decoded;u.RawQuery=""
	return &u,nil
}

func fileSHA256(path string)(string,error){
	f,err:=os.Open(path);if err!=nil{return "",err};defer f.Close()
	h:=sha256.New();if _,err:=io.Copy(h,f);err!=nil{return "",err}
	return hex.EncodeToString(h.Sum(nil)),nil
}

func (p *s3Offsite)signedRequest(ctx context.Context,method,key,payloadHash string,body io.Reader)(*http.Request,error){
	u,err:=p.objectURL(key);if err!=nil{return nil,err}
	req,err:=http.NewRequestWithContext(ctx,method,u.String(),body);if err!=nil{return nil,err}
	now:=time.Now().UTC();amzDate:=now.Format("20060102T150405Z");shortDate:=now.Format("20060102")
	req.Header.Set("x-amz-date",amzDate);req.Header.Set("x-amz-content-sha256",payloadHash)
	if p.sessionToken!=""{req.Header.Set("x-amz-security-token",p.sessionToken)}

	headers:=[]string{"host","x-amz-content-sha256","x-amz-date"}
	if p.sessionToken!=""{headers=append(headers,"x-amz-security-token")}
	sort.Strings(headers)
	var canonicalHeaders strings.Builder
	for _,name:=range headers{
		value:=""
		if name=="host"{value=req.URL.Host}else{value=strings.Join(strings.Fields(req.Header.Get(name))," ")}
		canonicalHeaders.WriteString(name);canonicalHeaders.WriteByte(':');canonicalHeaders.WriteString(value);canonicalHeaders.WriteByte('\n')
	}
	signedHeaders:=strings.Join(headers,";")
	canonicalURI:=req.URL.EscapedPath();if canonicalURI==""{canonicalURI="/"}
	canonicalRequest:=method+"\n"+canonicalURI+"\n\n"+canonicalHeaders.String()+"\n"+signedHeaders+"\n"+payloadHash
	sum:=sha256.Sum256([]byte(canonicalRequest))
	scope:=shortDate+"/"+p.region+"/s3/aws4_request"
	stringToSign:="AWS4-HMAC-SHA256\n"+amzDate+"\n"+scope+"\n"+hex.EncodeToString(sum[:])
	dateKey:=hmacSHA256([]byte("AWS4"+p.secretKey),shortDate)
	regionKey:=hmacSHA256(dateKey,p.region)
	serviceKey:=hmacSHA256(regionKey,"s3")
	signingKey:=hmacSHA256(serviceKey,"aws4_request")
	signature:=hex.EncodeToString(hmacSHA256(signingKey,stringToSign))
	req.Header.Set("Authorization","AWS4-HMAC-SHA256 Credential="+p.accessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return req,nil
}

func (p *s3Offsite)Put(ctx context.Context,key,source string)error{
	hash,err:=fileSHA256(source);if err!=nil{return err}
	f,err:=os.Open(source);if err!=nil{return err};defer f.Close()
	info,err:=f.Stat();if err!=nil{return err}
	req,err:=p.signedRequest(ctx,http.MethodPut,key,hash,f);if err!=nil{return err}
	req.ContentLength=info.Size();req.Header.Set("Content-Type","application/octet-stream")
	resp,err:=p.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("offsite S3 PUT returned status %d",resp.StatusCode)}
	return nil
}

func (p *s3Offsite)Open(ctx context.Context,key string)(io.ReadCloser,error){
	empty:=sha256.Sum256(nil)
	req,err:=p.signedRequest(ctx,http.MethodGet,key,hex.EncodeToString(empty[:]),nil);if err!=nil{return nil,err}
	resp,err:=p.client.Do(req);if err!=nil{return nil,err}
	if resp.StatusCode<200||resp.StatusCode>=300{resp.Body.Close();return nil,fmt.Errorf("offsite S3 GET returned status %d",resp.StatusCode)}
	return resp.Body,nil
}

func (p *s3Offsite)Delete(ctx context.Context,key string)error{
	empty:=sha256.Sum256(nil)
	req,err:=p.signedRequest(ctx,http.MethodDelete,key,hex.EncodeToString(empty[:]),nil);if err!=nil{return err}
	resp,err:=p.client.Do(req);if err!=nil{return err};defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("offsite S3 DELETE returned status %d",resp.StatusCode)}
	return nil
}

func newOffsiteProvider(client *http.Client)(offsiteProvider,error){
	providerName:=strings.ToLower(strings.TrimSpace(common.Env("HIMATE_BACKUP_PROVIDER","local")))
	switch providerName{
	case "local","render_disk":
		root:=common.Env("HIMATE_BACKUP_OFFSITE_ROOT","/offsite")
		if !filepath.IsAbs(root)||filepath.Clean(root)=="/"{return nil,fmt.Errorf("HIMATE_BACKUP_OFFSITE_ROOT must be an absolute non-root path")}
		if err:=os.MkdirAll(root,0700);err!=nil{return nil,err}
		probe,err:=os.CreateTemp(root,".himate-backup-write-test-*");if err!=nil{return nil,fmt.Errorf("backup storage root is not writable: %w",err)}
		probeName:=probe.Name()
		if err:=probe.Chmod(0600);err!=nil{_ = probe.Close();_ = os.Remove(probeName);return nil,err}
		if err:=probe.Close();err!=nil{_ = os.Remove(probeName);return nil,err}
		if err:=os.Remove(probeName);err!=nil{return nil,err}
		return &localOffsite{root:root,name:providerName},nil
	case "s3":
		endpoint,err:=validateS3Endpoint(os.Getenv("HIMATE_BACKUP_S3_ENDPOINT"));if err!=nil{return nil,err}
		bucket:=strings.TrimSpace(os.Getenv("HIMATE_BACKUP_S3_BUCKET"))
		region:=strings.TrimSpace(common.Env("HIMATE_BACKUP_S3_REGION","us-east-1"))
		access:=strings.TrimSpace(os.Getenv("HIMATE_BACKUP_S3_ACCESS_KEY"))
		secret:=strings.TrimSpace(os.Getenv("HIMATE_BACKUP_S3_SECRET_KEY"))
		if bucket==""||region==""||access==""||secret==""{return nil,fmt.Errorf("S3 bucket, region and credentials are required")}
		return &s3Offsite{
			endpoint:endpoint,bucket:bucket,region:region,accessKey:access,secretKey:secret,
			sessionToken:strings.TrimSpace(os.Getenv("HIMATE_BACKUP_S3_SESSION_TOKEN")),client:client,
		},nil
	default:
		return nil,fmt.Errorf("unsupported HIMATE_BACKUP_PROVIDER")
	}
}
