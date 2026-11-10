package httpapi

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
)

var allowedReaction = map[string]bool{"❤️":true,"👍":true,"😂":true,"😮":true,"😢":true,"🔥":true}
var safeFileName = regexp.MustCompile(`[\p{L}\p{N}._() -]`)

func messageNumericID(w http.ResponseWriter,r *http.Request)(string,bool){
	id:=r.PathValue("messageID");n,err:=strconv.ParseInt(id,10,64);if err!=nil||n<1{writeJSON(w,404,map[string]string{"error":"not_found"});return "",false};return strconv.FormatInt(n,10),true
}
func (h *chatHandlers) reactMessage(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=messageNumericID(w,r);if !ok{return};var in struct{Emoji string `json:"emoji"`};if !readAuthJSON(w,r,&in){return};if in.Emoji!=""&&!allowedReaction[in.Emoji]{writeJSON(w,400,map[string]string{"error":"invalid_reaction"});return}
	values,err:=h.store.SetReaction(r.Context(),id,u.ID,in.Emoji);if err!=nil{chatFailure(w,err);return};h.notifyMessageFeature(r,id,u.ID);writeJSON(w,200,map[string]any{"reactions":values})
}
func (h *chatHandlers) listMessageReactions(w http.ResponseWriter,r *http.Request,u auth.User){id,ok:=messageNumericID(w,r);if !ok{return};values,err:=h.store.Reactions(r.Context(),id,u.ID);if err!=nil{chatFailure(w,err);return};writeJSON(w,200,map[string]any{"reactions":values})}
func (h *chatHandlers) editMessage(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=messageNumericID(w,r);if !ok{return};var in struct{Body string `json:"body"`};if !readAuthJSON(w,r,&in){return};in.Body=strings.TrimSpace(in.Body);if !utf8.ValidString(in.Body)||strings.ContainsRune(in.Body,0)||utf8.RuneCountInString(in.Body)<1||utf8.RuneCountInString(in.Body)>1000{writeJSON(w,400,map[string]string{"error":"invalid_message"});return}
	m,err:=h.store.EditMessage(r.Context(),id,u.ID,in.Body);if errors.Is(err,chat.ErrMessageUnavailable){writeJSON(w,403,map[string]string{"error":"message_not_editable"});return};if err!=nil{chatFailure(w,err);return};h.notifyMessageFeature(r,id,u.ID);writeJSON(w,200,map[string]any{"message":m})
}
func (h *chatHandlers) searchMessages(w http.ResponseWriter,r *http.Request,u auth.User){
	cid,ok:=conversationID(w,r);if !ok{return};q:=strings.TrimSpace(r.URL.Query().Get("q"));kind:=r.URL.Query().Get("type");if !utf8.ValidString(q)||utf8.RuneCountInString(q)>80||strings.ContainsRune(q,0){writeJSON(w,400,map[string]string{"error":"invalid_query"});return};if kind!=""&&kind!="messages"&&kind!="photos"&&kind!="videos"&&kind!="voice"&&kind!="files"&&kind!="links"{writeJSON(w,400,map[string]string{"error":"invalid_type"});return};before,limit,ok:=pagination(w,r);if !ok{return};items,err:=h.store.SearchMessages(r.Context(),cid,u.ID,q,kind,before,limit+1);if err!=nil{chatFailure(w,err);return};cursor:="";if len(items)>limit{items=items[:limit];cursor=items[len(items)-1].ID};writeJSON(w,200,map[string]any{"messages":items,"next_cursor":cursor})
}
func (h *chatHandlers) listPins(w http.ResponseWriter,r *http.Request,u auth.User){cid,ok:=conversationID(w,r);if !ok{return};items,err:=h.store.Pinned(r.Context(),cid,u.ID);if err!=nil{chatFailure(w,err);return};writeJSON(w,200,map[string]any{"messages":items})}
func (h *chatHandlers) setPin(w http.ResponseWriter,r *http.Request,u auth.User){
	cid,ok:=conversationID(w,r);if !ok{return};var in struct{MessageID string `json:"message_id"`;Pinned bool `json:"pinned"`};if !readAuthJSON(w,r,&in){return};id,err:=strconv.ParseInt(in.MessageID,10,64);if err!=nil||id<1{writeJSON(w,400,map[string]string{"error":"invalid_message_id"});return};if err=h.store.SetPinned(r.Context(),cid,in.MessageID,u.ID,in.Pinned);err!=nil{chatFailure(w,err);return};h.notifyMessageFeature(r,in.MessageID,u.ID);writeJSON(w,200,map[string]bool{"pinned":in.Pinned})
}
func (h *chatHandlers) forwardMessage(w http.ResponseWriter,r *http.Request,u auth.User){
	destination,ok:=conversationID(w,r);if !ok{return};var in struct{MessageID string `json:"message_id"`;ClientMessageID string `json:"client_message_id"`};if !readAuthJSON(w,r,&in){return};source,err:=strconv.ParseInt(in.MessageID,10,64);if err!=nil||source<1||!uuidPattern.MatchString(strings.ToLower(in.ClientMessageID)){writeJSON(w,400,map[string]string{"error":"invalid_forward"});return};m,err:=h.store.ForwardMessage(r.Context(),strconv.FormatInt(source,10),destination,u.ID,strings.ToLower(in.ClientMessageID));if err!=nil{chatFailure(w,err);return};if h.publisher.Enabled(){_ = h.publisher.Publish("private-conversation-"+destination,"message.created",map[string]any{"message":m})};if members,e:=h.store.Members(r.Context(),destination);e==nil{h.publishConversationUpdate("conversation.message",destination,members...)};writeJSON(w,201,map[string]any{"message":m})
}

