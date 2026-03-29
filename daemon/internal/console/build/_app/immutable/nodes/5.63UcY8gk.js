import{d as Pe,a as W,b as d,f as c,s as p,c as Ne,t as J}from"../chunks/XRp6h11U.js";import{p as Ce,d as D,h as He,s,f as me,t as w,g as e,a as Me,c as l,b as o,r as a,i as Le}from"../chunks/CIDl57gl.js";import{i as b}from"../chunks/DU4npVVY.js";import{e as K,i as Q}from"../chunks/4OxDuC_C.js";import{s as je}from"../chunks/Cbi2Kzjr.js";import{s as ze}from"../chunks/BgR-eiR2.js";import{b as Ge}from"../chunks/Cnz1yEet.js";import{a as re}from"../chunks/BtbBZSSy.js";var Ue=c("<button> </button>"),We=c('<span class="muted">Pulling images and starting containers...</span>'),Ye=c('<span class="muted">Computing deploy diff...</span>'),qe=c('<span class="muted">Validating hivefile...</span>'),Be=c('<div class="card" style="border-color: var(--green); margin-bottom:1rem"><span class="badge badge-green">Valid</span> <span class="muted" style="margin-left:0.5rem">Hivefile passed all checks</span></div>'),Je=c('<span class="badge badge-red">error</span>'),Ke=c('<span class="badge badge-yellow">warning</span>'),Qe=c('<span class="badge">info</span>'),Xe=c('<tr><td><!></td><td> </td><td class="muted"> </td><td> </td></tr>'),Ze=c('<div class="card" style="border-color: var(--yellow); margin-bottom:1rem"><div class="card-title">Validation Issues</div> <table style="margin-top:0.5rem"><thead><tr><th>Severity</th><th>Service</th><th>Field</th><th>Message</th></tr></thead><tbody></tbody></table></div>'),$e=c('<span class="badge badge-green">create</span>'),et=c('<span class="badge badge-yellow">update</span>'),tt=c('<span class="badge">unchanged</span>'),at=c('<tr><td> </td><td><!></td><td class="muted"><!></td><td><!></td><td class="muted"> </td></tr>'),rt=c('<div class="card" style="border-color: var(--blue); margin-bottom:1rem"><div class="card-title">Deploy Preview</div> <table style="margin-top:0.5rem"><thead><tr><th>Service</th><th>Action</th><th>Image</th><th>Replicas</th><th>Changes</th></tr></thead><tbody></tbody></table></div>'),st=c('<div class="card" style="border-color: var(--red)"><div class="card-title text-red">Deploy Failed</div> <pre class="exec-output" style="color:var(--red)"> </pre></div>'),lt=c('<tr><td><a> </a></td><td class="muted"> </td><td> </td><td class="muted"> </td></tr>'),it=c('<div class="card" style="border-color: var(--green)"><div class="card-title text-green">Deployed Successfully</div> <table style="margin-top:0.5rem"><thead><tr><th>Service</th><th>Image</th><th>Replicas</th><th>ID</th></tr></thead><tbody></tbody></table></div>'),nt=c('<div class="page-header"><h1 class="page-title">Deploy</h1></div> <div style="display:flex; gap:0.5rem; margin-bottom:1rem; flex-wrap:wrap"><span class="muted" style="align-self:center; font-size:0.8125rem">Template:</span> <!> <label class="btn btn-sm" style="cursor:pointer">Upload .toml <input type="file" accept=".toml,.txt" style="display:none"/></label></div> <div class="card" style="margin-bottom:1rem"><div class="card-title">Hivefile (TOML)</div> <textarea rows="20" style="margin-top:0.5rem; resize:vertical" spellcheck="false"></textarea> <div style="margin-top:1rem; display:flex; gap:0.5rem; align-items:center"><button class="btn btn-primary"> </button> <button class="btn btn-sm"> </button> <button class="btn btn-sm"> </button> <!> <!> <!></div></div> <!> <!> <!> <!>',1);function gt(_e,ue){Ce(ue,!0);const X={blank:"",nginx:`[service.nginx]
image = "nginx:alpine"
replicas = 2
restart_policy = "on-failure"

[service.nginx.ports]
"8080" = "80"

[service.nginx.health]
type = "http"
port = 80
path = "/"
interval = "30s"
timeout = "5s"
retries = 3

[service.nginx.resources]
memory = "128M"
cpus = 0.5
`,postgres:`[service.postgres]
image = "postgres:16-alpine"
replicas = 1
restart_policy = "always"

[service.postgres.ports]
"5432" = "5432"

[service.postgres.env]
POSTGRES_PASSWORD = "{{ secret:pg-password }}"

[service.postgres.health]
type = "tcp"
port = 5432
interval = "30s"
timeout = "5s"
retries = 5

[service.postgres.resources]
memory = "512M"
cpus = 1.0
`,redis:`[service.redis]
image = "redis:7-alpine"
replicas = 1
restart_policy = "always"

[service.redis.ports]
"6379" = "6379"

[service.redis.health]
type = "tcp"
port = 6379
interval = "30s"
timeout = "5s"
retries = 3

[service.redis.resources]
memory = "256M"
cpus = 0.5
`};let h=D(He(X.nginx)),P=D(null),g=D(null),S=D(!1),Z=D("nginx"),I=D(null),F=D(!1),Y=D(null),N=D(!1);function ge(t){o(Z,t,!0),o(h,X[t],!0),o(P,null),o(g,null)}async function be(){if(!e(h).trim()){o(g,"Hivefile cannot be empty");return}o(F,!0),o(I,null),o(g,null);try{o(I,await re.validate(e(h),!0),!0)}catch(t){o(g,t.message,!0)}finally{o(F,!1)}}async function fe(){if(!e(h).trim()){o(g,"Hivefile cannot be empty");return}o(N,!0),o(Y,null),o(g,null);try{o(Y,await re.diff(e(h)),!0)}catch(t){o(g,t.message,!0)}finally{o(N,!1)}}async function ye(){if(!e(h).trim()){o(g,"Hivefile cannot be empty");return}o(S,!0),o(g,null),o(P,null),o(I,null);try{o(P,await re.deploy(e(h)),!0)}catch(t){o(g,t.message,!0)}finally{o(S,!1)}}function he(t){var u;const r=(u=t.target.files)==null?void 0:u[0];if(!r)return;const v=new FileReader;v.onload=()=>{o(h,v.result,!0),o(Z,"blank")},v.readAsText(r)}var se=nt(),$=s(me(se),2),le=s(l($),2);K(le,17,()=>Object.keys(X).filter(t=>t!=="blank"),Q,(t,r)=>{var v=Ue();let u;var E=l(v,!0);a(v),w(()=>{u=ze(v,1,"btn btn-sm",null,u,{"btn-primary":e(Z)===e(r)}),p(E,e(r))}),W("click",v,()=>ge(e(r))),d(t,v)});var ie=s(le,2),xe=s(l(ie));a(ie),a($);var ee=s($,2),te=s(l(ee),2);Le(te);var ne=s(te,2),C=l(ne),we=l(C,!0);a(C);var H=s(C,2),Ie=l(H,!0);a(H);var M=s(H,2),Re=l(M,!0);a(M);var oe=s(M,2);{var De=t=>{var r=We();d(t,r)};b(oe,t=>{e(S)&&t(De)})}var ve=s(oe,2);{var Te=t=>{var r=Ye();d(t,r)};b(ve,t=>{e(N)&&t(Te)})}var ke=s(ve,2);{var Ae=t=>{var r=qe();d(t,r)};b(ke,t=>{e(F)&&t(Ae)})}a(ne),a(ee);var de=s(ee,2);{var Se=t=>{var r=Ne(),v=me(r);{var u=n=>{var _=Be();d(n,_)},E=n=>{var _=Ze(),x=s(l(_),2),T=s(l(x));K(T,21,()=>e(I).issues,Q,(O,f)=>{var V=Xe(),R=l(V),L=l(R);{var k=y=>{var i=Je();d(y,i)},j=y=>{var i=Ke();d(y,i)},A=y=>{var i=Qe();d(y,i)};b(L,y=>{e(f).severity==="VALIDATION_SEVERITY_ERROR"?y(k):e(f).severity==="VALIDATION_SEVERITY_WARNING"?y(j,1):y(A,-1)})}a(R);var z=s(R),G=l(z,!0);a(z);var U=s(z),ae=l(U,!0);a(U);var q=s(U),B=l(q,!0);a(q),a(V),w(()=>{p(G,e(f).service||"-"),p(ae,e(f).field||"-"),p(B,e(f).message)}),d(O,V)}),a(T),a(x),a(_),d(n,_)};b(v,n=>{var _;e(I).valid&&(!e(I).issues||e(I).issues.length===0)?n(u):(_=e(I).issues)!=null&&_.length&&n(E,1)})}d(t,r)};b(de,t=>{e(I)&&t(Se)})}var ce=s(de,2);{var Ee=t=>{var r=rt(),v=s(l(r),2),u=s(l(v));K(u,21,()=>e(Y).diffs,Q,(E,n)=>{var _=at(),x=l(_),T=l(x,!0);a(x);var O=s(x),f=l(O);{var V=i=>{var m=$e();d(i,m)},R=i=>{var m=et();d(i,m)},L=i=>{var m=tt();d(i,m)};b(f,i=>{e(n).action==="DIFF_ACTION_CREATE"?i(V):e(n).action==="DIFF_ACTION_UPDATE"?i(R,1):i(L,-1)})}a(O);var k=s(O),j=l(k);{var A=i=>{var m=J();w(()=>p(m,`${e(n).oldImage??""} → ${e(n).newImage??""}`)),d(i,m)},z=i=>{var m=J();w(()=>p(m,e(n).newImage)),d(i,m)};b(j,i=>{e(n).oldImage&&e(n).oldImage!==e(n).newImage?i(A):i(z,-1)})}a(k);var G=s(k),U=l(G);{var ae=i=>{var m=J();w(()=>p(m,`${e(n).oldReplicas??""} → ${e(n).newReplicas??""}`)),d(i,m)},q=i=>{var m=J();w(()=>p(m,e(n).newReplicas??"-")),d(i,m)};b(U,i=>{e(n).oldReplicas&&e(n).oldReplicas!==e(n).newReplicas?i(ae):i(q,-1)})}a(G);var B=s(G),y=l(B,!0);a(B),a(_),w(i=>{p(T,e(n).name),p(y,i)},[()=>(e(n).changes??[]).join("; ")||"-"]),d(E,_)}),a(u),a(v),a(r),d(t,r)};b(ce,t=>{var r,v;(v=(r=e(Y))==null?void 0:r.diffs)!=null&&v.length&&t(Ee)})}var pe=s(ce,2);{var Oe=t=>{var r=st(),v=s(l(r),2),u=l(v,!0);a(v),a(r),w(()=>p(u,e(g))),d(t,r)};b(pe,t=>{e(g)&&t(Oe)})}var Ve=s(pe,2);{var Fe=t=>{var r=it(),v=s(l(r),2),u=s(l(v));K(u,21,()=>e(P).services,Q,(E,n)=>{var _=lt(),x=l(_),T=l(x),O=l(T,!0);a(T),a(x);var f=s(x),V=l(f,!0);a(f);var R=s(f),L=l(R);a(R);var k=s(R),j=l(k,!0);a(k),a(_),w(A=>{je(T,"href",`/services/${e(n).name??""}`),p(O,e(n).name),p(V,e(n).image),p(L,`${e(n).replicasRunning??0??""}/${e(n).replicasDesired??0??""}`),p(j,A)},[()=>{var A;return((A=e(n).id)==null?void 0:A.substring(0,12))||"-"}]),d(E,_)}),a(u),a(v),a(r),d(t,r)};b(Ve,t=>{var r,v;(v=(r=e(P))==null?void 0:r.services)!=null&&v.length&&t(Fe)})}w(()=>{C.disabled=e(S)||e(F),p(we,e(S)?"Deploying...":"Deploy"),H.disabled=e(N)||e(S),p(Ie,e(N)?"Previewing...":"Preview"),M.disabled=e(F)||e(S),p(Re,e(F)?"Validating...":"Validate")}),W("change",xe,he),Ge(te,()=>e(h),t=>o(h,t)),W("click",C,ye),W("click",H,fe),W("click",M,be),d(_e,se),Me()}Pe(["click","change"]);export{gt as component};
