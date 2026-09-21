package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"himate.local/services/internal/partnerdb"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	artifactMagic="HMBK0001"
	encryptionChunkSize=1<<20
)

func (a *app) internalJSON(ctx context.Context,method,host,path string,payload any,dst any,optional bool)error{
	if strings.TrimSpace(host)==""{return fmt.Errorf("required private service host is not configured")}
	var body io.Reader
	if payload!=nil{raw,err:=json.Marshal(payload);if err!=nil{return err};body=bytes.NewReader(raw)}
	req,err:=http.NewRequestWithContext(ctx,method,"http://"+host+path,body);if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.internalToken)
	req.Header.Set("X-Himate-User-ID","service:backups")
	if payload!=nil{req.Header.Set("Content-Type","application/json")}
	resp,err:=a.client.Do(req);if err!=nil{return err}
	defer resp.Body.Close()
	if optional&&resp.StatusCode==http.StatusNotFound{return nil}
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("private service returned status %d",resp.StatusCode)}
	if dst!=nil{return json.NewDecoder(resp.Body).Decode(dst)}
	return nil
}

func (a *app) fetchToFile(ctx context.Context,host,path,target string)error{
	if strings.TrimSpace(host)==""{return fmt.Errorf("storage service host is not configured")}
	req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"http://"+host+path,nil);if err!=nil{return err}
	req.Header.Set("X-Himate-Internal-Token",a.internalToken)
	req.Header.Set("X-Himate-User-ID","service:backups")
	resp,err:=a.client.Do(req);if err!=nil{return err}
	defer resp.Body.Close()
	if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("media archive returned status %d",resp.StatusCode)}
	f,err:=os.OpenFile(target,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if err!=nil{return err}
	_,copyErr:=io.Copy(f,resp.Body);closeErr:=f.Close()
	if copyErr!=nil{return copyErr};return closeErr
}

func sanitizeConfig(value any)any{
	switch v:=value.(type){
	case map[string]any:
		out:=map[string]any{}
		for key,item:=range v{
			k:=strings.ToLower(key)
			if strings.Contains(k,"password")||strings.Contains(k,"secret")||strings.Contains(k,"token")||strings.Contains(k,"credential")||strings.Contains(k,"api_key"){continue}
			out[key]=sanitizeConfig(item)
		}
		return out
	case []any:
		out:=make([]any,len(v));for i,item:=range v{out[i]=sanitizeConfig(item)};return out
	default:return value
	}
}

func partnerItems(raw map[string]any,partnerID string)[]any{
	items,ok:=raw["items"].([]any);if !ok{return []any{}}
	out:=[]any{}
	for _,item:=range items{
		m,ok:=item.(map[string]any);if !ok{continue}
		if strings.TrimSpace(fmt.Sprint(m["partner_id"]))==partnerID{out=append(out,m)}
	}
	return out
}

func (a *app) captureConfig(ctx context.Context,partnerID string)(map[string]any,error){
	var partner map[string]any
	if err:=a.internalJSON(ctx,http.MethodGet,a.partnersHost,"/api/v1/partners/"+url.PathEscape(partnerID),nil,&partner,false);err!=nil{return nil,fmt.Errorf("partner config: %w",err)}
	var environments map[string]any
	if err:=a.internalJSON(ctx,http.MethodGet,a.envHost,"/api/v1/environments?partner_id="+url.QueryEscape(partnerID),nil,&environments,false);err!=nil{return nil,fmt.Errorf("environment config: %w",err)}
	desired:=map[string]any{}
	for _,environment:=range []string{"STAGING","PRODUCTION"}{
		var state map[string]any
		err:=a.internalJSON(ctx,http.MethodGet,a.connectorHost,"/api/v1/connectors/"+url.PathEscape(partnerID)+"/desired-state?environment="+environment,nil,&state,true)
		if err!=nil{return nil,fmt.Errorf("connector desired state: %w",err)}
		if len(state)>0{desired[environment]=state}
	}
	value:=map[string]any{
		"schema_version":1,"partner_id":partnerID,"partner":partner,
		"environments":partnerItems(environments,partnerID),
		"connector_desired_state":desired,"captured_at":time.Now().UTC(),
	}
	return sanitizeConfig(value).(map[string]any),nil
}

