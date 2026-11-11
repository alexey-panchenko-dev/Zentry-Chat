package chat

import (
 "context"
 "errors"
 "github.com/jackc/pgx/v5"
)

// A private single-member conversation, always resolved from the signed-in user.
func (s *Store) Notes(ctx context.Context, user string) (Group,error) {
 tx,err:=s.db.Begin(ctx);if err!=nil{return Group{},err};defer tx.Rollback(context.Background())
 if _,err=tx.Exec(ctx,`SELECT pg_advisory_xact_lock(hashtextextended($1,302))`,user);err!=nil{return Group{},err}
 var id string
 err=tx.QueryRow(ctx,`SELECT group_id::text FROM private_note_chats WHERE user_id=$1`,user).Scan(&id)
 if errors.Is(err,pgx.ErrNoRows){
  err=tx.QueryRow(ctx,`INSERT INTO group_chats(name,owner_id) VALUES('Мои заметки',$1) RETURNING id::text`,user).Scan(&id);if err!=nil{return Group{},err}
  if _,err=tx.Exec(ctx,`INSERT INTO group_members(group_id,user_id) VALUES($1,$2)`,id,user);err!=nil{return Group{},err}
  if _,err=tx.Exec(ctx,`INSERT INTO private_note_chats(user_id,group_id) VALUES($1,$2)`,user,id);err!=nil{return Group{},err}
 } else if err!=nil{return Group{},err}
 if err=tx.Commit(ctx);err!=nil{return Group{},err}
 return s.Group(ctx,id,user)
}