func(h *chatHandlers)notifyMessageFeature(r *http.Request,messageID,userID string){
	cid,err:=h.store.MessageConversation(r.Context(),messageID,userID);if err!=nil{return};if h.publisher.Enabled(){_ = h.publisher.Publish("private-conversation-"+cid,"message.feature.updated",map[string]string{"message_id":messageID})};if members,e:=h.store.Members(r.Context(),cid);e==nil{h.publishConversationUpdate("conversation.message",cid,members...)}
}

const maxAttachment=15*1024*1024
func allowedAttachment(b []byte,reported string)(string,bool){
	if len(b)>=12&&string(b[4:8])=="ftyp"{return "video/mp4",true}
	if len(b)>=4&&b[0]==0x1a&&b[1]==0x45&&b[2]==0xdf&&b[3]==0xa3{return "video/webm",true}
	detected:=http.DetectContentType(b);switch detected{
	case "video/mp4": return "video/mp4",true
	case "video/webm": if len(b)>=4&&b[0]==0x1a&&b[1]==0x45&&b[2]==0xdf&&b[3]==0xa3{return "video/webm",true}
	case "application/pdf": if strings.HasPrefix(string(b),"%PDF-"){return "application/pdf",true}
	case "text/plain; charset=utf-8","text/plain; charset=utf-16le","text/plain; charset=utf-16be": if !strings.Contains(strings.ToLower(reported),"html")&&!strings.Contains(strings.ToLower(reported),"svg"){return "text/plain; charset=utf-8",true}
	case "text/csv; charset=utf-8":return "text/csv; charset=utf-8",true
	case "application/zip":if len(b)>=4&&b[0]=='P'&&b[1]=='K'{return "application/zip",true}
	case "application/x-ole-storage":if len(b)>=8&&string(b[:8])=="\xd0\xcf\x11\xe0\xa1\xb1\x1a\xe1"{return "application/x-ole-storage",true}
	};return "",false
}
func cleanAttachmentName(name string)string{
	name=filepath.Base(strings.ReplaceAll(name,"\\","/"));var b strings.Builder
	count:=0;for _,r:=range name{if count>=180{break};if r=='/'||r=='\\'||r==':'||r<32||r==127{continue};if safeFileName.MatchString(string(r)){b.WriteRune(r);count++}}
	return strings.TrimSpace(b.String())
}
func(h *chatHandlers)sendFile(w http.ResponseWriter,r *http.Request,u auth.User){
	cid,ok:=conversationID(w,r);if !ok{return};data,mimeType,ok:=mediaUpload(w,r,"file",maxAttachment);if !ok{return};if len(data)==0{writeJSON(w,400,map[string]string{"error":"invalid_media"});return};mimeType,ok=allowedAttachment(data,mimeType);if !ok{writeJSON(w,400,map[string]string{"error":"unsupported_file_type"});return};name:=cleanAttachmentName(r.FormValue("filename"));if name==""{name="attachment"};ext:=strings.ToLower(filepath.Ext(name));if ext==".html"||ext==".htm"||ext==".svg"||ext==".js"||ext==".mjs"||ext==".xml"{writeJSON(w,400,map[string]string{"error":"unsupported_file_type"});return};clientID:=strings.ToLower(r.FormValue("client_message_id"));caption:=strings.TrimSpace(r.FormValue("caption"));if !uuidPattern.MatchString(clientID)||!utf8.ValidString(caption)||utf8.RuneCountInString(caption)>1000{writeJSON(w,400,map[string]string{"error":"invalid_file"});return};m,created,err:=h.store.SendFile(r.Context(),cid,u.ID,clientID,caption,name,mimeType,data);if err!=nil{chatFailure(w,err);return};m.Kind="file";m.AttachmentURL="/api/messages/"+m.ID+"/file";status:=200;if created{status=201};if created{if h.publisher.Enabled(){_ = h.publisher.Publish("private-conversation-"+cid,"message.created",map[string]any{"message":m})};if members,e:=h.store.Members(r.Context(),cid);e==nil{h.publishConversationUpdate("conversation.message",cid,members...)}};writeJSON(w,status,map[string]any{"message":m,"attachment_url":fmt.Sprintf("/api/messages/%s/file",m.ID)})
}
func(h *chatHandlers)downloadFile(w http.ResponseWriter,r *http.Request,u auth.User){
	id,ok:=messageNumericID(w,r);if !ok{return};data,name,contentType,err:=h.store.File(r.Context(),id,u.ID);if err!=nil{chatFailure(w,err);return};w.Header().Set("Content-Type",contentType);w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("Cache-Control","private, no-store");disposition:="attachment";if strings.HasPrefix(contentType,"video/"){disposition="inline"};w.Header().Set("Content-Disposition",mime.FormatMediaType(disposition,map[string]string{"filename":name}));w.Header().Set("Content-Length",strconv.Itoa(len(data)));_,_=w.Write(data)
}