func shaFile(path string)(string,int64,error){
	f,err:=os.Open(path);if err!=nil{return "",0,err};defer f.Close()
	h:=sha256.New();n,err:=io.Copy(h,f);if err!=nil{return "",0,err}
	return hex.EncodeToString(h.Sum(nil)),n,nil
}

func (a *app) pgEnvironment(database string)([]string,error){
	u,err:=url.Parse(a.dbAdminURL);if err!=nil{return nil,err}
	if u.Scheme==""||u.Hostname()==""{return nil,fmt.Errorf("invalid partner database admin URL")}
	user:="";password:=""
	if u.User!=nil{user=u.User.Username();password,_=u.User.Password()}
	port:=u.Port();if port==""{port="5432"}
	sslmode:=u.Query().Get("sslmode");if sslmode==""{sslmode="require"}
	env:=append([]string{},os.Environ()...)
	env=append(env,"PGHOST="+u.Hostname(),"PGPORT="+port,"PGUSER="+user,"PGPASSWORD="+password,
		"PGDATABASE="+database,"PGSSLMODE="+sslmode,"PGCONNECT_TIMEOUT=10")
	return env,nil
}

func runCommand(ctx context.Context,env []string,name string,args ...string)error{
	cmd:=exec.CommandContext(ctx,name,args...);cmd.Env=env
	var stderr bytes.Buffer;cmd.Stderr=io.LimitWriter(&stderr,8192)
	if err:=cmd.Run();err!=nil{
		msg:=strings.TrimSpace(stderr.String());if msg!=""{return fmt.Errorf("%s failed: %s",name,msg)}
		return fmt.Errorf("%s failed",name)
	}
	return nil
}

func (a *app) dumpDatabase(ctx context.Context,partnerID,target string)error{
	dbName:=partnerdb.DatabaseName(partnerID);if dbName==""{return fmt.Errorf("invalid partner id")}
	env,err:=a.pgEnvironment(dbName);if err!=nil{return err}
	return runCommand(ctx,env,"pg_dump","--format=custom","--no-owner","--no-privileges","--file",target)
}

func writeJSONFile(path string,value any)error{
	raw,err:=json.MarshalIndent(value,"","  ");if err!=nil{return err};return os.WriteFile(path,raw,0600)
}

