package httpapi

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
)

func groupMessageID(w http.ResponseWriter,r *http.Request)(string,bool){
	n,err:=strconv.ParseInt(r.PathValue("messageID"),10,64);if err!=nil||n<1{writeJSON(w,404,map[string]string{"error":"not_found"});return "",false};return strconv.FormatInt(n,10),true
}
func (h *chatHandlers) groupReaction(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=groupMessageID(w,r);if !ok{return};var in struct{Emoji string `json:"emoji"`};if !readAuthJSON(w,r,&in){return};if in.Emoji!=""&&!allowedReaction[in.Emoji]{writeJSON(w,400,map[string]string{"error":"invalid_reaction"});return};v,err:=h.store.SetGroupReaction(r.Context(),id,u.ID,in.Emoji);if err!=nil{chatFailure(w,err);return};h.notifyGroupMessageFeature(r,u.ID);writeJSON(w,200,map[string]any{"reactions":v})
}
func (h *chatHandlers) listGroupReactions(w http.ResponseWriter,r *http.Request,u auth.User){id,ok:=groupMessageID(w,r);if !ok{return};v,err:=h.store.GroupReactions(r.Context(),id,u.ID);if err!=nil{chatFailure(w,err);return};writeJSON(w,200,map[string]any{"reactions":v})}
func (h *chatHandlers) editGroupMessage(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=groupMessageID(w,r);if !ok{return};var in struct{Body string `json:"body"`};if !readAuthJSON(w,r,&in){return};in.Body=strings.TrimSpace(in.Body);if !utf8.ValidString(in.Body)||strings.ContainsRune(in.Body,0)||utf8.RuneCountInString(in.Body)<1||utf8.RuneCountInString(in.Body)>1000{writeJSON(w,400,map[string]string{"error":"invalid_message"});return};edited,err:=h.store.EditGroupMessage(r.Context(),id,u.ID,in.Body);if errors.Is(err,chat.ErrMessageUnavailable){writeJSON(w,403,map[string]string{"error":"message_not_editable"});return};if err!=nil{chatFailure(w,err);return};h.notifyGroupMessageFeature(r,u.ID);writeJSON(w,200,map[string]any{"edited_at":edited})
}
func (h *chatHandlers) searchGroupMessages(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=groupID(w,r);if !ok{return};q:=strings.TrimSpace(r.URL.Query().Get("q"));kind:=r.URL.Query().Get("type");if !utf8.ValidString(q)||utf8.RuneCountInString(q)>80||strings.ContainsRune(q,0){writeJSON(w,400,map[string]string{"error":"invalid_query"});return};if kind!=""&&kind!="messages"&&kind!="photos"&&kind!="videos"&&kind!="files"&&kind!="links"{writeJSON(w,400,map[string]string{"error":"invalid_type"});return};before,limit,ok:=pagination(w,r);if !ok{return};items,err:=h.store.SearchGroupMessages(r.Context(),id,u.ID,q,kind,before,limit+1);if err!=nil{chatFailure(w,err);return};cursor:="";if len(items)>limit{items=items[:limit];cursor=items[len(items)-1].ID};writeJSON(w,200,map[string]any{"messages":items,"next_cursor":cursor})
}
func (h *chatHandlers) listGroupPins(w http.ResponseWriter,r *http.Request,u auth.User){id,ok:=groupID(w,r);if !ok{return};items,err:=h.store.GroupPins(r.Context(),id,u.ID);if err!=nil{chatFailure(w,err);return};writeJSON(w,200,map[string]any{"messages":items})}
func (h *chatHandlers) setGroupPin(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=groupID(w,r);if !ok{return};var in struct{MessageID string `json:"message_id"`;Pinned bool `json:"pinned"`};if !readAuthJSON(w,r,&in){return};n,err:=strconv.ParseInt(in.MessageID,10,64);if err!=nil||n<1{writeJSON(w,400,map[string]string{"error":"invalid_message_id"});return};if err=h.store.SetGroupPin(r.Context(),id,strconv.FormatInt(n,10),u.ID,in.Pinned);err!=nil{chatFailure(w,err);return};h.notifyGroupMessageFeature(r,u.ID);writeJSON(w,200,map[string]bool{"pinned":in.Pinned})
}

func(h *chatHandlers)notifyGroupMessageFeature(r *http.Request,userID string){
	id:=strings.ToLower(r.PathValue("id"));if members,err:=h.store.GroupMembers(r.Context(),id,userID);err==nil{for _,member:=range members{h.publishGroupUpdate(id,member.ID)}}
}

func(h *chatHandlers)sendGroupFile(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=groupID(w,r);if !ok{return};data,reported,ok:=mediaUpload(w,r,"file",maxAttachment);if !ok{return};kind,ok:=allowedAttachment(data,reported);if !ok{writeJSON(w,400,map[string]string{"error":"unsupported_file_type"});return};name:=cleanAttachmentName(r.FormValue("filename"));if name==""{name="attachment"};ext:=strings.ToLower(filepath.Ext(name));if ext==".html"||ext==".htm"||ext==".svg"||ext==".js"||ext==".mjs"||ext==".xml"{writeJSON(w,400,map[string]string{"error":"unsupported_file_type"});return};clientID:=strings.ToLower(r.FormValue("client_message_id"));caption:=strings.TrimSpace(r.FormValue("caption"));if !uuidPattern.MatchString(clientID)||!utf8.ValidString(caption)||utf8.RuneCountInString(caption)>1000{writeJSON(w,400,map[string]string{"error":"invalid_file"});return};message,created,members,err:=h.store.SendGroupFile(r.Context(),id,u.ID,clientID,caption,name,kind,data);if err!=nil{chatFailure(w,err);return};if created{h.publishGroupUpdate(id,members...)};status:=200;if created{status=201};writeJSON(w,status,map[string]any{"message":message,"attachment_url":fmt.Sprintf("/api/groups/%s/messages/%s/file",id,message.ID)})
}

func(h *chatHandlers)downloadGroupFile(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=groupID(w,r);if !ok{return};messageID,ok:=groupMessageID(w,r);if !ok{return};data,name,contentType,err:=h.store.GroupFile(r.Context(),id,messageID,u.ID);if err!=nil{chatFailure(w,err);return};w.Header().Set("Content-Type",contentType);w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Cache-Control","private, no-store");disposition:="attachment";if strings.HasPrefix(contentType,"video/"){disposition="inline"};w.Header().Set("Content-Disposition",mime.FormatMediaType(disposition,map[string]string{"filename":name}));w.Header().Set("Content-Length",strconv.Itoa(len(data)));_,_=w.Write(data)
}
