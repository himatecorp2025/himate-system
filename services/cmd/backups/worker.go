package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (a *app) worker(index int) {
	ticker:=time.NewTicker(2*time.Second);defer ticker.Stop()
	for{
		worked:=false
		ctx,cancel:=context.WithTimeout(context.Background(),45*time.Minute)
		if point,ok:=a.claimRestorePoint(ctx);ok{
			worked=true
			a.processBackup(ctx,point)
		}
		cancel()

		ctx,cancel=context.WithTimeout(context.Background(),45*time.Minute)
		if test,ok:=a.claimRestoreTest(ctx);ok{
			worked=true
			a.processRestoreTest(ctx,test)
		}
		cancel()

		if worked{continue}
		select{case<-a.wake:case<-ticker.C:}
	}
}

func (a *app) claimRestorePoint(ctx context.Context)(restorePoint,bool){
	tx,err:=a.db.BeginTx(ctx,&sql.TxOptions{});if err!=nil{return restorePoint{},false};defer tx.Rollback()
	point,err:=scanRestorePoint(tx.QueryRowContext(ctx,restorePointSelect+` WHERE status='QUEUED' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err!=nil{return restorePoint{},false}
	if _,err=tx.ExecContext(ctx,`UPDATE backups.restore_points SET status='RUNNING',error='' WHERE id=$1`,point.ID);err!=nil{return restorePoint{},false}
	if tx.Commit()!=nil{return restorePoint{},false};point.Status="RUNNING";return point,true
}

func (a *app) claimRestoreTest(ctx context.Context)(restoreTest,bool){
	tx,err:=a.db.BeginTx(ctx,&sql.TxOptions{});if err!=nil{return restoreTest{},false};defer tx.Rollback()
	test,err:=scanRestoreTest(tx.QueryRowContext(ctx,restoreTestSelect+` WHERE status='QUEUED' ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1`))
	if err!=nil{return restoreTest{},false}
	if _,err=tx.ExecContext(ctx,`UPDATE backups.restore_tests SET status='RUNNING',error='' WHERE id=$1`,test.ID);err!=nil{return restoreTest{},false}
	if tx.Commit()!=nil{return restoreTest{},false};test.Status="RUNNING";return test,true
}

func (a *app) processBackup(ctx context.Context,point restorePoint){
	if err:=a.createRestorePoint(ctx,point);err!=nil{
		_,_=a.db.Exec(`UPDATE backups.restore_points SET status='FAILED',error=$2,completed_at=NOW() WHERE id=$1`,point.ID,safeError(err))
		return
	}
	ready,err:=a.getRestorePoint(point.ID)
	if err==nil{
		_,_=a.queueRestoreTestRecord(ready,"automatic")
	}
	_,_=a.prunePartner(context.Background(),point.PartnerID)
}

func (a *app) processRestoreTest(ctx context.Context,test restoreTest){
	started:=time.Now()
	dbOK,mediaOK,configOK,err:=a.restoreFromOffsite(ctx,test)
	status:="PASSED";message:=""
	if err!=nil{status="FAILED";message=safeError(err)}
	_,_=a.db.Exec(`UPDATE backups.restore_tests
		SET status=$2,database_ok=$3,media_ok=$4,config_ok=$5,error=$6,completed_at=NOW(),duration_ms=$7
		WHERE id=$1`,test.ID,status,dbOK,mediaOK,configOK,message,time.Since(started).Milliseconds())
}

func (a *app) queueRestoreTestRecord(point restorePoint,actor string)(restoreTest,error){
	if point.Status!="READY"{return restoreTest{},fmt.Errorf("restore point must be READY")}
	var pending bool
	if err:=a.db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM backups.restore_tests WHERE restore_point_id=$1 AND status IN('QUEUED','RUNNING')
	)`,point.ID).Scan(&pending);err!=nil{return restoreTest{},err}
	if pending{return restoreTest{},fmt.Errorf("restore test already queued or running")}
	id:=newID("rst")
	_,err:=a.db.Exec(`INSERT INTO backups.restore_tests(id,restore_point_id,partner_id,status,created_by)
		VALUES($1,$2,$3,'QUEUED',$4)`,id,point.ID,point.PartnerID,actor)
	if err!=nil{return restoreTest{},err}
	test,err:=scanRestoreTest(a.db.QueryRow(restoreTestSelect+` WHERE id=$1`,id))
	if err==nil{a.signal()};return test,err
}

func (a *app) scheduler(){
	ticker:=time.NewTicker(time.Minute);defer ticker.Stop()
	run:=func(){
		rows,err:=a.db.Query(`SELECT partner_id,schedule_hours FROM backups.policies WHERE enabled=TRUE`);if err!=nil{return}
		type due struct{id string;hours int};items:=[]due{}
		for rows.Next(){var item due;if rows.Scan(&item.id,&item.hours)==nil{items=append(items,item)}};rows.Close()
		now:=time.Now().UTC()
		for _,item:=range items{
			var latest sql.NullTime
			_ = a.db.QueryRow(`SELECT MAX(created_at) FROM backups.restore_points
				WHERE partner_id=$1 AND status IN('QUEUED','RUNNING','READY')`,item.id).Scan(&latest)
			if latest.Valid&&now.Sub(latest.Time)<time.Duration(item.hours)*time.Hour{continue}
			if _,err:=a.queueBackup(item.id,"scheduler");err==nil{
				_,_=a.db.Exec(`UPDATE backups.policies SET last_scheduled_at=NOW(),updated_at=NOW() WHERE partner_id=$1`,item.id)
			}
		}
	}
	run()
	for range ticker.C{run()}
}

func (a *app) prunePartner(ctx context.Context,partnerID string)(int,error){
	p,err:=a.ensurePolicy(partnerID);if err!=nil{return 0,err}
	rows,err:=a.db.Query(restorePointSelect+` WHERE partner_id=$1 AND status='READY' ORDER BY created_at DESC`,partnerID)
	if err!=nil{return 0,err}
	points:=[]restorePoint{}
	for rows.Next(){if point,scanErr:=scanRestorePoint(rows);scanErr==nil{points=append(points,point)}};rows.Close()

	expire:=map[string]restorePoint{};now:=time.Now().UTC()
	for index,point:=range points{
		if index>=p.MaxRestorePoints||(point.ExpiresAt.Valid&&point.ExpiresAt.Time.Before(now)){expire[point.ID]=point}
	}
	count:=0
	for _,point:=range expire{
		if point.ObjectKey!=""{
			if err:=a.provider.Delete(ctx,point.ObjectKey);err!=nil{return count,err}
		}
		if _,err:=a.db.Exec(`UPDATE backups.restore_points SET status='EXPIRED',error='',object_key='' WHERE id=$1`,point.ID);err!=nil{return count,err}
		count++
	}
	return count,nil
}
