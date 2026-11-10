package chat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func(s *Store)SendGroupFile(ctx context.Context,groupID,userID,clientID,caption,name,mimeType string,data []byte)(GroupMessage,bool,[]string,error){
	tx,err:=s.db.Begin(ctx);if err!=nil{return GroupMessage{},false,nil,err};defer tx.Rollback(context.Background())
	var member bool;err=tx.QueryRow(ctx,`SELECT true FROM group_chats g JOIN group_members gm ON gm.group_id=g.id WHERE g.id=$1 AND gm.user_id=$2 FOR UPDATE OF g`,groupID,userID).Scan(&member);if errors.Is(err,pgx.ErrNoRows){return GroupMessage{},false,nil,ErrNotFound};if err!=nil{return GroupMessage{},false,nil,err}
	var blocked bool;err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM contact_blocks b JOIN group_members gm ON gm.group_id=$1 AND gm.user_id=CASE WHEN b.blocker_id=$2 THEN b.blocked_id ELSE b.blocker_id END WHERE b.blocker_id=$2 OR b.blocked_id=$2)`,groupID,userID).Scan(&blocked);if err!=nil{return GroupMessage{},false,nil,err};if blocked{return GroupMessage{},false,nil,ErrBlocked}
	var m GroupMessage;err=tx.QueryRow(ctx,`INSERT INTO group_messages(group_id,sender_id,client_message_id,body,kind,attachment,attachment_mime,attachment_name) VALUES($1,$2,$3,$4,'file',$5,$6,$7) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at,kind`,groupID,userID,clientID,caption,data,mimeType,name).Scan(&m.ID,&m.GroupID,&m.SenderID,&m.ClientMessageID,&m.Body,&m.CreatedAt,&m.Kind);created:=err==nil
	if errors.Is(err,pgx.ErrNoRows){var original []byte;err=tx.QueryRow(ctx,`SELECT id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at,kind,attachment FROM group_messages WHERE sender_id=$1 AND client_message_id=$2`,userID,clientID).Scan(&m.ID,&m.GroupID,&m.SenderID,&m.ClientMessageID,&m.Body,&m.CreatedAt,&m.Kind,&original);if err==nil&&(m.GroupID!=groupID||m.Body!=caption||m.Kind!="file"||!bytes.Equal(original,data)){return GroupMessage{},false,nil,ErrConflict}}
	if err!=nil{return m,false,nil,err};if err=tx.QueryRow(ctx,`SELECT name FROM users WHERE id=$1`,userID).Scan(&m.SenderName);err!=nil{return m,false,nil,err};m.AttachmentName=name;m.AttachmentMIME=mimeType;m.AttachmentURL="/api/groups/"+groupID+"/messages/"+m.ID+"/file"
	if _,err=tx.Exec(ctx,`UPDATE group_chats SET updated_at=now() WHERE id=$1`,groupID);err!=nil{return m,false,nil,err};rows,err:=tx.Query(ctx,`SELECT user_id::text FROM group_members WHERE group_id=$1`,groupID);if err!=nil{return m,false,nil,err};members:=[]string{};for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return m,false,nil,err};members=append(members,id)};rows.Close();if err=rows.Err();err!=nil{return m,false,nil,err};return m,created,members,tx.Commit(ctx)
}

func(s *Store)GroupFile(ctx context.Context,groupID,messageID,userID string)([]byte,string,string,error){
	var data []byte;var name,mimeType string;err:=s.db.QueryRow(ctx,`SELECT m.attachment,m.attachment_name,m.attachment_mime FROM group_messages m JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$3 WHERE m.group_id=$1 AND m.id=$2 AND m.kind='file'`,groupID,messageID,userID).Scan(&data,&name,&mimeType);if errors.Is(err,pgx.ErrNoRows){return nil,"","",ErrNotFound};return data,name,mimeType,err
}

func (s *Store) SetGroupReaction(ctx context.Context, messageID, userID, emoji string) ([]ReactionCount, error) {
	var visible bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM group_messages m JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$2 WHERE m.id=$1)`, messageID, userID).Scan(&visible)
	if err != nil { return nil, err }
	if !visible { return nil, ErrNotFound }
	if emoji == "" {
		_, err = s.db.Exec(ctx, `DELETE FROM group_message_reactions WHERE message_id=$1 AND user_id=$2`, messageID, userID)
	} else {
		_, err = s.db.Exec(ctx, `INSERT INTO group_message_reactions(message_id,user_id,emoji) VALUES($1,$2,$3) ON CONFLICT(message_id,user_id) DO UPDATE SET emoji=EXCLUDED.emoji`, messageID, userID, emoji)
	}
	if err != nil { return nil, err }
	rows, err := s.db.Query(ctx, `SELECT r.emoji,count(*)::int,bool_or(r.user_id=$2) FROM group_message_reactions r JOIN group_messages m ON m.id=r.message_id JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$2 WHERE r.message_id=$1 GROUP BY r.emoji ORDER BY r.emoji`, messageID, userID)
	if err != nil { return nil, err }
	defer rows.Close()
	out := []ReactionCount{}
	for rows.Next() { var v ReactionCount; if err=rows.Scan(&v.Emoji,&v.Count,&v.Mine);err!=nil{return nil,err};out=append(out,v) }
	return out, rows.Err()
}

