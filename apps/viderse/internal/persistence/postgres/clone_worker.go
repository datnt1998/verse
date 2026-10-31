package postgres

import (
 "context"
 "errors"
 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/datnt1998/verse/apps/viderse/internal/pipeline"
)

// Two-key namespace avoids collision with media/storage advisory locks.
const cloneWorkerNamespace int32 = 0x56494445
const cloneWorkerKey int32 = 1

type cloneWorkerLock struct{conn *pgxpool.Conn}
func (s *CloneStore) AcquireWorkerLock(ctx context.Context)(pipeline.WorkerLock,error){
 conn,err:=s.pool.Acquire(ctx);if err!=nil{return nil,err}
 var held bool
 if err:=conn.QueryRow(ctx,"SELECT pg_try_advisory_lock($1,$2)",cloneWorkerNamespace,cloneWorkerKey).Scan(&held);err!=nil{conn.Release();return nil,err}
 if !held{conn.Release();return nil,nil}
 return &cloneWorkerLock{conn:conn},nil
}
func(l *cloneWorkerLock)Check(ctx context.Context)error{if l.conn==nil{return errors.New("worker lock closed")};return l.conn.Ping(ctx)}
func(l *cloneWorkerLock)Close(ctx context.Context)error{if l.conn==nil{return nil};defer l.conn.Release();_,err:=l.conn.Exec(ctx,"SELECT pg_advisory_unlock($1,$2)",cloneWorkerNamespace,cloneWorkerKey);l.conn=nil;return err}