func createTarGz(target,root string,files []string)error{
	out,err:=os.OpenFile(target,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if err!=nil{return err}
	gz:=gzip.NewWriter(out);tw:=tar.NewWriter(gz)
	closeAll:=func()error{
		if err:=tw.Close();err!=nil{_ = gz.Close();_ = out.Close();return err}
		if err:=gz.Close();err!=nil{_ = out.Close();return err};return out.Close()
	}
	for _,name:=range files{
		source:=filepath.Join(root,name);info,err:=os.Stat(source)
		if err!=nil{_ = closeAll();return err}
		if !info.Mode().IsRegular(){_ = closeAll();return fmt.Errorf("backup component %s is not regular",name)}
		header:=&tar.Header{Name:filepath.ToSlash(name),Mode:0600,Size:info.Size(),ModTime:info.ModTime().UTC(),Typeflag:tar.TypeReg}
		if err:=tw.WriteHeader(header);err!=nil{_ = closeAll();return err}
		f,err:=os.Open(source);if err!=nil{_ = closeAll();return err}
		_,copyErr:=io.Copy(tw,f);_ = f.Close()
		if copyErr!=nil{_ = closeAll();return copyErr}
	}
	return closeAll()
}

func nonceFor(base []byte,counter uint64)[]byte{
	nonce:=append([]byte(nil),base...)
	value:=binary.BigEndian.Uint64(nonce[len(nonce)-8:])
	binary.BigEndian.PutUint64(nonce[len(nonce)-8:],value+counter)
	return nonce
}

func encryptFile(source,target string,key []byte)(string,int64,error){
	block,err:=aes.NewCipher(key);if err!=nil{return "",0,err}
	gcm,err:=cipher.NewGCM(block);if err!=nil{return "",0,err}
	in,err:=os.Open(source);if err!=nil{return "",0,err};defer in.Close()
	out,err:=os.OpenFile(target,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if err!=nil{return "",0,err}
	baseNonce:=make([]byte,gcm.NonceSize());if _,err=rand.Read(baseNonce);err!=nil{_ = out.Close();return "",0,err}
	h:=sha256.New();writer:=io.MultiWriter(out,h)
	if _,err=writer.Write([]byte(artifactMagic));err!=nil{_ = out.Close();return "",0,err}
	if _,err=writer.Write(baseNonce);err!=nil{_ = out.Close();return "",0,err}
	buf:=make([]byte,encryptionChunkSize);counter:=uint64(0)
	for{
		n,readErr:=in.Read(buf)
		if n>0{
			if counter==^uint64(0){_ = out.Close();return "",0,fmt.Errorf("backup artifact nonce space exhausted")}
			sealed:=gcm.Seal(nil,nonceFor(baseNonce,counter),buf[:n],nil)
			var length [4]byte;binary.BigEndian.PutUint32(length[:],uint32(n))
			if _,err=writer.Write(length[:]);err!=nil{_ = out.Close();return "",0,err}
			if _,err=writer.Write(sealed);err!=nil{_ = out.Close();return "",0,err};counter++
		}
		if errors.Is(readErr,io.EOF){break};if readErr!=nil{_ = out.Close();return "",0,readErr}
	}
	if err:=out.Sync();err!=nil{_ = out.Close();return "",0,err}
	info,err:=out.Stat();if err!=nil{_ = out.Close();return "",0,err}
	if err:=out.Close();err!=nil{return "",0,err}
	return hex.EncodeToString(h.Sum(nil)),info.Size(),nil
}

func decryptStreamToFile(reader io.Reader,target string,key []byte)error{
	block,err:=aes.NewCipher(key);if err!=nil{return err};gcm,err:=cipher.NewGCM(block);if err!=nil{return err}
	magic:=make([]byte,len(artifactMagic));if _,err=io.ReadFull(reader,magic);err!=nil{return err}
	if string(magic)!=artifactMagic{return fmt.Errorf("invalid backup artifact magic")}
	baseNonce:=make([]byte,gcm.NonceSize());if _,err=io.ReadFull(reader,baseNonce);err!=nil{return err}
	out,err:=os.OpenFile(target,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if err!=nil{return err};defer out.Close()
	counter:=uint64(0)
	for{
		var length [4]byte;_,err:=io.ReadFull(reader,length[:])
		if errors.Is(err,io.EOF){break};if err!=nil{return err}
		n:=binary.BigEndian.Uint32(length[:]);if n==0||n>encryptionChunkSize{return fmt.Errorf("invalid encrypted chunk length")}
		sealed:=make([]byte,int(n)+gcm.Overhead());if _,err=io.ReadFull(reader,sealed);err!=nil{return err}
		plain,err:=gcm.Open(nil,nonceFor(baseNonce,counter),sealed,nil);if err!=nil{return fmt.Errorf("backup artifact authentication failed")}
		if _,err=out.Write(plain);err!=nil{return err};counter++
	}
	return out.Sync()
}

func extractTarGz(source,target string)error{
	f,err:=os.Open(source);if err!=nil{return err};defer f.Close()
	gz,err:=gzip.NewReader(f);if err!=nil{return err};defer gz.Close()
	tr:=tar.NewReader(gz);root:=filepath.Clean(target)
	for{
		header,err:=tr.Next();if errors.Is(err,io.EOF){break};if err!=nil{return err}
		if header.Typeflag!=tar.TypeReg{return fmt.Errorf("unsupported archive entry type")}
		name:=filepath.Clean(filepath.FromSlash(header.Name))
		if name=="."||filepath.IsAbs(name)||strings.HasPrefix(name,".."+string(os.PathSeparator)){return fmt.Errorf("unsafe archive entry")}
		dest:=filepath.Clean(filepath.Join(root,name))
		if !strings.HasPrefix(dest,root+string(os.PathSeparator)){return fmt.Errorf("archive entry escapes target")}
		if err:=os.MkdirAll(filepath.Dir(dest),0700);err!=nil{return err}
		out,err:=os.OpenFile(dest,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600);if err!=nil{return err}
		_,copyErr:=io.Copy(out,tr);closeErr:=out.Close()
		if copyErr!=nil{return copyErr};if closeErr!=nil{return closeErr}
	}
	return nil
}

func (a *app) createRestorePoint(ctx context.Context,p restorePoint)error{
	work,err:=os.MkdirTemp(a.workRoot,"backup-"+p.ID+"-");if err!=nil{return err};defer os.RemoveAll(work)
	dbPath:=filepath.Join(work,"database.dump");mediaPath:=filepath.Join(work,"media.tar.gz")
	configPath:=filepath.Join(work,"config.json");manifestPath:=filepath.Join(work,"manifest.json")
	archivePath:=filepath.Join(work,"restore.tar.gz");encryptedPath:=filepath.Join(work,"restore.hmbk")

	if err:=a.dumpDatabase(ctx,p.PartnerID,dbPath);err!=nil{return fmt.Errorf("database backup: %w",err)}
	if err:=a.fetchToFile(ctx,a.storageHost,"/internal/v1/storage/partners/"+url.PathEscape(p.PartnerID)+"/archive",mediaPath);err!=nil{return fmt.Errorf("media backup: %w",err)}
	config,err:=a.captureConfig(ctx,p.PartnerID);if err!=nil{return fmt.Errorf("configuration backup: %w",err)}
	if err:=writeJSONFile(configPath,config);err!=nil{return err}

	dbSHA,dbBytes,err:=shaFile(dbPath);if err!=nil{return err}
	mediaSHA,mediaBytes,err:=shaFile(mediaPath);if err!=nil{return err}
	configSHA,configBytes,err:=shaFile(configPath);if err!=nil{return err}
	manifest:=map[string]any{
		"format":"HIMATE_RESTORE_POINT","version":1,"restore_point_id":p.ID,"partner_id":p.PartnerID,"created_at":time.Now().UTC(),
		"components":map[string]any{
			"database":map[string]any{"path":"database.dump","sha256":dbSHA,"bytes":dbBytes,"format":"postgres-custom"},
			"media":map[string]any{"path":"media.tar.gz","sha256":mediaSHA,"bytes":mediaBytes,"format":"tar-gzip"},
			"config":map[string]any{"path":"config.json","sha256":configSHA,"bytes":configBytes,"format":"json"},
		},
	}
	if err:=writeJSONFile(manifestPath,manifest);err!=nil{return err}
	if err:=createTarGz(archivePath,work,[]string{"database.dump","media.tar.gz","config.json","manifest.json"});err!=nil{return err}
	cipherSHA,cipherBytes,err:=encryptFile(archivePath,encryptedPath,a.key);if err!=nil{return fmt.Errorf("encrypt backup: %w",err)}
	objectKey:=p.PartnerID+"/"+p.ID+".hmbk"
	if err:=a.provider.Put(ctx,objectKey,encryptedPath);err!=nil{return fmt.Errorf("offsite replication: %w",err)}
	manifestRaw,_:=json.Marshal(manifest)
	_,err=a.db.Exec(`UPDATE backups.restore_points SET status='READY',provider=$2,object_key=$3,ciphertext_sha256=$4,ciphertext_bytes=$5,manifest=$6::jsonb,error='',completed_at=NOW() WHERE id=$1`,
		p.ID,a.provider.Name(),objectKey,cipherSHA,cipherBytes,string(manifestRaw))
	return err
}

func quoteIdent(value string)string{return \`"\`+strings.ReplaceAll(value,\`"\`,\`""\`)+\`"\`}

func (a *app) createScratchDatabase(ctx context.Context,name string)error{
	adminDSN,err:=partnerdb.AdminDSN(a.dbAdminURL);if err!=nil{return err}
	db,err:=sql.Open("pgx",adminDSN);if err!=nil{return err};defer db.Close()
	_,err=db.ExecContext(ctx,"CREATE DATABASE "+quoteIdent(name));return err
}

func (a *app) dropScratchDatabase(ctx context.Context,name string){
	adminDSN,err:=partnerdb.AdminDSN(a.dbAdminURL);if err!=nil{return}
	db,err:=sql.Open("pgx",adminDSN);if err!=nil{return};defer db.Close()
	_,_=db.ExecContext(ctx,`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=$1 AND pid<>pg_backend_pid()`,name)
	_,_=db.ExecContext(ctx,"DROP DATABASE IF EXISTS "+quoteIdent(name))
}

func (a *app) restoreDatabase(ctx context.Context,dumpPath,partnerID,testID string)error{
	suffix:=strings.ReplaceAll(strings.TrimPrefix(testID,"rst_"),"-","");if len(suffix)>12{suffix=suffix[:12]}
	name:="himate_restore_"+suffix
	if err:=a.createScratchDatabase(ctx,name);err!=nil{return fmt.Errorf("create scratch database: %w",err)}
	defer a.dropScratchDatabase(context.Background(),name)
	env,err:=a.pgEnvironment(name);if err!=nil{return err}
	if err:=runCommand(ctx,env,"pg_restore","--no-owner","--no-privileges","--exit-on-error","--dbname",name,dumpPath);err!=nil{return err}
	u,err:=url.Parse(a.dbAdminURL);if err!=nil{return err};u.Path="/"+name
	db,err:=sql.Open("pgx",u.String());if err!=nil{return err};defer db.Close()
	var restoredPartner string
	if err:=db.QueryRowContext(ctx,`SELECT value FROM partner_core.system_meta WHERE key='partner_id'`).Scan(&restoredPartner);err!=nil{return fmt.Errorf("verify restored partner identity: %w",err)}
	if restoredPartner!=partnerID{return fmt.Errorf("restored partner identity mismatch")};return nil
}

func verifyMediaArchive(source,target string)(int,error){
	if err:=os.MkdirAll(target,0700);err!=nil{return 0,err}
	if err:=extractTarGz(source,target);err!=nil{return 0,err}
	count:=0
	err:=filepath.WalkDir(target,func(path string,entry os.DirEntry,walkErr error)error{
		if walkErr!=nil{return walkErr}
		if entry.Type()&os.ModeSymlink!=0{return fmt.Errorf("media restore contains symlink")}
		if entry.Type().IsRegular(){count++};return nil
	})
	return count,err
}

func componentSHA(manifest map[string]any,component string)(string,error){
	components,ok:=manifest["components"].(map[string]any);if !ok{return "",fmt.Errorf("manifest components missing")}
	entry,ok:=components[component].(map[string]any);if !ok{return "",fmt.Errorf("manifest component %s missing",component)}
	value:=strings.TrimSpace(fmt.Sprint(entry["sha256"]));if value==""{return "",fmt.Errorf("manifest checksum missing")};return value,nil
}

func (a *app) restoreFromOffsite(ctx context.Context,t restoreTest)(bool,bool,bool,error){
	point,err:=a.getRestorePoint(t.RestorePointID);if err!=nil{return false,false,false,err}
	if point.Status!="READY"{return false,false,false,fmt.Errorf("restore point is not ready")}
	work,err:=os.MkdirTemp(a.workRoot,"restore-"+t.ID+"-");if err!=nil{return false,false,false,err};defer os.RemoveAll(work)
	encrypted:=filepath.Join(work,"offsite.hmbk")
	reader,err:=a.provider.Open(ctx,point.ObjectKey);if err!=nil{return false,false,false,fmt.Errorf("open offsite restore point: %w",err)}
	out,err:=os.OpenFile(encrypted,os.O_CREATE|os.O_TRUNC|os.O_WRONLY,0600)
	if err!=nil{reader.Close();return false,false,false,err}
	_,copyErr:=io.Copy(out,reader);reader.Close();closeErr:=out.Close()
	if copyErr!=nil{return false,false,false,copyErr};if closeErr!=nil{return false,false,false,closeErr}
	cipherSHA,_,err:=shaFile(encrypted);if err!=nil{return false,false,false,err}
	if !hmac.Equal([]byte(strings.ToLower(cipherSHA)),[]byte(strings.ToLower(point.CiphertextSHA256))){return false,false,false,fmt.Errorf("offsite ciphertext checksum mismatch")}

	archive:=filepath.Join(work,"restore.tar.gz");encReader,err:=os.Open(encrypted);if err!=nil{return false,false,false,err}
	err=decryptStreamToFile(encReader,archive,a.key);encReader.Close();if err!=nil{return false,false,false,err}
	extracted:=filepath.Join(work,"components");if err:=os.MkdirAll(extracted,0700);err!=nil{return false,false,false,err}
	if err:=extractTarGz(archive,extracted);err!=nil{return false,false,false,fmt.Errorf("extract restore point: %w",err)}
	manifestRaw,err:=os.ReadFile(filepath.Join(extracted,"manifest.json"));if err!=nil{return false,false,false,err}
	var manifest map[string]any;if err:=json.Unmarshal(manifestRaw,&manifest);err!=nil{return false,false,false,err}
	if fmt.Sprint(manifest["partner_id"])!=point.PartnerID{return false,false,false,fmt.Errorf("restore manifest partner mismatch")}
	for _,component:=range []string{"database","media","config"}{
		want,err:=componentSHA(manifest,component);if err!=nil{return false,false,false,err}
		path:=map[string]string{"database":"database.dump","media":"media.tar.gz","config":"config.json"}[component]
		got,_,err:=shaFile(filepath.Join(extracted,path));if err!=nil{return false,false,false,err}
		if !hmac.Equal([]byte(strings.ToLower(got)),[]byte(strings.ToLower(want))){return false,false,false,fmt.Errorf("%s component checksum mismatch",component)}
	}

	dbOK,mediaOK,configOK:=false,false,false
	if err:=a.restoreDatabase(ctx,filepath.Join(extracted,"database.dump"),point.PartnerID,t.ID);err!=nil{return dbOK,mediaOK,configOK,fmt.Errorf("database restore test: %w",err)}
	dbOK=true
	if _,err:=verifyMediaArchive(filepath.Join(extracted,"media.tar.gz"),filepath.Join(work,"media-restored"));err!=nil{return dbOK,mediaOK,configOK,fmt.Errorf("media restore test: %w",err)}
	mediaOK=true
	configRaw,err:=os.ReadFile(filepath.Join(extracted,"config.json"));if err!=nil{return dbOK,mediaOK,configOK,err}
	var config map[string]any;if err:=json.Unmarshal(configRaw,&config);err!=nil{return dbOK,mediaOK,configOK,fmt.Errorf("config restore test: %w",err)}
	if fmt.Sprint(config["partner_id"])!=point.PartnerID{return dbOK,mediaOK,configOK,fmt.Errorf("restored configuration partner mismatch")}
	configOK=true
	return dbOK,mediaOK,configOK,nil
}