func (s *Store) GroupReactions(ctx context.Context, messageID,userID string)([]ReactionCount,error){
	var visible bool
	if err:=s.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM group_messages m JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$2 WHERE m.id=$1)`,messageID,userID).Scan(&visible);err!=nil{return nil,err};if !visible{return nil,ErrNotFound}
	rows,err:=s.db.Query(ctx,`SELECT r.emoji,count(*)::int,bool_or(r.user_id=$2) FROM group_message_reactions r JOIN group_messages m ON m.id=r.message_id JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$2 WHERE r.message_id=$1 GROUP BY r.emoji ORDER BY r.emoji`,messageID,userID);if err!=nil{return nil,err};defer rows.Close();out:=[]ReactionCount{};for rows.Next(){var v ReactionCount;if err=rows.Scan(&v.Emoji,&v.Count,&v.Mine);err!=nil{return nil,err};out=append(out,v)};return out,rows.Err()
}

func (s *Store) EditGroupMessage(ctx context.Context,messageID,userID,body string)(time.Time,error){
	var edited time.Time
	err:=s.db.QueryRow(ctx,`UPDATE group_messages m SET body=$3,edited_at=now() FROM group_members gm WHERE m.id=$1 AND m.sender_id=$2 AND m.kind='text' AND m.created_at>now()-interval '15 minutes' AND gm.group_id=m.group_id AND gm.user_id=$2 RETURNING m.edited_at`,messageID,userID,body).Scan(&edited)
	if errors.Is(err,pgx.ErrNoRows){return edited,ErrMessageUnavailable};return edited,err
}

func (s *Store) SendGroupReply(ctx context.Context,groupID,userID,clientID,body,replyID string)(GroupMessage,bool,[]string,error){
	tx,err:=s.db.Begin(ctx);if err!=nil{return GroupMessage{},false,nil,err};defer tx.Rollback(context.Background())
	var member bool;err=tx.QueryRow(ctx,`SELECT true FROM group_chats g JOIN group_members gm ON gm.group_id=g.id WHERE g.id=$1 AND gm.user_id=$2 FOR UPDATE OF g`,groupID,userID).Scan(&member);if errors.Is(err,pgx.ErrNoRows){return GroupMessage{},false,nil,ErrNotFound};if err!=nil{return GroupMessage{},false,nil,err}
	var blocked bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM contact_blocks b JOIN group_members gm ON gm.group_id=$1 AND gm.user_id=CASE WHEN b.blocker_id=$2 THEN b.blocked_id ELSE b.blocker_id END WHERE b.blocker_id=$2 OR b.blocked_id=$2)`,groupID,userID).Scan(&blocked);err!=nil{return GroupMessage{},false,nil,err};if blocked{return GroupMessage{},false,nil,ErrBlocked}
	var reply any;if replyID!=""{var id int64;if _,err=fmt.Sscan(replyID,&id);err!=nil||id<1{return GroupMessage{},false,nil,ErrNotFound};var found bool;if err=tx.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM group_messages WHERE id=$1 AND group_id=$2)`,id,groupID).Scan(&found);err!=nil{return GroupMessage{},false,nil,err};if !found{return GroupMessage{},false,nil,ErrNotFound};reply=id}
	var m GroupMessage;err=tx.QueryRow(ctx,`INSERT INTO group_messages(group_id,sender_id,client_message_id,body,reply_to_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(sender_id,client_message_id) DO NOTHING RETURNING id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at,kind,COALESCE(reply_to_id::text,'')`,groupID,userID,clientID,body,reply).Scan(&m.ID,&m.GroupID,&m.SenderID,&m.ClientMessageID,&m.Body,&m.CreatedAt,&m.Kind,&m.ReplyToID)
	created:=err==nil;if errors.Is(err,pgx.ErrNoRows){err=tx.QueryRow(ctx,`SELECT id::text,group_id::text,sender_id::text,client_message_id::text,body,created_at,kind,COALESCE(reply_to_id::text,'') FROM group_messages WHERE sender_id=$1 AND client_message_id=$2`,userID,clientID).Scan(&m.ID,&m.GroupID,&m.SenderID,&m.ClientMessageID,&m.Body,&m.CreatedAt,&m.Kind,&m.ReplyToID);if err==nil&&(m.GroupID!=groupID||m.Body!=body||m.ReplyToID!=replyID){return GroupMessage{},false,nil,ErrConflict}}
	if err!=nil{return m,false,nil,err};if err=tx.QueryRow(ctx,`SELECT name FROM users WHERE id=$1`,userID).Scan(&m.SenderName);err!=nil{return m,false,nil,err};if m.ReplyToID!=""{_ = tx.QueryRow(ctx,`SELECT r.body,u.name FROM group_messages r JOIN users u ON u.id=r.sender_id WHERE r.id=$1`,m.ReplyToID).Scan(&m.ReplyBody,&m.ReplySenderName)}
	if _,err=tx.Exec(ctx,`UPDATE group_chats SET updated_at=now() WHERE id=$1`,groupID);err!=nil{return m,false,nil,err}
	rows,err:=tx.Query(ctx,`SELECT user_id::text FROM group_members WHERE group_id=$1`,groupID);if err!=nil{return m,false,nil,err};members:=[]string{};for rows.Next(){var id string;if err=rows.Scan(&id);err!=nil{rows.Close();return m,false,nil,err};members=append(members,id)};rows.Close();if err=rows.Err();err!=nil{return m,false,nil,err};return m,created,members,tx.Commit(ctx)
}

func (s *Store) SetGroupPin(ctx context.Context,groupID,messageID,userID string,pinned bool)error{
	var owner bool;if err:=s.db.QueryRow(ctx,`SELECT EXISTS(SELECT 1 FROM group_chats g JOIN group_members gm ON gm.group_id=g.id AND gm.user_id=$2 WHERE g.id=$1 AND g.owner_id=$2)`,groupID,userID).Scan(&owner);err!=nil{return err};if !owner{return ErrForbidden}
	if pinned{var exists bool;err:=s.db.QueryRow(ctx,`WITH added AS (INSERT INTO pinned_group_messages(group_id,message_id,pinned_by) SELECT m.group_id,m.id,$3 FROM group_messages m JOIN group_members gm ON gm.group_id=m.group_id AND gm.user_id=$3 WHERE m.group_id=$1 AND m.id=$2 ON CONFLICT DO NOTHING RETURNING 1) SELECT EXISTS(SELECT 1 FROM added) OR EXISTS(SELECT 1 FROM pinned_group_messages WHERE group_id=$1 AND message_id=$2)`,groupID,messageID,userID).Scan(&exists);if err!=nil{return err};if !exists{return ErrNotFound};return nil}
	result,err:=s.db.Exec(ctx,`DELETE FROM pinned_group_messages p USING group_members gm WHERE p.group_id=$1 AND p.message_id=$2 AND gm.group_id=p.group_id AND gm.user_id=$3 AND EXISTS(SELECT 1 FROM group_chats g WHERE g.id=p.group_id AND g.owner_id=$3)`,groupID,messageID,userID);if err!=nil{return err};if result.RowsAffected()==0{return ErrNotFound};return nil
}

func (s *Store) GroupPins(ctx context.Context,groupID,userID string)([]GroupMessage,error){
	rows,err:=s.db.Query(ctx,`SELECT m.id::text,m.group_id::text,m.sender_id::text,u.name,m.client_message_id::text,m.body,m.created_at,m.kind,COALESCE(m.reply_to_id::text,''),COALESCE(r.body,''),m.edited_at FROM pinned_group_messages p JOIN group_members gm ON gm.group_id=p.group_id AND gm.user_id=$2 JOIN group_messages m ON m.group_id=p.group_id AND m.id=p.message_id JOIN users u ON u.id=m.sender_id LEFT JOIN group_messages r ON r.id=m.reply_to_id WHERE p.group_id=$1 ORDER BY p.created_at DESC LIMIT 100`,groupID,userID);if err!=nil{return nil,err};defer rows.Close();out:=[]GroupMessage{};for rows.Next(){var m GroupMessage;if err=rows.Scan(&m.ID,&m.GroupID,&m.SenderID,&m.SenderName,&m.ClientMessageID,&m.Body,&m.CreatedAt,&m.Kind,&m.ReplyToID,&m.ReplyBody,&m.EditedAt);err!=nil{return nil,err};out=append(out,m)};return out,rows.Err()
}

func (s *Store) SearchGroupMessages(ctx context.Context,groupID,userID,query,kind string,before int64,limit int)([]GroupMessage,error){
	query=strings.ToLower(query);rows,err:=s.db.Query(ctx,`SELECT m.id::text,m.group_id::text,m.sender_id::text,u.name,m.client_message_id::text,m.body,m.created_at,m.kind,COALESCE(m.reply_to_id::text,''),COALESCE(r.body,''),m.edited_at,COALESCE(m.attachment_name,''),COALESCE(m.attachment_mime,''),COALESCE(octet_length(m.attachment),0) FROM group_members gm JOIN group_messages m ON m.group_id=gm.group_id LEFT JOIN users u ON u.id=m.sender_id LEFT JOIN group_messages r ON r.id=m.reply_to_id WHERE gm.group_id=$1 AND gm.user_id=$2 AND ($3::bigint=0 OR m.id<$3) AND ($4='' OR strpos(lower(m.body),$4)>0 OR strpos(lower(COALESCE(m.attachment_name,'')),$4)>0) AND ($5='' OR ($5='messages' AND m.kind='text') OR ($5='photos' AND m.kind='image') OR ($5='videos' AND m.kind='file' AND m.attachment_mime LIKE 'video/%') OR ($5='files' AND m.kind='file') OR ($5='links' AND m.body LIKE '%http%')) ORDER BY m.id DESC LIMIT $6`,groupID,userID,before,query,kind,limit);if err!=nil{return nil,err};defer rows.Close();out:=[]GroupMessage{};for rows.Next(){var m GroupMessage;if err=rows.Scan(&m.ID,&m.GroupID,&m.SenderID,&m.SenderName,&m.ClientMessageID,&m.Body,&m.CreatedAt,&m.Kind,&m.ReplyToID,&m.ReplyBody,&m.EditedAt,&m.AttachmentName,&m.AttachmentMIME,&m.AttachmentSize);err!=nil{return nil,err};if m.Kind=="image"{m.ImageURL="/api/groups/"+groupID+"/messages/"+m.ID+"/image"};if m.Kind=="file"{m.AttachmentURL="/api/groups/"+groupID+"/messages/"+m.ID+"/file"};out=append(out,m)};return out,rows.Err()
}
