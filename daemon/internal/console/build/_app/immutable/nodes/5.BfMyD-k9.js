import{d as se,a as $,b as m,f as u,s as p}from"../chunks/BBOQw17b.js";import{p as ie,d as b,e as le,s as i,f as oe,t as h,g as t,a as ne,c as l,b as o,r as s,h as pe}from"../chunks/ZOJb4-bF.js";import{i as z}from"../chunks/cwCSKSEH.js";import{e as L,i as U}from"../chunks/BvGaTVC-.js";import{s as ce}from"../chunks/pUtjcX6Y.js";import{s as de}from"../chunks/BIL0EoYH.js";import{b as ve}from"../chunks/DhHmdtA4.js";import{a as me}from"../chunks/kPz4V3f7.js";var ue=u("<button> </button>"),ge=u('<span class="muted">Pulling images and starting containers...</span>'),ye=u('<div class="card" style="border-color: var(--red)"><div class="card-title text-red">Deploy Failed</div> <pre class="exec-output" style="color:var(--red)"> </pre></div>'),fe=u('<tr><td><a> </a></td><td class="muted"> </td><td> </td><td class="muted"> </td></tr>'),be=u('<div class="card" style="border-color: var(--green)"><div class="card-title text-green">Deployed Successfully</div> <table style="margin-top:0.5rem"><thead><tr><th>Service</th><th>Image</th><th>Replicas</th><th>ID</th></tr></thead><tbody></tbody></table></div>'),_e=u('<div class="page-header"><h1 class="page-title">Deploy</h1></div> <div style="display:flex; gap:0.5rem; margin-bottom:1rem; flex-wrap:wrap"><span class="muted" style="align-self:center; font-size:0.8125rem">Template:</span> <!> <label class="btn btn-sm" style="cursor:pointer">Upload .toml <input type="file" accept=".toml,.txt" style="display:none"/></label></div> <div class="card" style="margin-bottom:1rem"><div class="card-title">Hivefile (TOML)</div> <textarea rows="20" style="margin-top:0.5rem; resize:vertical" spellcheck="false"></textarea> <div style="margin-top:1rem; display:flex; gap:0.5rem; align-items:center"><button class="btn btn-primary"> </button> <!></div></div> <!> <!>',1);function Me(W,q){ie(q,!0);const x={blank:"",nginx:`[service.nginx]
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
`};let c=b(le(x.nginx)),g=b(null),d=b(null),y=b(!1),k=b("nginx");function B(e){o(k,e,!0),o(c,x[e],!0),o(g,null),o(d,null)}async function C(){if(!t(c).trim()){o(d,"Hivefile cannot be empty");return}o(y,!0),o(d,null),o(g,null);try{o(g,await me.deploy(t(c)),!0)}catch(e){o(d,e.message,!0)}finally{o(y,!1)}}function J(e){var n;const r=(n=e.target.files)==null?void 0:n[0];if(!r)return;const a=new FileReader;a.onload=()=>{o(c,a.result,!0),o(k,"blank")},a.readAsText(r)}var A=_e(),w=i(oe(A),2),H=i(l(w),2);L(H,17,()=>Object.keys(x).filter(e=>e!=="blank"),U,(e,r)=>{var a=ue();let n;var T=l(a,!0);s(a),h(()=>{n=de(a,1,"btn btn-sm",null,n,{"btn-primary":t(k)===t(r)}),p(T,t(r))}),$("click",a,()=>B(t(r))),m(e,a)});var I=i(H,2),K=i(l(I));s(I),s(w);var D=i(w,2),S=i(l(D),2);pe(S);var j=i(S,2),f=l(j),N=l(f,!0);s(f);var Q=i(f,2);{var V=e=>{var r=ge();m(e,r)};z(Q,e=>{t(y)&&e(V)})}s(j),s(D);var E=i(D,2);{var X=e=>{var r=ye(),a=i(l(r),2),n=l(a,!0);s(a),s(r),h(()=>p(n,t(d))),m(e,r)};z(E,e=>{t(d)&&e(X)})}var Y=i(E,2);{var Z=e=>{var r=be(),a=i(l(r),2),n=i(l(a));L(n,21,()=>t(g).services,U,(T,v)=>{var R=fe(),M=l(R),O=l(M),ee=l(O,!0);s(O),s(M);var F=i(M),te=l(F,!0);s(F);var P=i(F),re=l(P);s(P);var G=i(P),ae=l(G,!0);s(G),s(R),h(_=>{ce(O,"href",`/services/${t(v).name??""}`),p(ee,t(v).name),p(te,t(v).image),p(re,`${t(v).replicasRunning??""}/${t(v).replicasDesired??""}`),p(ae,_)},[()=>{var _;return((_=t(v).id)==null?void 0:_.substring(0,12))||"-"}]),m(T,R)}),s(n),s(a),s(r),m(e,r)};z(Y,e=>{var r,a;(a=(r=t(g))==null?void 0:r.services)!=null&&a.length&&e(Z)})}h(()=>{f.disabled=t(y),p(N,t(y)?"Deploying...":"Deploy")}),$("change",K,J),ve(S,()=>t(c),e=>o(c,e)),$("click",f,C),m(W,A),ne()}se(["click","change"]);export{Me as component};
