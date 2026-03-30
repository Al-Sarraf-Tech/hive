import{d as Pe,a as W,b as d,f as c,s as p,c as Me,t as K}from"../chunks/BKzipXb4.js";import{o as Ne}from"../chunks/BBIrVZ6j.js";import{p as Ce,s as D,a as He,c as s,f as me,t as w,g as e,b as Le,d as l,e as o,r as a,i as je}from"../chunks/D67c9Gwo.js";import{i as f}from"../chunks/Cw8_URSI.js";import{e as Q,i as X}from"../chunks/DhzCUvt5.js";import{s as ze}from"../chunks/CKrvbhRz.js";import{s as Ge}from"../chunks/Dbb-5aXk.js";import{b as Ue}from"../chunks/BUgbhvgh.js";import{a as re}from"../chunks/D986jw_u.js";var We=c("<button> </button>"),Ye=c('<span class="muted">Pulling images and starting containers...</span>'),qe=c('<span class="muted">Computing deploy diff...</span>'),Be=c('<span class="muted">Validating hivefile...</span>'),Je=c('<div class="card" style="border-color: var(--green); margin-bottom:1rem"><span class="badge badge-green">Valid</span> <span class="muted" style="margin-left:0.5rem">Hivefile passed all checks</span></div>'),Ke=c('<span class="badge badge-red">error</span>'),Qe=c('<span class="badge badge-yellow">warning</span>'),Xe=c('<span class="badge">info</span>'),Ze=c('<tr><td><!></td><td> </td><td class="muted"> </td><td> </td></tr>'),$e=c('<div class="card" style="border-color: var(--yellow); margin-bottom:1rem"><div class="card-title">Validation Issues</div> <table style="margin-top:0.5rem"><thead><tr><th>Severity</th><th>Service</th><th>Field</th><th>Message</th></tr></thead><tbody></tbody></table></div>'),et=c('<span class="badge badge-green">create</span>'),tt=c('<span class="badge badge-yellow">update</span>'),at=c('<span class="badge">unchanged</span>'),rt=c('<tr><td> </td><td><!></td><td class="muted"><!></td><td><!></td><td class="muted"> </td></tr>'),st=c('<div class="card" style="border-color: var(--blue); margin-bottom:1rem"><div class="card-title">Deploy Preview</div> <table style="margin-top:0.5rem"><thead><tr><th>Service</th><th>Action</th><th>Image</th><th>Replicas</th><th>Changes</th></tr></thead><tbody></tbody></table></div>'),lt=c('<div class="card" style="border-color: var(--red)"><div class="card-title text-red">Deploy Failed</div> <pre class="exec-output" style="color:var(--red)"> </pre></div>'),it=c('<tr><td><a> </a></td><td class="muted"> </td><td> </td><td class="muted"> </td></tr>'),ot=c('<div class="card" style="border-color: var(--green)"><div class="card-title text-green">Deployed Successfully</div> <table style="margin-top:0.5rem"><thead><tr><th>Service</th><th>Image</th><th>Replicas</th><th>ID</th></tr></thead><tbody></tbody></table></div>'),nt=c('<div class="page-header"><h1 class="page-title">Deploy</h1></div> <div style="display:flex; gap:0.5rem; margin-bottom:1rem; flex-wrap:wrap"><span class="muted" style="align-self:center; font-size:0.8125rem">Template:</span> <!> <label class="btn btn-sm" style="cursor:pointer">Upload .toml <input type="file" accept=".toml,.txt" style="display:none"/></label></div> <div class="card" style="margin-bottom:1rem"><div class="card-title">Hivefile (TOML)</div> <textarea rows="20" style="margin-top:0.5rem; resize:vertical" spellcheck="false"></textarea> <div style="margin-top:1rem; display:flex; gap:0.5rem; align-items:center"><button class="btn btn-primary"> </button> <button class="btn btn-sm"> </button> <button class="btn btn-sm"> </button> <!> <!> <!></div></div> <!> <!> <!> <!>',1);function bt(_e,ue){Ce(ue,!0);const Z={blank:"",nginx:`[service.nginx]
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
`};let b=D(He(Z.nginx)),P=D(null),g=D(null),A=D(!1),Y=D("nginx"),I=D(null),F=D(!1),q=D(null),M=D(!1);Ne(()=>{const t=sessionStorage.getItem("hive_draft_toml");t&&(o(b,t,!0),o(Y,"blank"),sessionStorage.removeItem("hive_draft_toml"))});function ge(t){o(Y,t,!0),o(b,Z[t],!0),o(P,null),o(g,null)}async function fe(){if(!e(b).trim()){o(g,"Hivefile cannot be empty");return}o(F,!0),o(I,null),o(g,null);try{o(I,await re.validate(e(b),!0),!0)}catch(t){o(g,t.message,!0)}finally{o(F,!1)}}async function be(){if(!e(b).trim()){o(g,"Hivefile cannot be empty");return}o(M,!0),o(q,null),o(g,null);try{o(q,await re.diff(e(b)),!0)}catch(t){o(g,t.message,!0)}finally{o(M,!1)}}async function ye(){if(!e(b).trim()){o(g,"Hivefile cannot be empty");return}o(A,!0),o(g,null),o(P,null),o(I,null);try{o(P,await re.deploy(e(b)),!0)}catch(t){o(g,t.message,!0)}finally{o(A,!1)}}function he(t){var u;const r=(u=t.target.files)==null?void 0:u[0];if(!r)return;const v=new FileReader;v.onload=()=>{o(b,v.result,!0),o(Y,"blank")},v.readAsText(r)}var se=nt(),$=s(me(se),2),le=s(l($),2);Q(le,17,()=>Object.keys(Z).filter(t=>t!=="blank"),X,(t,r)=>{var v=We();let u;var E=l(v,!0);a(v),w(()=>{u=Ge(v,1,"btn btn-sm",null,u,{"btn-primary":e(Y)===e(r)}),p(E,e(r))}),W("click",v,()=>ge(e(r))),d(t,v)});var ie=s(le,2),xe=s(l(ie));a(ie),a($);var ee=s($,2),te=s(l(ee),2);je(te);var oe=s(te,2),N=l(oe),we=l(N,!0);a(N);var C=s(N,2),Ie=l(C,!0);a(C);var H=s(C,2),Re=l(H,!0);a(H);var ne=s(H,2);{var De=t=>{var r=Ye();d(t,r)};f(ne,t=>{e(A)&&t(De)})}var ve=s(ne,2);{var Te=t=>{var r=qe();d(t,r)};f(ve,t=>{e(M)&&t(Te)})}var ke=s(ve,2);{var Se=t=>{var r=Be();d(t,r)};f(ke,t=>{e(F)&&t(Se)})}a(oe),a(ee);var de=s(ee,2);{var Ae=t=>{var r=Me(),v=me(r);{var u=n=>{var _=Je();d(n,_)},E=n=>{var _=$e(),x=s(l(_),2),T=s(l(x));Q(T,21,()=>e(I).issues,X,(O,y)=>{var V=Ze(),R=l(V),L=l(R);{var k=h=>{var i=Ke();d(h,i)},j=h=>{var i=Qe();d(h,i)},S=h=>{var i=Xe();d(h,i)};f(L,h=>{e(y).severity==="VALIDATION_SEVERITY_ERROR"?h(k):e(y).severity==="VALIDATION_SEVERITY_WARNING"?h(j,1):h(S,-1)})}a(R);var z=s(R),G=l(z,!0);a(z);var U=s(z),ae=l(U,!0);a(U);var B=s(U),J=l(B,!0);a(B),a(V),w(()=>{p(G,e(y).service||"-"),p(ae,e(y).field||"-"),p(J,e(y).message)}),d(O,V)}),a(T),a(x),a(_),d(n,_)};f(v,n=>{var _;e(I).valid&&(!e(I).issues||e(I).issues.length===0)?n(u):(_=e(I).issues)!=null&&_.length&&n(E,1)})}d(t,r)};f(de,t=>{e(I)&&t(Ae)})}var ce=s(de,2);{var Ee=t=>{var r=st(),v=s(l(r),2),u=s(l(v));Q(u,21,()=>e(q).diffs,X,(E,n)=>{var _=rt(),x=l(_),T=l(x,!0);a(x);var O=s(x),y=l(O);{var V=i=>{var m=et();d(i,m)},R=i=>{var m=tt();d(i,m)},L=i=>{var m=at();d(i,m)};f(y,i=>{e(n).action==="DIFF_ACTION_CREATE"?i(V):e(n).action==="DIFF_ACTION_UPDATE"?i(R,1):i(L,-1)})}a(O);var k=s(O),j=l(k);{var S=i=>{var m=K();w(()=>p(m,`${e(n).oldImage??""} → ${e(n).newImage??""}`)),d(i,m)},z=i=>{var m=K();w(()=>p(m,e(n).newImage)),d(i,m)};f(j,i=>{e(n).oldImage&&e(n).oldImage!==e(n).newImage?i(S):i(z,-1)})}a(k);var G=s(k),U=l(G);{var ae=i=>{var m=K();w(()=>p(m,`${e(n).oldReplicas??""} → ${e(n).newReplicas??""}`)),d(i,m)},B=i=>{var m=K();w(()=>p(m,e(n).newReplicas??"-")),d(i,m)};f(U,i=>{e(n).oldReplicas&&e(n).oldReplicas!==e(n).newReplicas?i(ae):i(B,-1)})}a(G);var J=s(G),h=l(J,!0);a(J),a(_),w(i=>{p(T,e(n).name),p(h,i)},[()=>(e(n).changes??[]).join("; ")||"-"]),d(E,_)}),a(u),a(v),a(r),d(t,r)};f(ce,t=>{var r,v;(v=(r=e(q))==null?void 0:r.diffs)!=null&&v.length&&t(Ee)})}var pe=s(ce,2);{var Oe=t=>{var r=lt(),v=s(l(r),2),u=l(v,!0);a(v),a(r),w(()=>p(u,e(g))),d(t,r)};f(pe,t=>{e(g)&&t(Oe)})}var Ve=s(pe,2);{var Fe=t=>{var r=ot(),v=s(l(r),2),u=s(l(v));Q(u,21,()=>e(P).services,X,(E,n)=>{var _=it(),x=l(_),T=l(x),O=l(T,!0);a(T),a(x);var y=s(x),V=l(y,!0);a(y);var R=s(y),L=l(R);a(R);var k=s(R),j=l(k,!0);a(k),a(_),w(S=>{ze(T,"href",`/services/${e(n).name??""}`),p(O,e(n).name),p(V,e(n).image),p(L,`${e(n).replicasRunning??0??""}/${e(n).replicasDesired??0??""}`),p(j,S)},[()=>{var S;return((S=e(n).id)==null?void 0:S.substring(0,12))||"-"}]),d(E,_)}),a(u),a(v),a(r),d(t,r)};f(Ve,t=>{var r,v;(v=(r=e(P))==null?void 0:r.services)!=null&&v.length&&t(Fe)})}w(()=>{N.disabled=e(A)||e(F),p(we,e(A)?"Deploying...":"Deploy"),C.disabled=e(M)||e(A),p(Ie,e(M)?"Previewing...":"Preview"),H.disabled=e(F)||e(A),p(Re,e(F)?"Validating...":"Validate")}),W("change",xe,he),Ue(te,()=>e(b),t=>o(b,t)),W("click",N,ye),W("click",C,be),W("click",H,fe),d(_e,se),Le()}Pe(["click","change"]);export{bt as component};
