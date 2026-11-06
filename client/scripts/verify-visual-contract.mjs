import {execFileSync} from 'node:child_process';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const root=fileURLToPath(new URL('../../',import.meta.url));
const base='0d61250a8c9d31be2a49fb22c58b3edbc35dbf36';
// The user authorized call audio repairs and additive persistent media support.
// Preserve deployment/dependencies, original calls and all unrelated API files.
const mediaAPI=new Set(['api/CHAT.md','api/internal/auth/auth.go','api/internal/chat/chat.go','api/internal/chat/media.go','api/internal/chat/contact.go','api/internal/httpapi/contact.go','api/internal/httpapi/contact_test.go','api/internal/httpapi/contact_integration_test.go','api/internal/database/migrations/008_profile_contacts.sql','api/internal/httpapi/router.go','api/internal/httpapi/chat.go','api/internal/httpapi/chat_integration_test.go','api/internal/httpapi/auth_security.go','api/internal/httpapi/media.go','api/internal/httpapi/audio_validation.go','api/internal/httpapi/media_test.go','api/internal/httpapi/media_integration_test.go','api/internal/httpapi/media_security_test.go','api/internal/database/migrations/005_media.sql','api/internal/database/migrations/006_profile_photo_revision.sql','api/internal/database/migrations/007_media_content_not_null.sql']);
const changedAPI=execFileSync('git',['diff','--name-only',base,'--','api'],{cwd:root,encoding:'utf8'}).trim().split(/\r?\n/).filter(Boolean);
for(const path of changedAPI)assert.ok(mediaAPI.has(path)||path.startsWith('api/internal/httpapi/testdata/'),`Unrelated API change: ${path}`);
const protectedPaths=['client/src/calls.ts','client/src/RemoteAudio.tsx','client/src/remote-audio.ts','client/src/call-audio-context.ts','client/src/audio-meter.ts','client/src/realtime.ts','client/src/read-receipt.ts','client/src/read-visibility.ts','client/src/conversation-updates.ts','client/src/main.tsx','client/package.json','client/package-lock.json','compose.yaml','Dockerfile','render.yaml','client/vite.config.ts','client/vercel.json'];
execFileSync('git',['diff','--exit-code',base,'--',...protectedPaths],{cwd:root,stdio:'pipe'});
function calls(text,pattern){const found=[];for(const match of text.matchAll(pattern)){const start=match.index,open=text.indexOf('(',start);let depth=0,quote='',escaped=false;for(let i=open;i<text.length;i++){const c=text[i];if(quote){if(escaped){escaped=false;continue;}if(c==='\\'){escaped=true;continue;}if(c===quote)quote='';continue;}if(c==='"'||c==="'"||c==='`'){quote=c;continue;}if(c==='(')depth++;if(c===')'&&--depth===0){found.push(text.slice(start,i+1).replace(/\s+/g,''));break;}}}return found;}
for(const path of ['client/src/App.tsx','client/src/CallPanel.tsx']){const before=execFileSync('git',['show',`${base}:${path}`],{cwd:root,encoding:'utf8'}),after=await readFile(new URL(path,new URL('../../',import.meta.url)),'utf8');assert.deepEqual(calls(after,/\bapi\.(?:get|post|interceptors\.response\.use)\s*(?:<[^;\n]*?>)?\s*\(/g).sort(),calls(before,/\bapi\.(?:get|post|interceptors\.response\.use)\s*(?:<[^;\n]*?>)?\s*\(/g).sort(),`${path}: API calls/payloads changed`);const effects=calls(after,/\buseEffect\s*\(/g);for(const original of calls(before,/\buseEffect\s*\(/g))assert.ok(effects.includes(original),`${path}: original effect changed`);}
{
const path='client/src/calls.ts';
const before=execFileSync('git',['show',`${base}:${path}`],{cwd:root,encoding:'utf8'}),after=await readFile(new URL(path,new URL('../../',import.meta.url)),'utf8');
assert.deepEqual(calls(after,/\bapi\.(?:get|post)\s*(?:<[^;\n]*?>)?\s*\(/g).sort(),calls(before,/\bapi\.(?:get|post)\s*(?:<[^;\n]*?>)?\s*\(/g).sort(),'Call signaling API calls/payloads changed');
}
{
const path='client/src/api.ts',before=execFileSync('git',['show',`${base}:${path}`],{cwd:root,encoding:'utf8'}),after=await readFile(new URL(path,new URL('../../',import.meta.url)),'utf8');
assert.deepEqual(calls(after,/\baxios\.create\s*\(/g),calls(before,/\baxios\.create\s*\(/g),'API adapter transport changed');
}
console.log('PASS: deployment/dependencies/transport unchanged; original UI effects and text/signaling API calls preserved; backend edits restricted to authorized media files. Run Go and client tests for runtime behavior.');
