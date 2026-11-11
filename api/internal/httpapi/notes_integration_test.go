package httpapi

import (
 "encoding/json"
 "testing"
 "newsChat/api/internal/chat"
)

func TestPrivateNotesOnlyOwner(t *testing.T) {
 f:=newControlFixture(t)
 first:=f.request(t,0,"POST","/api/notes","{}",200)
 var group chat.Group
 if err:=json.Unmarshal(first["group"],&group);err!=nil||group.MemberCount!=1{t.Fatal("private notes",err)}
 second:=f.request(t,0,"POST","/api/notes","{}",200)
 var same chat.Group;json.Unmarshal(second["group"],&same)
 if same.ID!=group.ID{t.Fatal("duplicate notes chat")}
 f.request(t,1,"GET","/api/groups/"+group.ID,"",404)
 f.request(t,1,"GET","/api/groups/"+group.ID+"/messages","",404)
 f.request(t,0,"POST","/api/groups/"+group.ID+"/messages",`{"client_message_id":"10101010-1010-4010-8010-101010101010","body":"Личная заметка"}`,201)
}
