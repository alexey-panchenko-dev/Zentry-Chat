import {execFileSync} from 'node:child_process';
import {readFile} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const root=fileURLToPath(new URL('../../',import.meta.url));
const base='e6f6fe5d32a3820803e7797b9f91364c62c32526';
// Account tools/audio were subsequently authorized. Keep the remaining original
// contracts protected; their new behavior is checked by runtime tests.
const authorizedBackend=['api/internal/auth/auth.go','api/internal/auth/recovery*','api/internal/httpapi/recovery*','api/internal/httpapi/saved*','api/internal/httpapi/router.go','api/internal/httpapi/chat.go','api/internal/httpapi/chat_integration_test.go','api/internal/calls/store.go','api/internal/database/migrations/004_account_tools.sql'];
const protectedPaths=['api',...authorizedBackend.map(path=>`:(exclude)${path}`),'client/src/api.ts','client/src/main.tsx','client/package.json','client/package-lock.json','Dockerfile','render.yaml','client/vite.config.ts','client/vercel.json'];
execFileSync('git',['diff','--exit-code',base,'--',...protectedPaths],{cwd:root,stdio:'pipe'});
function calls(text,pattern){const found=[];for(const match of text.matchAll(pattern)){const start=match.index,open=text.indexOf('(',start);let depth=0,quote='',escaped=false;for(let i=open;i<text.length;i++){const c=text[i];if(quote){if(escaped){escaped=false;continue;}if(c==='\\'){escaped=true;continue;}if(c===quote)quote='';continue;}if(c==='"'||c==="'"||c==='`'){quote=c;continue;}if(c==='(')depth++;if(c===')'&&--depth===0){found.push(text.slice(start,i+1).replace(/\s+/g,''));break;}}}return found;}
for(const path of ['client/src/App.tsx','client/src/CallPanel.tsx']){const before=execFileSync('git',['show',`${base}:${path}`],{cwd:root,encoding:'utf8'}),after=await readFile(new URL(path,new URL('../../',import.meta.url)),'utf8');const pattern=/\bapi\.(?:get|post|interceptors\.response\.use)\s*(?:<[^;\n]*?>)?\s*\(/g;const remaining=calls(after,pattern);for(const original of calls(before,pattern)){const index=remaining.indexOf(original);assert.ok(index>=0,`${path}: original API call/payload changed`);remaining.splice(index,1)}for(const added of remaining)assert.ok(added.includes("'/conversations/saved'"),`${path}: unexpected added API call`);const effects=calls(after,/\buseEffect\s*\(/g);for(const original of calls(before,/\buseEffect\s*\(/g))assert.ok(effects.includes(original),`${path}: original effect changed`);}
console.log('PASS: original API calls/effects and unaffected backend/deployment/dependency files preserved; authorized account tools/audio changes require their runtime tests.');
