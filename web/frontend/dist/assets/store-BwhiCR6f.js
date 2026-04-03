import{E as e}from"./runtime-core.esm-bundler-y1Sx3e_r.js";import{Dn as t,En as n,Mn as r,Nn as i,On as a,Qt as o,Sn as s,bn as c,en as l,hn as u,it as d,jn as f,kn as p,pn as m,vn as h,wn as g,yn as _}from"./dist-fTOPYDxG.js";import{i as ee,r as te}from"./browser-JyujKM2J.js";var v=e=>e!=null,ne={"&":`&amp;`,"<":`&lt;`,">":`&gt;`,'"':`&quot;`,"'":`&apos;`};function y(e){return e.replace(/[&<>"']/g,e=>ne[e]??e)}function re(e,t={}){let{indent:n=`  `,format:r=!0,xmlDeclaration:i=!0}=t,a=(e,t,r)=>{let i=``;if(Array.isArray(e))for(let n=0,o=e.length;n<o;n++)i+=a(e[n],t,r);else if(typeof e==`object`&&e){let o=!1,s=``,c=``;for(let t in e)t.charAt(0)===`@`&&(s+=` `+t.substr(1)+`="`+y(e[t].toString())+`"`);for(let t in e)if(t===`#text`)c+=y(e[t]?.toString()??``);else if(t===`#cdata`){let n=e[t]?.toString()??``;c+=`<![CDATA[`+n.replace(/]]>/g,`]]]]><![CDATA[>`)+`]]>`}else t.charAt(0)!==`@`&&(o=!0,c+=a(e[t],t,r+n));o||c?(i+=r+`<`+t+s+`>
`,i+=c,i+=r+`</`+t+`>
`):i+=r+`<`+t+s+`/>
`}else i+=r+`<`+t+`>`+y(e?.toString()||``)+`</`+t+`>
`;return i},o=``;i&&(o+=`<?xml version="1.0" encoding="UTF-8"?>`,r&&(o+=`
`));for(let t in e)Object.hasOwn(e,t)&&(o+=a(e[t],t,``));return r?o.trim():o.replace(/\n/g,``).replace(/>\s+</g,`><`).trim()}var ie=[`post`,`put`,`patch`,`delete`],ae=e=>ie.includes(e.toLowerCase()),oe={get:{short:`GET`,colorClass:`text-blue`,colorVar:`var(--scalar-color-blue)`,backgroundColor:`bg-blue/10`},post:{short:`POST`,colorClass:`text-green`,colorVar:`var(--scalar-color-green)`,backgroundColor:`bg-green/10`},put:{short:`PUT`,colorClass:`text-orange`,colorVar:`var(--scalar-color-orange)`,backgroundColor:`bg-orange/10`},patch:{short:`PATCH`,colorClass:`text-yellow`,colorVar:`var(--scalar-color-yellow)`,backgroundColor:`bg-yellow/10`},delete:{short:`DEL`,colorClass:`text-red`,colorVar:`var(--scalar-color-red)`,backgroundColor:`bg-red/10`},options:{short:`OPTS`,colorClass:`text-purple`,colorVar:`var(--scalar-color-purple)`,backgroundColor:`bg-purple/10`},head:{short:`HEAD`,colorClass:`text-c-2`,colorVar:`var(--scalar-color-2)`,backgroundColor:`bg-c-2/10`},trace:{short:`TRACE`,colorClass:`text-c-2`,colorVar:`var(--scalar-color-2)`,backgroundColor:`bg-c-2/10`}},se=e=>{let t=e.trim().toLowerCase();return oe[t]??{short:t,color:`text-c-2`,backgroundColor:`bg-c-2`}},ce={100:{name:`Continue`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/100`,color:`var(--scalar-color-blue)`},101:{name:`Switching Protocols`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/101`,color:`var(--scalar-color-blue)`},102:{name:`Processing`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/102`,color:`var(--scalar-color-blue)`},103:{name:`Early Hints`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/103`,color:`var(--scalar-color-blue)`},200:{name:`OK`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/200`,color:`var(--scalar-color-green)`},201:{name:`Created`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/201`,color:`var(--scalar-color-green)`},202:{name:`Accepted`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/202`,color:`var(--scalar-color-green)`},203:{name:`Non-Authoritative Information`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/203`,color:`var(--scalar-color-green)`},204:{name:`No Content`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/204`,color:`var(--scalar-color-green)`},205:{name:`Reset Content`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/205`,color:`var(--scalar-color-green)`},206:{name:`Partial Content`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/206`,color:`var(--scalar-color-green)`},207:{name:`Multi-Status`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/207`,color:`var(--scalar-color-green)`},208:{name:`Already Reported`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/208`,color:`var(--scalar-color-green)`},226:{name:`IM Used`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/226`,color:`var(--scalar-color-green)`},300:{name:`Multiple Choices`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/300`,color:`var(--scalar-color-yellow)`},301:{name:`Moved Permanently`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/301`,color:`var(--scalar-color-yellow)`},302:{name:`Found`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/302`,color:`var(--scalar-color-yellow)`},303:{name:`See Other`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/303`,color:`var(--scalar-color-yellow)`},304:{name:`Not Modified`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/304`,color:`var(--scalar-color-yellow)`},305:{name:`Use Proxy`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/305`,color:`var(--scalar-color-yellow)`},306:{name:`(Unused)`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/306`,color:`var(--scalar-color-yellow)`},307:{name:`Temporary Redirect`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/307`,color:`var(--scalar-color-yellow)`},308:{name:`Permanent Redirect`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/308`,color:`var(--scalar-color-yellow)`},400:{name:`Bad Request`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/400`,color:`var(--scalar-color-red)`},401:{name:`Unauthorized`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/401`,color:`var(--scalar-color-red)`},402:{name:`Payment Required`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/402`,color:`var(--scalar-color-red)`},403:{name:`Forbidden`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/403`,color:`var(--scalar-color-red)`},404:{name:`Not Found`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404`,color:`var(--scalar-color-red)`},405:{name:`Method Not Allowed`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/405`,color:`var(--scalar-color-red)`},406:{name:`Not Acceptable`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/406`,color:`var(--scalar-color-red)`},407:{name:`Proxy Authentication Required`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/407`,color:`var(--scalar-color-red)`},408:{name:`Request Timeout`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/408`,color:`var(--scalar-color-red)`},409:{name:`Conflict`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/409`,color:`var(--scalar-color-red)`},410:{name:`Gone`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/410`,color:`var(--scalar-color-red)`},411:{name:`Length Required`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/411`,color:`var(--scalar-color-red)`},412:{name:`Precondition Failed`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/412`,color:`var(--scalar-color-red)`},413:{name:`Content Too Large`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/413`,color:`var(--scalar-color-red)`},414:{name:`URI Too Long`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/414`,color:`var(--scalar-color-red)`},415:{name:`Unsupported Media Type`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/415`,color:`var(--scalar-color-red)`},416:{name:`Range Not Satisfiable`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/416`,color:`var(--scalar-color-red)`},417:{name:`Expectation Failed`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/417`,color:`var(--scalar-color-red)`},418:{name:`I'm a teapot`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/418`,color:`var(--scalar-color-red)`},421:{name:`Misdirected Request`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/421`,color:`var(--scalar-color-red)`},422:{name:`Unprocessable Content`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/422`,color:`var(--scalar-color-red)`},423:{name:`Locked`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/423`,color:`var(--scalar-color-red)`},424:{name:`Failed Dependency`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/424`,color:`var(--scalar-color-red)`},425:{name:`Too Early`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/425`,color:`var(--scalar-color-red)`},426:{name:`Upgrade Required`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/426`,color:`var(--scalar-color-red)`},428:{name:`Precondition Required`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/428`,color:`var(--scalar-color-red)`},429:{name:`Too Many Requests`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/429`,color:`var(--scalar-color-red)`},431:{name:`Request Header Fields Too Large`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/431`,color:`var(--scalar-color-red)`},451:{name:`Unavailable For Legal Reasons`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/451`,color:`var(--scalar-color-red)`},500:{name:`Internal Server Error`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500`,color:`var(--scalar-color-red)`},501:{name:`Not Implemented`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/501`,color:`var(--scalar-color-red)`},502:{name:`Bad Gateway`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/502`,color:`var(--scalar-color-red)`},503:{name:`Service Unavailable`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503`,color:`var(--scalar-color-red)`},504:{name:`Gateway Timeout`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/504`,color:`var(--scalar-color-red)`},505:{name:`HTTP Version Not Supported`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/505`,color:`var(--scalar-color-red)`},506:{name:`Variant Also Negotiates`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/506`,color:`var(--scalar-color-red)`},507:{name:`Insufficient Storage`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/507`,color:`var(--scalar-color-red)`},508:{name:`Loop Detected`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/508`,color:`var(--scalar-color-red)`},510:{name:`Not Extended`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/510`,color:`var(--scalar-color-red)`},511:{name:`Network Authentication Required`,url:`https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/511`,color:`var(--scalar-color-red)`}},le=[`delete`,`get`,`head`,`options`,`patch`,`post`,`put`,`trace`],ue=Object.freeze(new Set(le)),de=e=>e&&typeof e==`string`?ue.has(e.toLowerCase()):!1,fe=e=>Object.keys(e),b={PROTOCOL:/^(?:https?|ftp|file|mailto|tel|data|wss?)*:\/\//,MULTIPLE_SLASHES:/(?<!:)\/{2,}/g,VARIABLES:/{{((?:[^{}]|{[^{}]*})*)}}/g,PATH:/(?:{)([^{}]+)}(?!})/g,REF_NAME:/\/([^\/]+)$/,TEMPLATE_VARIABLE:/{{\s*([^}\s]+?)\s*}}|{\s*([^}\s]+?)\s*}|:\b[\w.]+\b/g},pe=(e,{includePath:t=!0,includeEnv:n=!0}={})=>[t&&b.PATH,n&&b.VARIABLES].flatMap(t=>t?[...e.matchAll(t)].map(e=>e[1]?.trim()).filter(e=>e!==void 0):[]);function me(e,t){let n=/{{\s*([\w.-]+)\s*}}/g,r=/{\s*([\w.-]+)\s*}/g,i=(e,n)=>typeof t==`function`?t(n):t[n]?.toString()||`{${n}}`;return e.replace(n,i).replace(r,i)}var he=(e,t={})=>e.replace(b.VARIABLES,(e,n)=>t[n]??e),ge=e=>{let t=0,n=0,r=0;if(!e?.length)return n;for(r=0;r<e.length;r++)t=e.charCodeAt(r),n=(n<<5)-n+t,n|=0;return n};function _e(e){return b.PROTOCOL.test(e)?e:`http://${e.replace(/^\//,``)}`}var ve=[`localhost`,`127.0.0.1`,`[::1]`,`0.0.0.0`],ye=[`test`,`example`,`invalid`,`localhost`];function be(e){try{let{hostname:t}=new URL(e);if(ve.includes(t))return!0;let n=t.split(`.`).pop();return!!(n&&ye.includes(n))}catch{return!0}}var x=e=>!(b.PROTOCOL.test(e)||/^[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+(\/|$)/.test(e));function xe(e){try{return!!new URL(e)}catch{return!1}}var Se=(...e)=>{let t={};e.forEach(e=>{let n=Array.from(e.keys());new Set(n).forEach(n=>{let r=e.getAll(n);t[n]=r.length>1?r:r[0]??``})});let n=new URLSearchParams;return Object.entries(t).forEach(([e,t])=>{Array.isArray(t)?t.forEach(t=>n.append(e,t)):n.append(e,t)}),n},S=(e,t)=>!t||e===t?e.trim():e?`${e.trim()}/${t.trim()}`.replace(b.MULTIPLE_SLASHES,`/`):t.trim(),Ce=(e,t,n=new URLSearchParams,r=!1)=>{if(e&&(!x(e)||typeof window<`u`)){let[i=``,a]=(r?e:x(e)?S(window.location.origin,e):_e(e)).split(`?`),o=new URLSearchParams(a||``),[s=``,c]=t.split(`?`),l=new URLSearchParams(c||``),u=e===t?i:S(i,s),d=Se(o,l,n).toString();return d?`${u}?${d}`:u}return t?S(e,t):``},we=(e,{baseUrl:t,basePath:n}={})=>{if(typeof window>`u`&&!t)return e;try{return new URL(e),e}catch{}try{let r=t||window.location.href;return n&&(r=S(t?new URL(t).origin:window.location.origin,n+`/`)),new URL(e,r).toString()}catch{return e}},Te=async(e,t,n)=>{let r=e;for(let e of n){let n=e.hooks?.[t];n&&(r=await n(r)??r)}return r},Ee=(e,t)=>{try{if(!De(e,t))return t??``;let n=new URL(t);return n.href=x(e)?`http://localhost${e}`:e,n.searchParams.append(`scalar_url`,t),x(e)?n.toString().replace(/^http:\/\/localhost/,``):n.toString()}catch{return t??``}},De=(e,t)=>{try{return!e||!t||x(t)?!1:x(e)||be(e)?!0:!be(t)}catch{return!1}},Oe={parse:e=>{let t=te(e,{merge:!0,maxAliasCount:1e4});if(typeof t!=`object`)throw Error(`Invalid YAML object`);return t},parseSafe(e,t){try{return Oe.parse(e)}catch(e){return typeof t==`function`?t(e):t}},stringify:ee},ke={parse:e=>{let t=JSON.parse(e);if(typeof t!=`object`)throw Error(`Invalid JSON object`);return t},parseSafe(e,t){try{return ke.parse(e)}catch(e){return typeof t==`function`?t(e):t}},stringify:e=>JSON.stringify(e)};function Ae(e){let t=e.trim();if(t[0]!==`{`&&t[0]!==`[`)return e;try{return JSON.stringify(JSON.parse(e),null,2)}catch{return e}}var je=e=>{if(typeof e!=`string`)return e;let t=ke.parseSafe(e,null);if(t)return t;if(e.length>0&&[`{`,`[`].includes(e[0]??``))throw Error(`Invalid JSON or YAML`);return Oe.parseSafe(e,e=>{throw Error(e)})},Me=`https://api.scalar.com/request-proxy`,Ne=`https://proxy.scalar.com`;async function Pe(e,t,n,r=!0){t===Me&&(t=Ne);let i=await(n?n(e,void 0):fetch(Ee(t,e)));if(i.status!==200)throw console.error(`[fetchDocument] Failed to fetch the OpenAPI document from ${e} (Status: ${i.status})`),t||console.warn(`[fetchDocument] Tried to fetch the OpenAPI document from ${e} without a proxy. Are the CORS headers configured to allow cross-domain requests? https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS`),Error(`Failed to fetch the OpenAPI document from ${e} (Status: ${i.status})`);return r?Ae(await i.text()):await i.text()}function Fe(e){if(typeof e==`string`)return e.replace(/;.*$/,``).replace(/\/(?!.*vnd\.|fhir\+).*\+/,`/`).trim()}function Ie(e){if(!e)return e;let t={...e};return Object.entries(t).forEach(([e,n])=>{let r=Fe(e);r&&(t[r]=n)}),t}var Le=t({lang:f().optional().catch(void 0),label:f().optional().catch(void 0),source:f()}),Re=t({"x-codeSamples":Le.array().optional().catch(void 0),"x-code-samples":Le.array().optional().catch(void 0),"x-custom-examples":Le.array().optional().catch(void 0)}),ze=f(),Be=t({"x-post-response":ze.optional()}),Ve=t({"x-scalar-sdk-installation":t({lang:f(),source:f().optional().catch(void 0),description:f().optional().catch(void 0)}).array().optional().catch(void 0)}),C={Deprecated:`deprecated`,Experimental:`experimental`,Stable:`stable`};t({"x-scalar-stability":m(Object.values(C)).optional().catch(void 0)});var He=e=>e.deprecated||e[`x-scalar-stability`]===C.Deprecated,Ue=e=>e.deprecated?C.Deprecated:e[`x-scalar-stability`],We=e=>{switch(Ue(e)){case C.Deprecated:return`text-red`;case C.Experimental:return`text-orange`;case C.Stable:return`text-green`;default:return``}};function Ge(e,t,n=!0,r){let i=t.safeParse(e);if(i.success||(console.group(`Schema Error`+(r?` - ${r}`:``)),console.warn(JSON.stringify(i.error.format(),null,2)),console.log(`Received: `,e),console.groupEnd()),n&&!i.success)throw Error(`Zod validation failure`);return i.data}var w=f().min(7).default(()=>o()),Ke=t({enum:_(f()).optional(),default:f().optional(),description:f().optional()}).extend({value:f().optional()}).refine(e=>(Array.isArray(e.enum)&&!e.enum.includes(e.default??``)&&e.enum.length>0&&(e.default=e.enum[0]),Array.isArray(e.enum)&&e.enum.length===0&&delete e.enum,!0)),qe=t({url:f(),description:f().optional(),variables:p(f(),Ke).optional()}).extend({uid:w.brand()}),Je=e=>e?.[`x-internal`]===!0||e?.[`x-scalar-ignore`]===!0;function Ye(e,t){let n=e.split(`.`).reduce((e,t)=>e?.[t],t);return typeof n==`string`?n:JSON.stringify(n)}function Xe(e,t){let n=e,r=new Set;return n=n.replace(b.VARIABLES,(e,n)=>{let i=n.trim();r.add(i);let a=Ye(i,t);return v(a)&&a!==``?a:`{{${i}}}`}),n=n.replace(b.PATH,(e,n)=>{let i=n.trim();if(r.has(i))return`{${i}}`;let a=Ye(i,t);return v(a)&&a!==``?a:`{${i}}`}),n=n.replace(/:\b[\w.]+\b/g,e=>{let n=e.slice(1);if(r.has(n))return e;let i=Ye(n,t);return v(i)&&i!==``?i:e}),n}t({uid:f().brand(),name:f().optional().default(`Default Environment`),color:f().optional().default(`#FFFFFF`),value:f().default(``),isDefault:c().optional()});var Ze={SELECTED_CLIENT:`scalar-reference-selected-client-v2`,AUTH:`scalar-reference-auth`},Qe={AUTH:`scalar-client-auth`,SELECTED_SECURITY_SCHEMES:`scalar-client-selected-security-schemes`},$e=()=>typeof window>`u`?{getItem:()=>null,setItem:()=>null,removeItem:()=>null}:localStorage,T=t({description:f().optional()}),E=t({uid:w.brand(),nameKey:f().optional().default(``)}),et=T.extend({type:g(`apiKey`),name:f().optional().default(``),in:m([`query`,`header`,`cookie`]).optional().default(`header`).catch(`header`)}),tt=t({value:f().default(``)}),nt=et.merge(E).merge(tt),rt=T.extend({type:g(`http`),scheme:f().toLowerCase().pipe(m([`basic`,`bearer`])).optional().default(`basic`),bearerFormat:r([g(`JWT`),f()]).optional().default(`JWT`)}),it=t({username:f().default(``),password:f().default(``),token:f().default(``)}),at=rt.merge(E).merge(it),ot=T.extend({type:g(`openIdConnect`),openIdConnectUrl:f().optional().default(``)}),st=ot.merge(E),ct=f().default(``),lt=f().default(``),D=t({refreshUrl:f().optional().default(``),scopes:p(f(),f().optional().default(``)).optional().default({}).catch({}),selectedScopes:_(f()).optional().default([]),"x-scalar-client-id":f().optional().default(``),token:f().default(``),"x-scalar-security-query":p(f(),f()).optional(),"x-scalar-security-body":p(f(),f()).optional(),"x-tokenName":f().optional()}),ut=typeof window<`u`?window.location.origin+window.location.pathname:``,dt=[`SHA-256`,`plain`,`no`],O=m([`header`,`body`]).optional(),ft=T.extend({type:g(`oauth2`),"x-default-scopes":_(f()).optional(),flows:t({implicit:D.extend({type:g(`implicit`).default(`implicit`),authorizationUrl:ct,"x-scalar-redirect-uri":f().optional().default(ut)}),password:D.extend({type:g(`password`).default(`password`),tokenUrl:lt,clientSecret:f().default(``),username:f().default(``),password:f().default(``),"x-scalar-credentials-location":O}),clientCredentials:D.extend({type:g(`clientCredentials`).default(`clientCredentials`),tokenUrl:lt,clientSecret:f().default(``),"x-scalar-credentials-location":O}),authorizationCode:D.extend({type:g(`authorizationCode`).default(`authorizationCode`),authorizationUrl:ct,"x-usePkce":m(dt).optional().default(`no`),"x-scalar-redirect-uri":f().optional().default(ut),tokenUrl:lt,clientSecret:f().default(``),"x-scalar-credentials-location":O})}).partial().default({implicit:{selectedScopes:[],scopes:{},"x-scalar-client-id":``,refreshUrl:``,token:``,type:`implicit`,authorizationUrl:`http://localhost:8080`,"x-scalar-redirect-uri":ut}})}),pt=ft.merge(E),mt=p(f(),_(f()).optional().default([]));r([et,rt,ft,ot]);var ht=s(`type`,[nt,at,st,pt]).transform(e=>(e.type===`oauth2`&&e[`x-default-scopes`]?.length&&Object.keys(e.flows).forEach(t=>{e.flows[t]?.selectedScopes&&e[`x-default-scopes`]&&(e.flows[t].selectedScopes=[e[`x-default-scopes`]].flat())}),e)),gt=r([f().brand(),f().brand().array()]).array().default([]),_t=r([t({description:f().optional(),default:f().default(``)}),f()]),vt=t({description:f().optional(),color:f().optional(),variables:p(f(),_t)}),yt=p(f(),vt),bt=t({description:f().optional(),example:f().optional()}),xt=p(f(),bt),k=e=>Array.isArray(e)?e.map(e=>typeof e==`object`&&e?k(e):e):Object.fromEntries(Object.entries(e).filter(([e,t])=>t!==void 0).map(([e,t])=>typeof t==`object`&&t?[e,k(t)]:[e,t])),St=t({name:f().optional().nullable().catch(null),identifier:f().optional().catch(void 0),url:f().url().optional().catch(void 0)}).transform(k),Ct=t({name:f().optional(),url:f().url().optional().catch(void 0),email:f().optional().catch(void 0)}).transform(k),wt=t({title:f().catch(`API`),summary:f().optional().catch(void 0),description:f().optional().catch(void 0),termsOfService:f().url().optional().catch(void 0),contact:Ct.optional().catch(void 0),license:St.optional().catch(void 0),version:f().catch(`1.0`)}).merge(Ve).transform(k),A=t({description:f().optional().catch(void 0),url:f()}).transform(k),Tt=t({tagName:f()}).array(),Et=t({type:g(`tag`).optional().default(`tag`),name:f(),description:f().optional().catch(void 0),externalDocs:A.optional(),"x-scalar-children":Tt.default([]).optional(),"x-internal":c().optional(),"x-scalar-ignore":c().optional()}).extend({uid:w.brand(),children:r([f().brand(),f().brand()]).array().default([])}),Dt=t({type:g(`collection`).optional().default(`collection`),openapi:r([f(),g(`3.0.0`),g(`3.1.0`),g(`4.0.0`)]).optional().default(`3.1.0`),jsonSchemaDialect:f().optional(),info:wt.catch({title:`API`,version:`1.0`}),security:_(mt).optional().default([]),externalDocs:A.optional().catch(void 0),components:p(f(),i()).optional(),webhooks:p(f(),i()).optional(),"x-scalar-icon":f().optional().default(`interface-content-folder`),"x-scalar-active-environment":f().optional(),"x-scalar-environments":yt.optional(),"x-scalar-secrets":xt.optional()}),Ot=t({uid:w.brand(),securitySchemes:f().array().default([]),selectedSecuritySchemeUids:gt,selectedServerUid:f().brand().optional(),servers:f().brand().array().default([]),requests:f().brand().array().default([]),tags:f().brand().array().default([]),children:r([f().brand(),f().brand()]).array().default([]),documentUrl:f().optional(),watchMode:c().optional().default(!1),integration:f().nullable().optional(),useCollectionSecurity:c().optional().default(!1),watchModeStatus:m([`IDLE`,`WATCHING`,`ERROR`]).optional().default(`IDLE`)}),kt=Dt.merge(Ot),j;(function(e){e.Deprecated=`deprecated`,e.Experimental=`experimental`,e.Stable=`stable`})(j||={});var At=m([`path`,`query`,`header`,`cookie`]),jt=m([`matrix`,`simple`,`form`,`label`,`spaceDelimited`,`pipeDelimited`,`deepObject`]),Mt=t({in:At,name:f(),description:f().optional(),required:c().optional().default(!1),deprecated:c().optional().default(!1),schema:i().optional(),content:i().optional(),style:jt.optional(),explode:c().optional(),example:i().optional(),examples:r([p(f(),t({value:i().optional(),summary:f().optional(),externalValue:f().optional()})),_(i())]).optional()}),Nt=e=>e[`$ref-value`],M=(e,t=Nt)=>typeof e==`object`&&e&&`$ref`in e?t(e):e,Pt=e=>decodeURI(e.replace(/~1/g,`/`).replace(/~0/g,`~`)),Ft=e=>e.split(`/`).slice(1).map(Pt),N=e=>{if(typeof e!=`object`||!e)return!1;let t=Object.getPrototypeOf(e);return t===Object.prototype||t===null},P=(e,t,n)=>{let[r,i]=e.split(`#`,2);if(r)return n.has(r)?i?i.startsWith(`/`)?`${n.get(r)}${i}`:n.get(`${r}#${i}`):n.get(r):void 0;if(i)return i.startsWith(`/`)?i.slice(1):n.get(`${t}#${i}`)},F=e=>{if(e&&typeof e==`object`&&e.$id&&typeof e.$id==`string`)return e.$id},It=e=>e.join(`/`),I=(e,t=``,n=[],r=new Map,i=new WeakSet)=>{if(typeof e!=`object`||!e||i.has(e))return r;i.add(e);let a=F(e);a&&r.set(a,It(n));let o=a??t;e.$anchor&&typeof e.$anchor==`string`&&r.set(`${o}#${e.$anchor}`,It(n));for(let t in e)typeof e[t]==`object`&&e[t]!==null&&I(e[t],o,[...n,t],r,i);return r};function Lt(e,t){return t.reduce((e,t)=>e.value===void 0||typeof e.value!=`object`||e.value===null?{context:``,value:void 0}:{context:F(e.value)??e.context,value:e.value?.[t]},{context:``,value:e})}function Rt(e,t){return t.reduce((e,t)=>(e[t]===void 0&&(isNaN(Number(t))?e[t]={}:e[t]=[]),e[t]),e)}var zt=Symbol(`isMagicProxy`),Bt=Symbol(`magicProxyTarget`),L=`$ref-value`,R=`$ref`,Vt=(e,t,n={root:e,proxyCache:new WeakMap,cache:new Map,schemas:I(e),currentContext:``})=>{if(!N(e)&&!Array.isArray(e))return e;if(n.proxyCache.has(e))return n.proxyCache.get(e);let r=new Proxy(e,{get(e,r,i){if(r===zt)return!0;if(r===Bt)return e;if(typeof r==`string`&&r.startsWith(`__scalar_`)&&!t?.showInternal)return;let a=Reflect.get(e,R,i),o=F(e);if(r===L&&typeof a==`string`){if(n.cache.has(a))return n.cache.get(a);let e=P(a,o??n.currentContext,n.schemas);if(e===void 0)return;let r=Lt(n.root,Ft(`/${e}`));if(Ht(r.value))return r.value;let i=Vt(r.value,t,{...n,currentContext:r.context});return n.cache.set(a,i),i}let s=Reflect.get(e,r,i);return Ht(s)?s:Vt(s,t,{...n,currentContext:o??n.currentContext})},set(e,r,i,a){let o=Reflect.get(e,R,a);if(typeof r==`string`&&r.startsWith(`__scalar_`)&&!t?.showInternal)return!0;if(r===L&&typeof o==`string`){let t=P(o,F(e)??n.currentContext,n.schemas);if(t===void 0)return;let r=Ft(`/${t}`);if(r.length===0)return!1;let a=()=>Lt(n.root,r.slice(0,-1)).value;return a()===void 0&&(Rt(n.root,r.slice(0,-1)),console.warn(`Trying to set $ref-value for invalid reference: ${o}\n\nPlease fix your input file to fix this issue.`)),a()[r.at(-1)]=i,!0}return Reflect.set(e,r,i,a)},deleteProperty(e,t){return Reflect.deleteProperty(e,t)},has(e,n){return typeof n==`string`&&n.startsWith(`__scalar_`)&&!t?.showInternal?!1:n===L&&R in e?!0:Reflect.has(e,n)},ownKeys(e){let n=Reflect.ownKeys(e).filter(e=>typeof e!=`string`||!(e.startsWith(`__scalar_`)&&!t?.showInternal));return R in e&&!n.includes(L)&&n.push(L),n},getOwnPropertyDescriptor(e,n){if(typeof n==`string`&&n.startsWith(`__scalar_`)&&!t?.showInternal)return;let r=Reflect.get(e,R);return n===L&&typeof r==`string`?{configurable:!0,enumerable:!0,value:void 0,writable:!1}:Reflect.getOwnPropertyDescriptor(e,n)}});return n.proxyCache.set(e,r),r},Ht=e=>typeof e==`object`&&!!e&&e[zt]===!0;function Ut(e){return typeof e!=`object`||!e?e:e[zt]?e[Bt]:e}var z=Symbol(`isOverridesProxy`),Wt=Symbol(`getOverridesTarget`),Gt=(e,t,n={cache:new WeakMap})=>{if(!e||typeof e!=`object`)return e;if(n.cache.has(e))return n.cache.get(e);let{overrides:r}=t??{},i=new Proxy(e,{get(e,t,i){if(t===z)return!0;if(t===Wt)return e;let a=Reflect.get(e,t,i);return Kt(a)?a:N(a)?Gt(a,{overrides:Reflect.get(r??{},t)},n):Reflect.get(r??{},t)??a},set(e,t,n,i){return t===z||t===Wt?!1:r&&Reflect.has(r,t)&&r&&typeof r==`object`?(r[t]=n,!0):Reflect.set(e,t,n,i)}});return n.cache.set(e,i),i},Kt=e=>typeof e==`object`&&!!e&&e[z]===!0;function qt(e){return typeof e==`object`&&e&&e[z]?e[Wt]:e}var Jt=10,Yt=`additionalProperty`,Xt=new Date().toISOString(),Zt=Xt.split(`T`)[0],Qt=Xt.split(`T`)[1].split(`.`)[0],$t={"date-time":Xt,date:Zt,email:`hello@example.com`,hostname:`example.com`,"idn-email":`jane.doe@example.com`,"idn-hostname":`example.com`,ipv4:`127.0.0.1`,ipv6:`51d4:7fab:bfbf:b7d7:b2cb:d4b4:3dad:d998`,"iri-reference":`/entitiy/1`,iri:`https://example.com/entity/123`,"json-pointer":`/nested/objects`,password:`super-secret`,regex:`/[a-z]/`,"relative-json-pointer":`1/nested/objects`,time:Qt,"uri-reference":`../folder`,"uri-template":`https://example.com/{id}`,uri:`https://example.com`,uuid:`123e4567-e89b-12d3-a456-426614174000`,"object-id":`6592008029c8c3e4dc76256c`},en=e=>{if(!(`propertyNames`in e)||!e.propertyNames)return;let t=M(e.propertyNames);if(t&&`enum`in t&&Array.isArray(t.enum)&&t.enum.length>0)return t.enum},tn=(e,t=!1,n=``)=>`type`in e&&e.type===`string`&&`format`in e&&e.format===`binary`?new File([``],`filename`):t&&`format`in e&&e.format?$t[e.format]??n:n,nn=new WeakMap,rn=new WeakMap,an=e=>{if(!e)return;let t=rn.get(e);if(t)return t;if(`required`in e){let t=e.required;if(Array.isArray(t)&&t.length>0){let n=new Set(t);return rn.set(e,n),n}}},B=(e,t)=>(typeof t!=`object`||!t||nn.set(Ut(qt(e)),t),t),on=e=>!!(e.allOf||e.oneOf||e.anyOf),sn=(e,t,n,r)=>{if(r?.omitEmptyAndOptionalProperties!==!0||`type`in e&&(e.type===`object`||e.type===`array`)||on(e)||`examples`in e&&Array.isArray(e.examples)&&e.examples.length>0||`example`in e&&e.example!==void 0||`default`in e&&e.default!==void 0||`const`in e&&e.const!==void 0||`enum`in e&&Array.isArray(e.enum)&&e.enum.length>0)return!1;let i=n??e.title??``,a=an(t);return!(a&&a.has(i))},cn=(e,t)=>Array.isArray(e)&&Array.isArray(t)?[...e,...t]:e&&typeof e==`object`&&t&&typeof t==`object`?{...e,...t}:t,ln=(e,t,n,r)=>{let i={};if(`properties`in e&&e.properties){let a=Object.keys(e.properties),o=a.length;for(let s=0;s<o;s++){let o=a[s],c=M(e.properties[o]);if(!c)continue;let l=t?.xml&&`xml`in c?c.xml?.name:void 0,u=V(c,t,{level:n+1,parentSchema:e,name:o,seen:r});u!==void 0&&(i[l??o]=u)}}if(`patternProperties`in e&&e.patternProperties)for(let a of Object.keys(e.patternProperties)){let o=M(e.patternProperties[a]);o&&(i[a]=V(o,t,{level:n+1,parentSchema:e,name:a,seen:r}))}if(`additionalProperties`in e&&e.additionalProperties!==void 0&&e.additionalProperties!==!1){let a=M(e.additionalProperties),o=e.additionalProperties===!0||typeof e.additionalProperties==`object`&&Object.keys(e.additionalProperties).length===0,s=typeof a==`object`&&`x-additionalPropertiesName`in a&&typeof a[`x-additionalPropertiesName`]==`string`&&a[`x-additionalPropertiesName`].trim().length>0,c=s?void 0:en(e),l=s?a[`x-additionalPropertiesName`].trim():Yt,u=o?`anything`:typeof a==`object`?V(a,t,{level:n+1,seen:r}):`anything`;c&&c.length>0?i[String(c[0])]=u:i[l]=u}if(e.oneOf?.[0])Object.assign(i,V(M(e.oneOf[0]),t,{level:n+1,seen:r}));else if(e.anyOf?.[0])Object.assign(i,V(M(e.anyOf[0]),t,{level:n+1,seen:r}));else if(Array.isArray(e.allOf)&&e.allOf.length>0){let a=i;for(let i of e.allOf){let o=V(M(i),t,{level:n+1,parentSchema:e,seen:r});a=cn(a,o)}a&&typeof a==`object`&&Object.assign(i,a)}if(t?.xml&&`xml`in e&&e.xml?.name&&n===0){let t={};return t[e.xml.name]=i,B(e,t)}return B(e,i)},un=(e,t,n,r)=>{let i=`items`in e?M(e.items):void 0,a=i&&typeof i==`object`&&`xml`in i?i.xml?.name:void 0,o=!!(t?.xml&&`xml`in e&&e.xml?.wrapped&&a);if(e.example!==void 0)return B(e,o?{[a]:e.example}:e.example);if(i&&typeof i==`object`){if(Array.isArray(i.allOf)&&i.allOf.length>0){let s=i.allOf.filter(v),c=M(s[0]);if(c&&typeof c==`object`&&`type`in c&&c.type===`object`){let i=V({type:`object`,allOf:s},t,{level:n+1,parentSchema:e,seen:r});return B(e,o?[{[a]:i}]:[i])}let l=s.map(i=>V(M(i),t,{level:n+1,parentSchema:e,seen:r})).filter(v);return B(e,o?l.map(e=>({[a]:e})):l)}let s=i.anyOf||i.oneOf;if(s&&s.length>0){let i=s[0],c=V(M(i),t,{level:n+1,parentSchema:e,seen:r});return B(e,o?[{[a]:c}]:[c])}}let s=i&&typeof i==`object`&&(`type`in i&&i.type===`object`||`properties`in i),c=i&&typeof i==`object`&&(`type`in i&&i.type===`array`||`items`in i);if(i&&typeof i==`object`&&(`type`in i&&i.type||s||c)){let s=V(i,t,{level:n+1,seen:r});return B(e,o?[{[a]:s}]:[s])}return B(e,[])},dn=(e,t,n)=>{if(`type`in e&&e.type&&!Array.isArray(e.type))switch(e.type){case`string`:return tn(e,t,n??``);case`boolean`:return!0;case`integer`:return`minimum`in e&&typeof e.minimum==`number`?e.minimum:1;case`number`:return`minimum`in e&&typeof e.minimum==`number`?e.minimum:1;case`array`:return[];default:return}},fn=(e,t,n)=>{if(`type`in e&&Array.isArray(e.type)){if(e.type.includes(`null`))return null;let r=e.type[0];if(r)switch(r){case`string`:return tn(e,t,n??``);case`boolean`:return!0;case`integer`:return`minimum`in e&&typeof e.minimum==`number`?e.minimum:1;case`number`:return`minimum`in e&&typeof e.minimum==`number`?e.minimum:1;case`null`:return null;default:return}}},V=(e,t,n)=>{let{level:r=0,parentSchema:i,name:a,seen:o=new WeakSet}=n??{},s=M(e);if(!v(s))return;let c=Ut(qt(s));if(o.has(c))return;if(o.add(c),nn.has(c))return o.delete(c),nn.get(c);if(r>Jt)return o.delete(c),`[Max Depth Exceeded]`;let l=!!t?.emptyString;if(s.deprecated||t?.mode===`write`&&s.readOnly||t?.mode===`read`&&s.writeOnly||sn(s,i,a,t)){o.delete(c);return}if(`x-variable`in s&&s[`x-variable`]){let e=t?.variables?.[s[`x-variable`]];if(e!==void 0)return`type`in s&&(s.type===`number`||s.type===`integer`)?(o.delete(c),B(s,Number(e))):(o.delete(c),B(s,e))}if(Array.isArray(s.examples)&&s.examples.length>0)return o.delete(c),B(s,s.examples[0]);if(s.example!==void 0)return o.delete(c),B(s,s.example);if(s.default!==void 0)return o.delete(c),B(s,s.default);if(s.const!==void 0)return o.delete(c),B(s,s.const);if(Array.isArray(s.enum)&&s.enum.length>0)return o.delete(c),B(s,s.enum[0]);if(`properties`in s||`type`in s&&s.type===`object`){let e=ln(s,t,r,o);return o.delete(c),e}if(`type`in s&&s.type===`array`||`items`in s){let e=un(s,t,r,o);return o.delete(c),e}let u=dn(s,l,t?.emptyString);if(u!==void 0)return o.delete(c),B(s,u);let d=s.oneOf||s.anyOf;if(Array.isArray(d)&&d.length>0){for(let e of d){let n=M(e);if(n&&(!(`type`in n)||n.type!==`null`))return o.delete(c),B(s,V(n,t,{level:r+1,seen:o}))}return o.delete(c),B(s,null)}if(Array.isArray(s.allOf)&&s.allOf.length>0){let e,n=s.allOf;for(let i of n){let n=V(i,t,{level:r+1,parentSchema:s,seen:o});e===void 0?e=n:e&&typeof e==`object`&&n&&typeof n==`object`?e=cn(e,n):n!==void 0&&(e=n)}return o.delete(c),B(s,e??null)}let f=fn(s,l,t?.emptyString);return f===void 0?(o.delete(c),B(s,null)):(o.delete(c),B(s,f))};function pn(e=[],t=[],n,r=!0){return[...t||[],...e||[]].filter(e=>e.in===n).filter(e=>r&&e.required||!r).map(e=>({name:e.name??`Unknown Parameter`,description:e.description??null,value:e.example?e.example:e.schema?V(e.schema,{mode:`write`}):``,required:e.required??!1,enabled:e.required??!1})).sort((e,t)=>e.required&&!t.required?-1:!e.required&&t.required?1:0)}function mn(e,t=!1,n){return Object.entries(e).flatMap(([e,r])=>{let i=n??e;return Array.isArray(r)&&!t?mn(r,!0,e):(typeof r==`object`&&!(r instanceof File)&&r!==null&&(r=JSON.stringify(r)),[{name:i,value:r}])})}var hn=[`application/json`,`application/octet-stream`,`application/x-www-form-urlencoded`,`application/xml`,`multipart/form-data`,`text/plain`];function gn(e,t,n){let r=e.requestBody?.content,i=Ie(r),a=hn.find(e=>!!i?.[e])??(Object.keys(i??{})[0]||`application/json`),o=a.includes(`json`)||a.endsWith(`+json`),s=i?.[a]?.examples??i?.[`application/json`]?.examples,c=s?.[t??Object.keys(s??{})[0]??``];if(c)return{mimeType:a,text:d(`value`in c?c.value:c)};let l=pn(e.parameters??[],[],`body`,!1);if(l.length>0)return{mimeType:`application/json`,text:d(l[0]?.value??``)};let u=pn(e.parameters??[],[],`formData`,!1);if(u.length>0)return{mimeType:`application/x-www-form-urlencoded`,params:u.map(e=>({name:e.name,value:typeof e.value==`string`?e.value:JSON.stringify(e.value)}))};if(!a)return null;let f=i?.[a],p=f?.example?f?.example:void 0;if(o){let e=f?.schema?V(M(f?.schema),{mode:`write`,omitEmptyAndOptionalProperties:n??!1}):null,t=p??e;return{mimeType:a,text:t?typeof t==`string`?t:JSON.stringify(t,null,2):void 0}}if(a===`application/xml`){let e=f?.schema?V(M(f?.schema),{xml:!0,mode:`write`}):null;return{mimeType:a,text:p??re(e)}}if(a===`application/octet-stream`)return{mimeType:a,text:`BINARY`};if(a===`text/plain`){let e=f?.schema?V(M(f?.schema),{xml:!0,mode:`write`}):null;return{mimeType:a,text:p??e??``}}if(a===`multipart/form-data`||a===`application/x-www-form-urlencoded`){let e=f?.schema?V(M(f?.schema),{xml:!0,mode:`write`}):null;return{mimeType:a,params:mn(p??e??{})}}return null}var _n=e=>{let t={};if(e.variables)for(let[n,r]of Object.entries(e.variables))t[n]=r.enum?.filter(e=>typeof e==`string`)??[r.default];return t},H=t({key:f().default(``),value:l().default(``),enabled:c().default(!0),file:h().optional(),description:f().optional(),required:c().optional(),enum:_(f()).optional(),examples:_(h()).optional(),type:r([f(),_(f())]).optional(),format:f().optional(),minimum:n().optional(),maximum:n().optional(),default:h().optional(),nullable:c().optional()}).transform(e=>{let t={...e};return Array.isArray(t.type)&&t.type.includes(`null`)&&(t.nullable=!0),Array.isArray(t.type)&&t.type.length===2&&t.type.includes(`null`)&&(t.type=t.type.find(e=>e!==`null`)),t}),vn=t({url:f(),base64:f().optional()}).nullable();r([t({type:g(`string`),value:f()}),t({type:g(`file`),file:vn})]);var yn=[`json`,`text`,`html`,`javascript`,`xml`,`yaml`,`edn`],bn=[`application/json`,`text/plain`,`text/html`,`application/javascript`,`application/xml`,`application/yaml`,`application/edn`,`application/octet-stream`,`application/x-www-form-urlencoded`,`multipart/form-data`,`binary`],xn=t({raw:t({encoding:m(yn),value:f().default(``),mimeType:f().optional()}).optional(),formData:t({encoding:r([g(`form-data`),g(`urlencoded`)]).default(`form-data`),value:H.array().default([])}).optional(),binary:u(Blob).optional(),activeBody:r([g(`raw`),g(`formData`),g(`binary`)]).default(`raw`)}),Sn=t({encoding:m(bn),content:r([p(f(),h()),f()]),file:vn.optional()}),Cn=t({uid:w.brand(),type:g(`requestExample`).optional().default(`requestExample`),requestUid:f().brand().optional(),name:f().optional().default(`Name`),body:xn.optional().default({activeBody:`raw`}),parameters:t({path:H.array().default([]),query:H.array().default([]),headers:H.array().default([{key:`Accept`,value:`*/*`,enabled:!0}]),cookies:H.array().default([])}).optional().default({path:[],query:[],headers:[{key:`Accept`,value:`*/*`,enabled:!0}],cookies:[]}),serverVariables:p(f(),_(f())).optional()}),U=p(f(),f()).optional(),wn=t({name:f().optional(),body:Sn.optional(),parameters:t({path:U,query:U,headers:U,cookies:U})});function Tn(e){let t=e.schema,n=(()=>{if(e.examples&&!Array.isArray(e.examples)&&fe(e.examples).length>0){let t=Object.entries(e.examples).map(([e,t])=>t.externalValue?t.externalValue:t.value);return{value:t[0],examples:t}}if(v(e.example))return{value:e.example};if(Array.isArray(e.examples)&&e.examples.length>0)return{value:e.examples[0]};if(v(t?.example))return{value:t.example};if(Array.isArray(t?.examples)&&t.examples.length>0)return t?.type===`boolean`?{value:t.default??!1}:{value:t.examples[0]};if(e.content){let t=fe(e.content)[0];if(t){let n=e.content[t];if(n?.examples){let e=Object.keys(n.examples)[0];if(e){let t=n.examples[e];if(v(t?.value))return{value:t.value}}}if(v(n?.example))return{value:n.example}}}return null})(),r=String(n?.value??t?.default??``),i=t?.enum&&t?.type!==`string`?t.enum?.map(String):t?.items?.enum&&t?.type===`array`?t.items.enum.map(String):t?.enum,a=n?.examples||(t?.examples&&t?.type!==`string`?t.examples?.map(String):t?.examples);return Ge({...t,key:e.name,value:r,description:e.description,required:e.required,enabled:!!e.required,enum:i,examples:a},H,!1)||(console.warn(`Example at ${e.name} is invalid.`),H.parse({}))}function En(e,t,n){let r={path:[],query:[],cookie:[],header:[],headers:[{key:`Accept`,value:`*/*`,enabled:!0}]};e.parameters?.forEach(e=>r[e.in].push(Tn(e))),r.header.length>0&&(r.headers=r.header,r.header=[]);let i=r.headers.find(e=>e.key.toLowerCase()===`content-type`),a={activeBody:`raw`};if(e.requestBody||i?.value){let t=gn(e),n=e.requestBody?t?.mimeType:i?.value;(n?.includes(`/json`)||n?.endsWith(`+json`))&&(a.activeBody=`raw`,a.raw={encoding:`json`,mimeType:n,value:t?.text??JSON.stringify({})}),n===`application/xml`&&(a.activeBody=`raw`,a.raw={encoding:`xml`,value:t?.text??``}),n===`application/octet-stream`&&(a.activeBody=`binary`,a.binary=void 0),(n===`application/x-www-form-urlencoded`||n===`multipart/form-data`)&&(a.activeBody=`formData`,a.formData={encoding:n===`application/x-www-form-urlencoded`?`urlencoded`:`form-data`,value:(t?.params||[]).map(e=>e.value instanceof File?{key:e.name,value:`BINARY`,file:e.value,enabled:!0}:{key:e.name,value:e.value||``,enabled:!0})}),n?.startsWith(`text/`)&&(a.activeBody=`raw`,a.raw={encoding:`text`,value:t?.text??``}),t?.mimeType&&!i&&!t.mimeType.startsWith(`multipart/`)&&r.headers.push({key:`Content-Type`,value:t.mimeType,enabled:!0})}let o=n?_n(n):{};return Ge({requestUid:e.uid,parameters:r,name:t,body:a,serverVariables:o},Cn,!1)||(console.warn(`Example at ${e.uid} is invalid.`),Cn.parse({}))}var Dn=[`delete`,`get`,`head`,`options`,`patch`,`post`,`put`,`trace`],On=h(),kn=t({tags:f().array().optional(),summary:f().optional(),description:f().optional(),operationId:f().optional(),security:_(mt).optional(),requestBody:On.optional(),parameters:Mt.array().optional(),externalDocs:A.optional(),deprecated:c().optional(),responses:p(f(),h()).optional(),callbacks:p(f(),p(f(),p(f(),h()))).optional(),"x-scalar-examples":p(f(),wn).optional(),"x-internal":c().optional(),"x-scalar-ignore":c().optional()}),An=t({"x-scalar-stability":m([j.Deprecated,j.Experimental,j.Stable]).optional().catch(void 0)}),jn=t({type:g(`request`).optional().default(`request`),uid:w.brand(),path:f().optional().default(``),method:m(Dn).default(`get`),servers:f().brand().array().default([]),selectedServerUid:f().brand().optional().nullable().default(null),examples:f().brand().array().default([]),selectedSecuritySchemeUids:gt}),Mn=kn.omit({"x-scalar-examples":!0}).merge(Re).merge(An).merge(jn).merge(Be),Nn=(e={})=>{let{delay:t=328,maxWait:n}=e,r=new Map,i=new Map,a=new Map,o=()=>{r.forEach(clearTimeout),i.forEach(clearTimeout),r.clear(),i.clear(),a.clear()},s=e=>{let t=a.get(e),n=r.get(e);n!==void 0&&(clearTimeout(n),r.delete(e));let o=i.get(e);if(o!==void 0&&(clearTimeout(o),i.delete(e)),a.delete(e),t!==void 0)try{t()}catch{}};return{execute:(e,o)=>{a.set(e,o);let c=r.get(e);c!==void 0&&clearTimeout(c),r.set(e,setTimeout(()=>s(e),t)),n!==void 0&&!i.has(e)&&i.set(e,setTimeout(()=>s(e),n))},cleanup:o,flush:e=>{a.has(e)&&s(e)},flushAll:()=>{let e=[...a.keys()];for(let t of e)s(t)}}},{parse:Pn,stringify:Fn}=JSON,{keys:In}=Object,W={BUILDING_REQUEST_FAILED:`An error occurred while building the request`,DEFAULT:`An unknown error has occurred.`,INVALID_URL:`The URL seems to be invalid. Try adding a valid URL.`,INVALID_HEADER:`There is an invalid header present, please double check your params.`,MISSING_FILE:`File uploads are not saved in history, you must re-upload the file.`,REQUEST_ABORTED:`The request has been cancelled`,REQUEST_FAILED:`An error occurred while making the request`,URL_EMPTY:`The address bar input seems to be empty. Try adding a URL.`,ON_BEFORE_REQUEST_FAILED:`onBeforeRequest request hook failed`},Ln=(e,t=W.DEFAULT)=>(console.error(e),e instanceof Error?(e.message=Rn(e.message),e):Error(typeof e==`string`?Rn(e):t)),Rn=e=>e===`Failed to execute 'append' on 'FormData': parameter 2 is not of type 'Blob'.`?W.MISSING_FILE:e===`Failed to construct 'URL': Invalid URL`?W.INVALID_URL:e===`Failed to execute 'fetch' on 'Window': Invalid name`?W.INVALID_HEADER:e;function G(e,t,n=[]){let r={};for(let[i,a]of Object.entries(e)){let e=[...n,i];if(Array.isArray(a)){r[i]=a.map((n,r)=>typeof n==`object`&&!Array.isArray(n)&&n!==null?G(n,t,[...e,r.toString()]):n);continue}if(typeof a==`object`&&a){r[i]=G(a,t,e);continue}r[i]=a}return t(r,n)}var K=`application/json`;function zn(e){let t=e[`x-example`],n=e[`x-examples`];return delete e[`x-example`],delete e[`x-examples`],{xExample:t,xExamples:n}}function q(e){return typeof e==`object`&&!!e&&!Array.isArray(e)&&Object.keys(e).length>0}function Bn(e){return q(e)&&Object.values(e).every(e=>typeof e==`object`&&!!e&&!Array.isArray(e))}function Vn(e){if(typeof e!=`object`||!e)return!0;let t=e;return!(t.allOf||t.oneOf||t.anyOf||t.items||t.$ref||`additionalProperties`in t||[`enum`,`const`,`not`,`format`,`multipleOf`,`maximum`,`exclusiveMaximum`,`minimum`,`exclusiveMinimum`,`maxLength`,`minLength`,`pattern`,`maxItems`,`minItems`,`uniqueItems`,`maxProperties`,`minProperties`,`required`].some(e=>e in t)||typeof t.properties==`object`&&t.properties!==null&&Object.keys(t.properties).length>0)}function Hn(e){let t=Object.keys(e);if(t.some(t=>{let n=e[t];return n?.example!==void 0||n?.examples!==void 0}))for(let n of t){let t=e[n];if(!t)continue;let r=t.example!==void 0||t.examples!==void 0;t.schema!==void 0&&!r&&Object.keys(t).length===1&&Vn(t.schema)&&delete e[n]}}var Un=new Set([`summary`,`description`,`value`,`externalValue`]);function Wn(e){if(typeof e!=`object`||!e)return!1;let t=e,n=`value`in t||`externalValue`in t,r=Object.keys(t).every(e=>Un.has(e));return n&&r}function J(e){return Wn(e)?e:{value:e}}var Gn=/^[a-zA-Z0-9*+.-]+\/[a-zA-Z0-9*+.+-]+$/;function Kn(e){return Gn.test(e)}function qn(e){return Object.entries(e).reduce((e,[t,n])=>(e[t]={value:n},e),{})}var Jn=e=>{switch(e){case`application`:return`clientCredentials`;case`accessCode`:return`authorizationCode`;case`implicit`:return`implicit`;case`password`:return`password`;default:return e}};function Yn(e){let t=e;if(typeof t==`object`&&t&&typeof t.swagger==`string`&&t.swagger?.startsWith(`2.0`))t.openapi=`3.0.4`,delete t.swagger;else return t;if(t.host){let e=Array.isArray(t.schemes)&&t.schemes?.length?t.schemes:[`http`];t.servers=e.map(e=>({url:`${e}://${t.host}${t.basePath??``}`})),delete t.basePath,delete t.schemes,delete t.host}else t.basePath&&(t.servers=[{url:t.basePath}],delete t.basePath);if(t.definitions&&(t.components=Object.assign({},t.components,{schemas:t.definitions}),delete t.definitions,t=G(t,e=>(typeof e.$ref==`string`&&e.$ref.startsWith(`#/definitions/`)&&(e.$ref=e.$ref.replace(/^#\/definitions\//,`#/components/schemas/`)),e))),t=G(t,e=>(e.type===`file`&&(e.type=`string`,e.format=`binary`),e)),Object.hasOwn(t,`parameters`)){t=G(t,e=>{if(typeof e.$ref==`string`&&e.$ref.startsWith(`#/parameters/`)){let n=e.$ref.split(`/`)[2];if(!n)return e;let r=t.parameters&&typeof t.parameters==`object`&&n in t.parameters?t.parameters[n]:void 0;r&&typeof r==`object`&&`in`in r&&(r.in===`body`||r.in===`formData`)?e.$ref=e.$ref.replace(/^#\/parameters\//,`#/components/requestBodies/`):e.$ref=e.$ref.replace(/^#\/parameters\//,`#/components/parameters/`)}return e}),t.components??={};let e={},n={},r=t.parameters&&typeof t.parameters==`object`?t.parameters:{};for(let[i,a]of Object.entries(r))a&&typeof a==`object`&&(`$ref`in a?e[i]=Qn(a):`in`in a&&(a.in===`body`?n[i]=ir(a,t.consumes??[K]):a.in===`formData`?n[i]=ar([a],t.consumes):e[i]=Qn(a)));Object.keys(e).length>0&&(t.components.parameters=e),Object.keys(n).length>0&&(t.components.requestBodies=n),delete t.parameters}if(Object.hasOwn(t,`responses`)&&typeof t.responses==`object`&&t.responses!==null){t=G(t,e=>(typeof e.$ref==`string`&&e.$ref.startsWith(`#/responses/`)&&(e.$ref=e.$ref.replace(/^#\/responses\//,`#/components/responses/`)),e)),t.components??={};let e={},n=t.responses;for(let[r,i]of Object.entries(n))if(i&&typeof i==`object`)if(`$ref`in i)e[r]=i;else{let n=i,a=t.produces??[K];if(n.schema){typeof n.content!=`object`&&(n.content={});for(let e of a)n.content[e]={schema:n.schema};delete n.schema}if(n.examples&&typeof n.examples==`object`){typeof n.content!=`object`&&(n.content={});let e=a[0]??K;for(let[t,r]of Object.entries(n.examples))if(Kn(t))typeof n.content[t]!=`object`&&(n.content[t]={}),n.content[t].example=r;else{typeof n.content[e]!=`object`&&(n.content[e]={});let i=n.content[e];typeof i.examples!=`object`&&(i.examples={}),i.examples[t]=J(r)}delete n.examples}n.content&&typeof n.content==`object`&&Hn(n.content),n.headers&&typeof n.headers==`object`&&(n.headers=Object.entries(n.headers).reduce((e,[t,n])=>n&&typeof n==`object`?{[t]:$n(n),...e}:e,{})),e[r]=n}Object.keys(e).length>0&&(t.components.responses=e),delete t.responses}if(typeof t.paths==`object`){for(let e in t.paths)if(Object.hasOwn(t.paths,e)){let n=t.paths&&typeof t.paths==`object`&&e in t.paths?t.paths[e]:void 0;if(!n||typeof n!=`object`)continue;let r;for(let e in n)if(e===`parameters`&&Object.hasOwn(n,e)){let e=or(n.parameters,t.consumes??[K]);n.parameters=e.parameters,r=e.requestBody}else if(Object.hasOwn(n,e)){let i=n[e];if(r&&(i.requestBody=r),i.parameters){let e=or(i.parameters,i.consumes??t.consumes??[K]);i.parameters=e.parameters,e.requestBody&&(i.requestBody=e.requestBody)}if(delete i.consumes,i.responses){for(let e in i.responses)if(Object.hasOwn(i.responses,e)){let n=i.responses[e];if(n.headers&&typeof n.headers==`object`&&(n.headers=Object.entries(n.headers).reduce((e,[t,n])=>n&&typeof n==`object`?{[t]:$n(n),...e}:e,{})),n.schema){let e=t.produces??i.produces??[K];typeof n.content!=`object`&&(n.content={});for(let t of e)n.content[t]={schema:n.schema};delete n.schema}if(n.examples&&typeof n.examples==`object`){typeof n.content!=`object`&&(n.content={});let e=(t.produces??i.produces??[K])[0]??K;for(let[t,r]of Object.entries(n.examples))if(Kn(t))typeof n.content[t]!=`object`&&(n.content[t]={}),n.content[t].example=r;else{typeof n.content[e]!=`object`&&(n.content[e]={});let i=n.content[e];typeof i.examples!=`object`&&(i.examples={}),i.examples[t]=J(r)}delete n.examples}n.content&&typeof n.content==`object`&&Hn(n.content)}}delete i.produces,i.parameters?.length===0&&delete i.parameters}}}if(t.securityDefinitions){(typeof t.components!=`object`||t.components===null)&&(t.components={}),t.components&&typeof t.components==`object`&&Object.assign(t.components,{securitySchemes:{}});for(let[e,n]of Object.entries(t.securityDefinitions))if(typeof n==`object`)if(`type`in n&&n.type===`oauth2`){let{flow:r,authorizationUrl:i,tokenUrl:a,scopes:o}=n;t.components&&typeof t.components==`object`&&`securitySchemes`in t.components&&t.components.securitySchemes&&Object.assign(t.components.securitySchemes,{[e]:{type:`oauth2`,flows:{[Jn(r||`implicit`)]:Object.assign({},i&&{authorizationUrl:i},a&&{tokenUrl:a},o&&{scopes:o})}}})}else `type`in n&&n.type===`basic`?t.components&&typeof t.components==`object`&&`securitySchemes`in t.components&&t.components.securitySchemes&&Object.assign(t.components.securitySchemes,{[e]:{type:`http`,scheme:`basic`}}):t.components&&typeof t.components==`object`&&`securitySchemes`in t.components&&t.components.securitySchemes&&Object.assign(t.components.securitySchemes,{[e]:n});delete t.securityDefinitions}return delete t.consumes,delete t.produces,t}function Xn(e){return[`type`,`format`,`default`,`items`,`maximum`,`exclusiveMaximum`,`minimum`,`exclusiveMinimum`,`maxLength`,`minLength`,`pattern`,`maxItems`,`minItems`,`uniqueItems`,`enum`,`multipleOf`].reduce((t,n)=>(Object.hasOwn(e,n)&&(t[n]=e[n],delete e[n]),t),{})}function Zn(e){if(e===`formData`)throw Error(`Encountered a formData parameter which should have been filtered out by the caller`);if(e===`body`)throw Error(`Encountered a body parameter which should have been filtered out by the caller`);return e}function Qn(e){if(Object.hasOwn(e,`$ref`)&&typeof e.$ref==`string`)return{$ref:e.$ref};let t=rr(e),n=Xn(e),{xExample:r,xExamples:i}=zn(e);if(q(r)?e.examples=qn(r):q(i)&&(e.examples=Object.entries(i).reduce((e,[t,n])=>(e[t]=J(n),e),{})),delete e.collectionFormat,delete e.default,!e.in)throw Error(`Parameter object must have an "in" property`);return{schema:n,...t,...e,in:Zn(e.in)}}function $n(e){if(Object.hasOwn(e,`$ref`)&&typeof e.$ref==`string`)return{$ref:e.$ref};let t=Xn(e);return{...e,schema:t}}var er={ssv:{style:`spaceDelimited`,explode:!1},pipes:{style:`pipeDelimited`,explode:!1},multi:{style:`form`,explode:!0},csv:{style:`form`,explode:!1},tsv:{}},tr={ssv:{},pipes:{},multi:{},csv:{style:`simple`,explode:!1},tsv:{}},nr={header:tr,query:er,path:tr};function rr(e){if(e.type!==`array`||!(e.in===`query`||e.in===`path`||e.in===`header`))return{};let t=e.collectionFormat??`csv`;return e.in in nr&&t in nr[e.in]?nr[e.in][t]:{}}function ir(e,t){let{xExample:n,xExamples:r}=zn(e);delete e.name,delete e.in;let{schema:i,...a}=e,o={content:{},...a};if(o.content)for(let e of t){if(o.content[e]={schema:i},q(r)&&e in r){let t=r[e];q(t)&&Object.values(t).every(e=>Wn(e))?o.content[e].examples=t:Bn(t)?o.content[e].examples=Object.entries(t).reduce((e,[t,n])=>(e[t]=J(n),e),{}):o.content[e].examples={default:J(t)}}else q(r)&&!Object.keys(r).some(Kn)&&(o.content[e].examples=Object.entries(r).reduce((e,[t,n])=>(e[t]=J(n),e),{}));!o.content[e].examples&&q(n)&&e in n&&(o.content[e].example=n[e])}return o}function ar(e,t=[`multipart/form-data`]){let n={content:{}},r=t.filter(e=>e===`multipart/form-data`||e===`application/x-www-form-urlencoded`),i=r.length>0?r:[`multipart/form-data`];if(n.content)for(let t of i){n.content[t]={schema:{type:`object`,properties:{},required:[]}};let r=n.content?.[t];if(r?.schema&&typeof r.schema==`object`&&`properties`in r.schema)for(let t of e)t.name&&r.schema.properties&&(r.schema.properties[t.name]={type:t.type,description:t.description,...t.format?{format:t.format}:{}},t.required&&Array.isArray(r.schema.required)&&r.schema.required.push(t.name))}return n}function or(e,t){let n={parameters:e.filter(e=>!(e.in===`body`||e.in===`formData`)).map(e=>Qn(e))},r=structuredClone(e.find(e=>e.in===`body`)??{});r&&Object.keys(r).length&&(n.requestBody=ir(r,t));let i=e.filter(e=>e.in===`formData`);if(i.length>0){let e=ar(i,t);typeof n.requestBody==`object`?n.requestBody={...n.requestBody,content:{...n.requestBody.content,...e.content}}:n.requestBody=e,typeof n.requestBody!=`object`&&(n.requestBody={content:{}})}return n}var sr=new Set([`properties`,`items`,`allOf`,`anyOf`,`oneOf`,`not`,`additionalProperties`,`schema`]);function cr(e){return e?!!(e.some(e=>sr.has(e))||e.some(e=>e.endsWith(`Schema`))||e.length>=2&&e[0]===`components`&&e[1]===`schemas`):!1}function lr(e){let t=e;return t===null||typeof t.openapi!=`string`||!t.openapi.startsWith(`3.0`)?t:(t.openapi=`3.1.1`,t=G(t,ur),t)}var ur=(e,t)=>{e.type!==void 0&&e.nullable===!0&&(e.type=[e.type,`null`],delete e.nullable),e.exclusiveMinimum===!0?(e.exclusiveMinimum=e.minimum,delete e.minimum):e.exclusiveMinimum===!1&&delete e.exclusiveMinimum,e.exclusiveMaximum===!0?(e.exclusiveMaximum=e.maximum,delete e.maximum):e.exclusiveMaximum===!1&&delete e.exclusiveMaximum;let n=t?.some((e,n)=>e===`examples`&&n>0?t[n-1]!==`properties`:!1);if(e.example!==void 0&&!n&&(cr(t)?e.examples=[e.example]:e.examples={default:{value:e.example}},delete e.example),e.type===`object`&&e.properties!==void 0&&(t?.slice(0,-1))?.some((e,n)=>e===`content`&&t?.[n+1]===`multipart/form-data`)&&e.properties!==null)for(let t of Object.values(e.properties))typeof t==`object`&&t&&`type`in t&&`format`in t&&t.type===`string`&&t.format===`binary`&&(t.contentMediaType=`application/octet-stream`,delete t.format);if(t?.includes(`content`)&&t?.includes(`application/octet-stream`))return{};let{format:r,...i}=e;if(e.type===`string`){if(e.format===`binary`)return{...i,type:`string`,contentMediaType:`application/octet-stream`};if(e.format===`base64`)return{...i,type:`string`,contentEncoding:`base64`};if(e.format===`byte`){let e=(t?.slice(0,-1))?.find((e,n)=>t?.[n-1]===`content`);return{...i,type:`string`,contentEncoding:`base64`,contentMediaType:e}}}return e[`x-webhooks`]!==void 0&&(e.webhooks=e[`x-webhooks`],delete e[`x-webhooks`]),e},dr={title:`A JSON Schema for Swagger 2.0 API.`,id:`http://swagger.io/v2/schema.json#`,$schema:`http://json-schema.org/draft-04/schema#`,type:`object`,required:[`swagger`,`info`,`paths`],additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{swagger:{type:`string`,enum:[`2.0`],description:`The Swagger version of this document.`},info:{$ref:`#/definitions/info`},host:{type:`string`,pattern:`^[^{}/ :\\\\]+(?::\\d+)?$`,description:`The host (name or ip) of the API. Example: 'swagger.io'`},basePath:{type:`string`,pattern:`^/`,description:`The base path to the API. Example: '/api'.`},schemes:{$ref:`#/definitions/schemesList`},consumes:{description:`A list of MIME types accepted by the API.`,allOf:[{$ref:`#/definitions/mediaTypeList`}]},produces:{description:`A list of MIME types the API can produce.`,allOf:[{$ref:`#/definitions/mediaTypeList`}]},paths:{$ref:`#/definitions/paths`},definitions:{$ref:`#/definitions/definitions`},parameters:{$ref:`#/definitions/parameterDefinitions`},responses:{$ref:`#/definitions/responseDefinitions`},security:{$ref:`#/definitions/security`},securityDefinitions:{$ref:`#/definitions/securityDefinitions`},tags:{type:`array`,items:{$ref:`#/definitions/tag`},uniqueItems:!0},externalDocs:{$ref:`#/definitions/externalDocs`}},definitions:{info:{type:`object`,description:`General information about the API.`,required:[`version`,`title`],additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{title:{type:`string`,description:`A unique and precise title of the API.`},version:{type:`string`,description:`A semantic version number of the API.`},description:{type:`string`,description:`A longer description of the API. Should be different from the title.  GitHub Flavored Markdown is allowed.`},termsOfService:{type:`string`,description:`The terms of service for the API.`},contact:{$ref:`#/definitions/contact`},license:{$ref:`#/definitions/license`}}},contact:{type:`object`,description:`Contact information for the owners of the API.`,additionalProperties:!1,properties:{name:{type:`string`,description:`The identifying name of the contact person/organization.`},url:{type:`string`,description:`The URL pointing to the contact information.`,format:`uri`},email:{type:`string`,description:`The email address of the contact person/organization.`,format:`email`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},license:{type:`object`,required:[`name`],additionalProperties:!1,properties:{name:{type:`string`,description:`The name of the license type. It's encouraged to use an OSI compatible license.`},url:{type:`string`,description:`The URL pointing to the license.`,format:`uri`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},paths:{type:`object`,description:`Relative paths to the individual endpoints. They must be relative to the 'basePath'.`,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`},"^/":{oneOf:[{$ref:`#/definitions/pathItem`},{$ref:`#/definitions/jsonReference`}]}},additionalProperties:!1},definitions:{type:`object`,additionalProperties:{$ref:`#/definitions/schema`},description:`One or more JSON objects describing the schemas being consumed and produced by the API.`},parameterDefinitions:{type:`object`,additionalProperties:{$ref:`#/definitions/parameter`},description:`One or more JSON representations for parameters`},responseDefinitions:{type:`object`,additionalProperties:{$ref:`#/definitions/response`},description:`One or more JSON representations for responses`},externalDocs:{type:`object`,additionalProperties:!1,description:`information about external documentation`,required:[`url`],properties:{description:{type:`string`},url:{type:`string`,format:`uri`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},examples:{type:`object`,additionalProperties:!0},mimeType:{type:`string`,description:`The MIME type of the HTTP message.`},operation:{type:`object`,required:[`responses`],additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{tags:{type:`array`,items:{type:`string`},uniqueItems:!0},summary:{type:`string`,description:`A brief summary of the operation.`},description:{type:`string`,description:`A longer description of the operation, GitHub Flavored Markdown is allowed.`},externalDocs:{$ref:`#/definitions/externalDocs`},operationId:{type:`string`,description:`A unique identifier of the operation.`},produces:{description:`A list of MIME types the API can produce.`,allOf:[{$ref:`#/definitions/mediaTypeList`}]},consumes:{description:`A list of MIME types the API can consume.`,allOf:[{$ref:`#/definitions/mediaTypeList`}]},parameters:{$ref:`#/definitions/parametersList`},responses:{$ref:`#/definitions/responses`},schemes:{$ref:`#/definitions/schemesList`},deprecated:{type:`boolean`,default:!1},security:{$ref:`#/definitions/security`}}},pathItem:{type:`object`,additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{$ref:{type:`string`},get:{$ref:`#/definitions/operation`},put:{$ref:`#/definitions/operation`},post:{$ref:`#/definitions/operation`},delete:{$ref:`#/definitions/operation`},options:{$ref:`#/definitions/operation`},head:{$ref:`#/definitions/operation`},patch:{$ref:`#/definitions/operation`},parameters:{$ref:`#/definitions/parametersList`}}},responses:{type:`object`,description:`Response objects names can either be any valid HTTP status code or 'default'.`,minProperties:1,additionalProperties:!1,patternProperties:{"^([0-9]{3})$|^(default)$":{$ref:`#/definitions/responseValue`},"^x-":{$ref:`#/definitions/vendorExtension`}},not:{type:`object`,additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}}},responseValue:{oneOf:[{$ref:`#/definitions/response`},{$ref:`#/definitions/jsonReference`}]},response:{type:`object`,required:[`description`],properties:{description:{type:`string`},schema:{oneOf:[{$ref:`#/definitions/schema`},{$ref:`#/definitions/fileSchema`}]},headers:{$ref:`#/definitions/headers`},examples:{$ref:`#/definitions/examples`}},additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},headers:{type:`object`,additionalProperties:{$ref:`#/definitions/header`}},header:{type:`object`,additionalProperties:!1,required:[`type`],properties:{type:{type:`string`,enum:[`string`,`number`,`integer`,`boolean`,`array`]},format:{type:`string`},items:{$ref:`#/definitions/primitivesItems`},collectionFormat:{$ref:`#/definitions/collectionFormat`},default:{$ref:`#/definitions/default`},maximum:{$ref:`#/definitions/maximum`},exclusiveMaximum:{$ref:`#/definitions/exclusiveMaximum`},minimum:{$ref:`#/definitions/minimum`},exclusiveMinimum:{$ref:`#/definitions/exclusiveMinimum`},maxLength:{$ref:`#/definitions/maxLength`},minLength:{$ref:`#/definitions/minLength`},pattern:{$ref:`#/definitions/pattern`},maxItems:{$ref:`#/definitions/maxItems`},minItems:{$ref:`#/definitions/minItems`},uniqueItems:{$ref:`#/definitions/uniqueItems`},enum:{$ref:`#/definitions/enum`},multipleOf:{$ref:`#/definitions/multipleOf`},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},vendorExtension:{description:`Any property starting with x- is valid.`,additionalProperties:!0,additionalItems:!0},bodyParameter:{type:`object`,required:[`name`,`in`,`schema`],patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{description:{type:`string`,description:`A brief description of the parameter. This could contain examples of use.  GitHub Flavored Markdown is allowed.`},name:{type:`string`,description:`The name of the parameter.`},in:{type:`string`,description:`Determines the location of the parameter.`,enum:[`body`]},required:{type:`boolean`,description:`Determines whether or not this parameter is required or optional.`,default:!1},schema:{$ref:`#/definitions/schema`}},additionalProperties:!1},headerParameterSubSchema:{additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{required:{type:`boolean`,description:`Determines whether or not this parameter is required or optional.`,default:!1},in:{type:`string`,description:`Determines the location of the parameter.`,enum:[`header`]},description:{type:`string`,description:`A brief description of the parameter. This could contain examples of use.  GitHub Flavored Markdown is allowed.`},name:{type:`string`,description:`The name of the parameter.`},type:{type:`string`,enum:[`string`,`number`,`boolean`,`integer`,`array`]},format:{type:`string`},items:{$ref:`#/definitions/primitivesItems`},collectionFormat:{$ref:`#/definitions/collectionFormat`},default:{$ref:`#/definitions/default`},maximum:{$ref:`#/definitions/maximum`},exclusiveMaximum:{$ref:`#/definitions/exclusiveMaximum`},minimum:{$ref:`#/definitions/minimum`},exclusiveMinimum:{$ref:`#/definitions/exclusiveMinimum`},maxLength:{$ref:`#/definitions/maxLength`},minLength:{$ref:`#/definitions/minLength`},pattern:{$ref:`#/definitions/pattern`},maxItems:{$ref:`#/definitions/maxItems`},minItems:{$ref:`#/definitions/minItems`},uniqueItems:{$ref:`#/definitions/uniqueItems`},enum:{$ref:`#/definitions/enum`},multipleOf:{$ref:`#/definitions/multipleOf`}}},queryParameterSubSchema:{additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{required:{type:`boolean`,description:`Determines whether or not this parameter is required or optional.`,default:!1},in:{type:`string`,description:`Determines the location of the parameter.`,enum:[`query`]},description:{type:`string`,description:`A brief description of the parameter. This could contain examples of use.  GitHub Flavored Markdown is allowed.`},name:{type:`string`,description:`The name of the parameter.`},allowEmptyValue:{type:`boolean`,default:!1,description:`allows sending a parameter by name only or with an empty value.`},type:{type:`string`,enum:[`string`,`number`,`boolean`,`integer`,`array`]},format:{type:`string`},items:{$ref:`#/definitions/primitivesItems`},collectionFormat:{$ref:`#/definitions/collectionFormatWithMulti`},default:{$ref:`#/definitions/default`},maximum:{$ref:`#/definitions/maximum`},exclusiveMaximum:{$ref:`#/definitions/exclusiveMaximum`},minimum:{$ref:`#/definitions/minimum`},exclusiveMinimum:{$ref:`#/definitions/exclusiveMinimum`},maxLength:{$ref:`#/definitions/maxLength`},minLength:{$ref:`#/definitions/minLength`},pattern:{$ref:`#/definitions/pattern`},maxItems:{$ref:`#/definitions/maxItems`},minItems:{$ref:`#/definitions/minItems`},uniqueItems:{$ref:`#/definitions/uniqueItems`},enum:{$ref:`#/definitions/enum`},multipleOf:{$ref:`#/definitions/multipleOf`}}},formDataParameterSubSchema:{additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{required:{type:`boolean`,description:`Determines whether or not this parameter is required or optional.`,default:!1},in:{type:`string`,description:`Determines the location of the parameter.`,enum:[`formData`]},description:{type:`string`,description:`A brief description of the parameter. This could contain examples of use.  GitHub Flavored Markdown is allowed.`},name:{type:`string`,description:`The name of the parameter.`},allowEmptyValue:{type:`boolean`,default:!1,description:`allows sending a parameter by name only or with an empty value.`},type:{type:`string`,enum:[`string`,`number`,`boolean`,`integer`,`array`,`file`]},format:{type:`string`},items:{$ref:`#/definitions/primitivesItems`},collectionFormat:{$ref:`#/definitions/collectionFormatWithMulti`},default:{$ref:`#/definitions/default`},maximum:{$ref:`#/definitions/maximum`},exclusiveMaximum:{$ref:`#/definitions/exclusiveMaximum`},minimum:{$ref:`#/definitions/minimum`},exclusiveMinimum:{$ref:`#/definitions/exclusiveMinimum`},maxLength:{$ref:`#/definitions/maxLength`},minLength:{$ref:`#/definitions/minLength`},pattern:{$ref:`#/definitions/pattern`},maxItems:{$ref:`#/definitions/maxItems`},minItems:{$ref:`#/definitions/minItems`},uniqueItems:{$ref:`#/definitions/uniqueItems`},enum:{$ref:`#/definitions/enum`},multipleOf:{$ref:`#/definitions/multipleOf`}}},pathParameterSubSchema:{additionalProperties:!1,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},required:[`required`],properties:{required:{type:`boolean`,enum:[!0],description:`Determines whether or not this parameter is required or optional.`},in:{type:`string`,description:`Determines the location of the parameter.`,enum:[`path`]},description:{type:`string`,description:`A brief description of the parameter. This could contain examples of use.  GitHub Flavored Markdown is allowed.`},name:{type:`string`,description:`The name of the parameter.`},type:{type:`string`,enum:[`string`,`number`,`boolean`,`integer`,`array`]},format:{type:`string`},items:{$ref:`#/definitions/primitivesItems`},collectionFormat:{$ref:`#/definitions/collectionFormat`},default:{$ref:`#/definitions/default`},maximum:{$ref:`#/definitions/maximum`},exclusiveMaximum:{$ref:`#/definitions/exclusiveMaximum`},minimum:{$ref:`#/definitions/minimum`},exclusiveMinimum:{$ref:`#/definitions/exclusiveMinimum`},maxLength:{$ref:`#/definitions/maxLength`},minLength:{$ref:`#/definitions/minLength`},pattern:{$ref:`#/definitions/pattern`},maxItems:{$ref:`#/definitions/maxItems`},minItems:{$ref:`#/definitions/minItems`},uniqueItems:{$ref:`#/definitions/uniqueItems`},enum:{$ref:`#/definitions/enum`},multipleOf:{$ref:`#/definitions/multipleOf`}}},nonBodyParameter:{type:`object`,required:[`name`,`in`,`type`],oneOf:[{$ref:`#/definitions/headerParameterSubSchema`},{$ref:`#/definitions/formDataParameterSubSchema`},{$ref:`#/definitions/queryParameterSubSchema`},{$ref:`#/definitions/pathParameterSubSchema`}]},parameter:{oneOf:[{$ref:`#/definitions/bodyParameter`},{$ref:`#/definitions/nonBodyParameter`}]},schema:{type:`object`,description:`A deterministic version of a JSON Schema object.`,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},properties:{$ref:{type:`string`},format:{type:`string`},title:{$ref:`http://json-schema.org/draft-04/schema#/properties/title`},description:{$ref:`http://json-schema.org/draft-04/schema#/properties/description`},default:{$ref:`http://json-schema.org/draft-04/schema#/properties/default`},multipleOf:{$ref:`http://json-schema.org/draft-04/schema#/properties/multipleOf`},maximum:{$ref:`http://json-schema.org/draft-04/schema#/properties/maximum`},exclusiveMaximum:{$ref:`http://json-schema.org/draft-04/schema#/properties/exclusiveMaximum`},minimum:{$ref:`http://json-schema.org/draft-04/schema#/properties/minimum`},exclusiveMinimum:{$ref:`http://json-schema.org/draft-04/schema#/properties/exclusiveMinimum`},maxLength:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveInteger`},minLength:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveIntegerDefault0`},pattern:{$ref:`http://json-schema.org/draft-04/schema#/properties/pattern`},maxItems:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveInteger`},minItems:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveIntegerDefault0`},uniqueItems:{$ref:`http://json-schema.org/draft-04/schema#/properties/uniqueItems`},maxProperties:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveInteger`},minProperties:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveIntegerDefault0`},required:{$ref:`http://json-schema.org/draft-04/schema#/definitions/stringArray`},enum:{$ref:`http://json-schema.org/draft-04/schema#/properties/enum`},additionalProperties:{anyOf:[{$ref:`#/definitions/schema`},{type:`boolean`}],default:{}},type:{$ref:`http://json-schema.org/draft-04/schema#/properties/type`},items:{anyOf:[{$ref:`#/definitions/schema`},{type:`array`,minItems:1,items:{$ref:`#/definitions/schema`}}],default:{}},allOf:{type:`array`,minItems:1,items:{$ref:`#/definitions/schema`}},properties:{type:`object`,additionalProperties:{$ref:`#/definitions/schema`},default:{}},discriminator:{type:`string`},readOnly:{type:`boolean`,default:!1},xml:{$ref:`#/definitions/xml`},externalDocs:{$ref:`#/definitions/externalDocs`},example:{}},additionalProperties:!1},fileSchema:{type:`object`,description:`A deterministic version of a JSON Schema object.`,patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}},required:[`type`],properties:{format:{type:`string`},title:{$ref:`http://json-schema.org/draft-04/schema#/properties/title`},description:{$ref:`http://json-schema.org/draft-04/schema#/properties/description`},default:{$ref:`http://json-schema.org/draft-04/schema#/properties/default`},required:{$ref:`http://json-schema.org/draft-04/schema#/definitions/stringArray`},type:{type:`string`,enum:[`file`]},readOnly:{type:`boolean`,default:!1},externalDocs:{$ref:`#/definitions/externalDocs`},example:{}},additionalProperties:!1},primitivesItems:{type:`object`,additionalProperties:!1,properties:{type:{type:`string`,enum:[`string`,`number`,`integer`,`boolean`,`array`]},format:{type:`string`},items:{$ref:`#/definitions/primitivesItems`},collectionFormat:{$ref:`#/definitions/collectionFormat`},default:{$ref:`#/definitions/default`},maximum:{$ref:`#/definitions/maximum`},exclusiveMaximum:{$ref:`#/definitions/exclusiveMaximum`},minimum:{$ref:`#/definitions/minimum`},exclusiveMinimum:{$ref:`#/definitions/exclusiveMinimum`},maxLength:{$ref:`#/definitions/maxLength`},minLength:{$ref:`#/definitions/minLength`},pattern:{$ref:`#/definitions/pattern`},maxItems:{$ref:`#/definitions/maxItems`},minItems:{$ref:`#/definitions/minItems`},uniqueItems:{$ref:`#/definitions/uniqueItems`},enum:{$ref:`#/definitions/enum`},multipleOf:{$ref:`#/definitions/multipleOf`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},security:{type:`array`,items:{$ref:`#/definitions/securityRequirement`},uniqueItems:!0},securityRequirement:{type:`object`,additionalProperties:{type:`array`,items:{type:`string`},uniqueItems:!0}},xml:{type:`object`,additionalProperties:!1,properties:{name:{type:`string`},namespace:{type:`string`},prefix:{type:`string`},attribute:{type:`boolean`,default:!1},wrapped:{type:`boolean`,default:!1}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},tag:{type:`object`,additionalProperties:!1,required:[`name`],properties:{name:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/definitions/externalDocs`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},securityDefinitions:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/basicAuthenticationSecurity`},{$ref:`#/definitions/apiKeySecurity`},{$ref:`#/definitions/oauth2ImplicitSecurity`},{$ref:`#/definitions/oauth2PasswordSecurity`},{$ref:`#/definitions/oauth2ApplicationSecurity`},{$ref:`#/definitions/oauth2AccessCodeSecurity`}]}},basicAuthenticationSecurity:{type:`object`,additionalProperties:!1,required:[`type`],properties:{type:{type:`string`,enum:[`basic`]},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},apiKeySecurity:{type:`object`,additionalProperties:!1,required:[`type`,`name`,`in`],properties:{type:{type:`string`,enum:[`apiKey`]},name:{type:`string`},in:{type:`string`,enum:[`header`,`query`]},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},oauth2ImplicitSecurity:{type:`object`,additionalProperties:!1,required:[`type`,`flow`,`authorizationUrl`],properties:{type:{type:`string`,enum:[`oauth2`]},flow:{type:`string`,enum:[`implicit`]},scopes:{$ref:`#/definitions/oauth2Scopes`},authorizationUrl:{type:`string`,format:`uri`},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},oauth2PasswordSecurity:{type:`object`,additionalProperties:!1,required:[`type`,`flow`,`tokenUrl`],properties:{type:{type:`string`,enum:[`oauth2`]},flow:{type:`string`,enum:[`password`]},scopes:{$ref:`#/definitions/oauth2Scopes`},tokenUrl:{type:`string`,format:`uri`},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},oauth2ApplicationSecurity:{type:`object`,additionalProperties:!1,required:[`type`,`flow`,`tokenUrl`],properties:{type:{type:`string`,enum:[`oauth2`]},flow:{type:`string`,enum:[`application`]},scopes:{$ref:`#/definitions/oauth2Scopes`},tokenUrl:{type:`string`,format:`uri`},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},oauth2AccessCodeSecurity:{type:`object`,additionalProperties:!1,required:[`type`,`flow`,`authorizationUrl`,`tokenUrl`],properties:{type:{type:`string`,enum:[`oauth2`]},flow:{type:`string`,enum:[`accessCode`]},scopes:{$ref:`#/definitions/oauth2Scopes`},authorizationUrl:{type:`string`,format:`uri`},tokenUrl:{type:`string`,format:`uri`},description:{type:`string`}},patternProperties:{"^x-":{$ref:`#/definitions/vendorExtension`}}},oauth2Scopes:{type:`object`,additionalProperties:{type:`string`}},mediaTypeList:{type:`array`,items:{$ref:`#/definitions/mimeType`},uniqueItems:!0},parametersList:{type:`array`,description:`The parameters needed to send a valid API call.`,additionalItems:!1,items:{oneOf:[{$ref:`#/definitions/parameter`},{$ref:`#/definitions/jsonReference`}]},uniqueItems:!0},schemesList:{type:`array`,description:`The transfer protocol of the API.`,items:{type:`string`,enum:[`http`,`https`,`ws`,`wss`]},uniqueItems:!0},collectionFormat:{type:`string`,enum:[`csv`,`ssv`,`tsv`,`pipes`],default:`csv`},collectionFormatWithMulti:{type:`string`,enum:[`csv`,`ssv`,`tsv`,`pipes`,`multi`],default:`csv`},title:{$ref:`http://json-schema.org/draft-04/schema#/properties/title`},description:{$ref:`http://json-schema.org/draft-04/schema#/properties/description`},default:{$ref:`http://json-schema.org/draft-04/schema#/properties/default`},multipleOf:{$ref:`http://json-schema.org/draft-04/schema#/properties/multipleOf`},maximum:{$ref:`http://json-schema.org/draft-04/schema#/properties/maximum`},exclusiveMaximum:{$ref:`http://json-schema.org/draft-04/schema#/properties/exclusiveMaximum`},minimum:{$ref:`http://json-schema.org/draft-04/schema#/properties/minimum`},exclusiveMinimum:{$ref:`http://json-schema.org/draft-04/schema#/properties/exclusiveMinimum`},maxLength:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveInteger`},minLength:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveIntegerDefault0`},pattern:{$ref:`http://json-schema.org/draft-04/schema#/properties/pattern`},maxItems:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveInteger`},minItems:{$ref:`http://json-schema.org/draft-04/schema#/definitions/positiveIntegerDefault0`},uniqueItems:{$ref:`http://json-schema.org/draft-04/schema#/properties/uniqueItems`},enum:{$ref:`http://json-schema.org/draft-04/schema#/properties/enum`},jsonReference:{type:`object`,required:[`$ref`],additionalProperties:!1,properties:{$ref:{type:`string`}}}}},fr={id:`https://spec.openapis.org/oas/3.0/schema/2021-09-28`,$schema:`http://json-schema.org/draft-04/schema#`,description:`The description of OpenAPI v3.0.x documents, as defined by https://spec.openapis.org/oas/v3.0.3`,type:`object`,required:[`openapi`,`info`,`paths`],properties:{openapi:{type:`string`,pattern:`^3\\.0\\.\\d(-.+)?$`},info:{$ref:`#/definitions/Info`},externalDocs:{$ref:`#/definitions/ExternalDocumentation`},servers:{type:`array`,items:{$ref:`#/definitions/Server`}},security:{type:`array`,items:{$ref:`#/definitions/SecurityRequirement`}},tags:{type:`array`,items:{$ref:`#/definitions/Tag`},uniqueItems:!0},paths:{$ref:`#/definitions/Paths`},components:{$ref:`#/definitions/Components`}},patternProperties:{"^x-":{}},additionalProperties:!1,definitions:{Reference:{type:`object`,required:[`$ref`],patternProperties:{"^\\$ref$":{type:`string`,format:`uri-reference`}}},Info:{type:`object`,required:[`title`,`version`],properties:{title:{type:`string`},description:{type:`string`},termsOfService:{type:`string`,format:`uri-reference`},contact:{$ref:`#/definitions/Contact`},license:{$ref:`#/definitions/License`},version:{type:`string`}},patternProperties:{"^x-":{}},additionalProperties:!1},Contact:{type:`object`,properties:{name:{type:`string`},url:{type:`string`,format:`uri-reference`},email:{type:`string`,format:`email`}},patternProperties:{"^x-":{}},additionalProperties:!1},License:{type:`object`,required:[`name`],properties:{name:{type:`string`},url:{type:`string`,format:`uri-reference`}},patternProperties:{"^x-":{}},additionalProperties:!1},Server:{type:`object`,required:[`url`],properties:{url:{type:`string`},description:{type:`string`},variables:{type:`object`,additionalProperties:{$ref:`#/definitions/ServerVariable`}}},patternProperties:{"^x-":{}},additionalProperties:!1},ServerVariable:{type:`object`,required:[`default`],properties:{enum:{type:`array`,items:{type:`string`}},default:{type:`string`},description:{type:`string`}},patternProperties:{"^x-":{}},additionalProperties:!1},Components:{type:`object`,properties:{schemas:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]}}},responses:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/Response`}]}}},parameters:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/Parameter`}]}}},examples:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/Example`}]}}},requestBodies:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/RequestBody`}]}}},headers:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/Header`}]}}},securitySchemes:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/SecurityScheme`}]}}},links:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/Link`}]}}},callbacks:{type:`object`,patternProperties:{"^[a-zA-Z0-9\\.\\-_]+$":{oneOf:[{$ref:`#/definitions/Reference`},{$ref:`#/definitions/Callback`}]}}}},patternProperties:{"^x-":{}},additionalProperties:!1},Schema:{type:`object`,properties:{title:{type:`string`},multipleOf:{type:`number`,minimum:0,exclusiveMinimum:!0},maximum:{type:`number`},exclusiveMaximum:{type:`boolean`,default:!1},minimum:{type:`number`},exclusiveMinimum:{type:`boolean`,default:!1},maxLength:{type:`integer`,minimum:0},minLength:{type:`integer`,minimum:0,default:0},pattern:{type:`string`,format:`regex`},maxItems:{type:`integer`,minimum:0},minItems:{type:`integer`,minimum:0,default:0},uniqueItems:{type:`boolean`,default:!1},maxProperties:{type:`integer`,minimum:0},minProperties:{type:`integer`,minimum:0,default:0},required:{type:`array`,items:{type:`string`},minItems:1,uniqueItems:!0},enum:{type:`array`,items:{},minItems:1,uniqueItems:!1},type:{type:`string`,enum:[`array`,`boolean`,`integer`,`number`,`object`,`string`]},not:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]},allOf:{type:`array`,items:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]}},oneOf:{type:`array`,items:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]}},anyOf:{type:`array`,items:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]}},items:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]},properties:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]}},additionalProperties:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`},{type:`boolean`}],default:!0},description:{type:`string`},format:{type:`string`},default:{},nullable:{type:`boolean`,default:!1},discriminator:{$ref:`#/definitions/Discriminator`},readOnly:{type:`boolean`,default:!1},writeOnly:{type:`boolean`,default:!1},example:{},externalDocs:{$ref:`#/definitions/ExternalDocumentation`},deprecated:{type:`boolean`,default:!1},xml:{$ref:`#/definitions/XML`}},patternProperties:{"^x-":{}},additionalProperties:!1},Discriminator:{type:`object`,required:[`propertyName`],properties:{propertyName:{type:`string`},mapping:{type:`object`,additionalProperties:{type:`string`}}}},XML:{type:`object`,properties:{name:{type:`string`},namespace:{type:`string`,format:`uri`},prefix:{type:`string`},attribute:{type:`boolean`,default:!1},wrapped:{type:`boolean`,default:!1}},patternProperties:{"^x-":{}},additionalProperties:!1},Response:{type:`object`,required:[`description`],properties:{description:{type:`string`},headers:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Header`},{$ref:`#/definitions/Reference`}]}},content:{type:`object`,additionalProperties:{$ref:`#/definitions/MediaType`}},links:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Link`},{$ref:`#/definitions/Reference`}]}}},patternProperties:{"^x-":{}},additionalProperties:!1},MediaType:{type:`object`,properties:{schema:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]},example:{},examples:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Example`},{$ref:`#/definitions/Reference`}]}},encoding:{type:`object`,additionalProperties:{$ref:`#/definitions/Encoding`}}},patternProperties:{"^x-":{}},additionalProperties:!1,allOf:[{$ref:`#/definitions/ExampleXORExamples`}]},Example:{type:`object`,properties:{summary:{type:`string`},description:{type:`string`},value:{},externalValue:{type:`string`,format:`uri-reference`}},patternProperties:{"^x-":{}},additionalProperties:!1},Header:{type:`object`,properties:{description:{type:`string`},required:{type:`boolean`,default:!1},deprecated:{type:`boolean`,default:!1},allowEmptyValue:{type:`boolean`,default:!1},style:{type:`string`,enum:[`simple`],default:`simple`},explode:{type:`boolean`},allowReserved:{type:`boolean`,default:!1},schema:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]},content:{type:`object`,additionalProperties:{$ref:`#/definitions/MediaType`},minProperties:1,maxProperties:1},example:{},examples:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Example`},{$ref:`#/definitions/Reference`}]}}},patternProperties:{"^x-":{}},additionalProperties:!1,allOf:[{$ref:`#/definitions/ExampleXORExamples`},{$ref:`#/definitions/SchemaXORContent`}]},Paths:{type:`object`,patternProperties:{"^\\/":{$ref:`#/definitions/PathItemOrReference`},"^x-":{}},additionalProperties:!1},PathItem:{type:`object`,properties:{summary:{type:`string`},description:{type:`string`},servers:{type:`array`,items:{$ref:`#/definitions/Server`}},parameters:{type:`array`,items:{oneOf:[{$ref:`#/definitions/Parameter`},{$ref:`#/definitions/Reference`}]},uniqueItems:!0}},patternProperties:{"^(get|put|post|delete|options|head|patch|trace)$":{$ref:`#/definitions/Operation`},"^x-":{}},additionalProperties:!1},PathItemOrReference:{oneOf:[{$ref:`#/definitions/PathItem`},{$ref:`#/definitions/Reference`}]},Operation:{type:`object`,required:[`responses`],properties:{tags:{type:`array`,items:{type:`string`}},summary:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/definitions/ExternalDocumentation`},operationId:{type:`string`},parameters:{type:`array`,items:{oneOf:[{$ref:`#/definitions/Parameter`},{$ref:`#/definitions/Reference`}]},uniqueItems:!0},requestBody:{oneOf:[{$ref:`#/definitions/RequestBody`},{$ref:`#/definitions/Reference`}]},responses:{$ref:`#/definitions/Responses`},callbacks:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Callback`},{$ref:`#/definitions/Reference`}]}},deprecated:{type:`boolean`,default:!1},security:{type:`array`,items:{$ref:`#/definitions/SecurityRequirement`}},servers:{type:`array`,items:{$ref:`#/definitions/Server`}}},patternProperties:{"^x-":{}},additionalProperties:!1},Responses:{type:`object`,properties:{default:{oneOf:[{$ref:`#/definitions/Response`},{$ref:`#/definitions/Reference`}]}},patternProperties:{"^[1-5](?:\\d{2}|XX)$":{oneOf:[{$ref:`#/definitions/Response`},{$ref:`#/definitions/Reference`}]},"^x-":{}},minProperties:1,additionalProperties:!1},SecurityRequirement:{type:`object`,additionalProperties:{type:`array`,items:{type:`string`}}},Tag:{type:`object`,required:[`name`],properties:{name:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/definitions/ExternalDocumentation`}},patternProperties:{"^x-":{}},additionalProperties:!1},ExternalDocumentation:{type:`object`,required:[`url`],properties:{description:{type:`string`},url:{type:`string`,format:`uri-reference`}},patternProperties:{"^x-":{}},additionalProperties:!1},ExampleXORExamples:{description:`Example and examples are mutually exclusive`,not:{required:[`example`,`examples`]}},SchemaXORContent:{description:`Schema and content are mutually exclusive, at least one is required`,not:{required:[`schema`,`content`]},oneOf:[{required:[`schema`]},{required:[`content`],description:`Some properties are not allowed if content is present`,allOf:[{not:{required:[`style`]}},{not:{required:[`explode`]}},{not:{required:[`allowReserved`]}},{not:{required:[`example`]}},{not:{required:[`examples`]}}]}]},Parameter:{type:`object`,properties:{name:{type:`string`},in:{type:`string`},description:{type:`string`},required:{type:`boolean`,default:!1},deprecated:{type:`boolean`,default:!1},allowEmptyValue:{type:`boolean`,default:!1},style:{type:`string`},explode:{type:`boolean`},allowReserved:{type:`boolean`,default:!1},schema:{oneOf:[{$ref:`#/definitions/Schema`},{$ref:`#/definitions/Reference`}]},content:{type:`object`,additionalProperties:{$ref:`#/definitions/MediaType`},minProperties:1,maxProperties:1},example:{},examples:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Example`},{$ref:`#/definitions/Reference`}]}}},patternProperties:{"^x-":{}},additionalProperties:!1,required:[`name`,`in`],allOf:[{$ref:`#/definitions/ExampleXORExamples`},{$ref:`#/definitions/SchemaXORContent`},{$ref:`#/definitions/ParameterLocation`}]},ParameterLocation:{description:`Parameter location`,oneOf:[{description:`Parameter in path`,required:[`required`],properties:{in:{enum:[`path`]},style:{enum:[`matrix`,`label`,`simple`],default:`simple`},required:{enum:[!0]}}},{description:`Parameter in query`,properties:{in:{enum:[`query`]},style:{enum:[`form`,`spaceDelimited`,`pipeDelimited`,`deepObject`],default:`form`}}},{description:`Parameter in header`,properties:{in:{enum:[`header`]},style:{enum:[`simple`],default:`simple`}}},{description:`Parameter in cookie`,properties:{in:{enum:[`cookie`]},style:{enum:[`form`],default:`form`}}}]},RequestBody:{type:`object`,required:[`content`],properties:{description:{type:`string`},content:{type:`object`,additionalProperties:{$ref:`#/definitions/MediaType`}},required:{type:`boolean`,default:!1}},patternProperties:{"^x-":{}},additionalProperties:!1},SecurityScheme:{oneOf:[{$ref:`#/definitions/APIKeySecurityScheme`},{$ref:`#/definitions/HTTPSecurityScheme`},{$ref:`#/definitions/OAuth2SecurityScheme`},{$ref:`#/definitions/OpenIdConnectSecurityScheme`}],discriminator:{propertyName:`type`}},APIKeySecurityScheme:{type:`object`,required:[`type`,`name`,`in`],properties:{type:{type:`string`,enum:[`apiKey`]},name:{type:`string`},in:{type:`string`,enum:[`header`,`query`,`cookie`]},description:{type:`string`}},patternProperties:{"^x-":{}},additionalProperties:!1},HTTPSecurityScheme:{type:`object`,required:[`scheme`,`type`],properties:{scheme:{type:`string`},bearerFormat:{type:`string`},description:{type:`string`},type:{type:`string`,enum:[`http`]}},patternProperties:{"^x-":{}},additionalProperties:!1,oneOf:[{description:`Bearer`,properties:{scheme:{type:`string`,pattern:`^[Bb][Ee][Aa][Rr][Ee][Rr]$`}}},{description:`Non Bearer`,not:{required:[`bearerFormat`]},properties:{scheme:{not:{type:`string`,pattern:`^[Bb][Ee][Aa][Rr][Ee][Rr]$`}}}}]},OAuth2SecurityScheme:{type:`object`,required:[`type`,`flows`],properties:{type:{type:`string`,enum:[`oauth2`]},flows:{$ref:`#/definitions/OAuthFlows`},description:{type:`string`}},patternProperties:{"^x-":{}},additionalProperties:!1},OpenIdConnectSecurityScheme:{type:`object`,required:[`type`,`openIdConnectUrl`],properties:{type:{type:`string`,enum:[`openIdConnect`]},openIdConnectUrl:{type:`string`,format:`uri-reference`},description:{type:`string`}},patternProperties:{"^x-":{}},additionalProperties:!1},OAuthFlows:{type:`object`,properties:{implicit:{$ref:`#/definitions/ImplicitOAuthFlow`},password:{$ref:`#/definitions/PasswordOAuthFlow`},clientCredentials:{$ref:`#/definitions/ClientCredentialsFlow`},authorizationCode:{$ref:`#/definitions/AuthorizationCodeOAuthFlow`}},patternProperties:{"^x-":{}},additionalProperties:!1},ImplicitOAuthFlow:{type:`object`,required:[`authorizationUrl`,`scopes`],properties:{authorizationUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{type:`object`,additionalProperties:{type:`string`}}},patternProperties:{"^x-":{}},additionalProperties:!1},PasswordOAuthFlow:{type:`object`,required:[`tokenUrl`,`scopes`],properties:{tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{type:`object`,additionalProperties:{type:`string`}}},patternProperties:{"^x-":{}},additionalProperties:!1},ClientCredentialsFlow:{type:`object`,required:[`tokenUrl`,`scopes`],properties:{tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{type:`object`,additionalProperties:{type:`string`}}},patternProperties:{"^x-":{}},additionalProperties:!1},AuthorizationCodeOAuthFlow:{type:`object`,required:[`authorizationUrl`,`tokenUrl`,`scopes`],properties:{authorizationUrl:{type:`string`,format:`uri-reference`},tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{type:`object`,additionalProperties:{type:`string`}}},patternProperties:{"^x-":{}},additionalProperties:!1},Link:{type:`object`,properties:{operationId:{type:`string`},operationRef:{type:`string`,format:`uri-reference`},parameters:{type:`object`,additionalProperties:{}},requestBody:{},description:{type:`string`},server:{$ref:`#/definitions/Server`}},patternProperties:{"^x-":{}},additionalProperties:!1,not:{description:`Operation Id and Operation Ref are mutually exclusive`,required:[`operationId`,`operationRef`]}},Callback:{type:`object`,additionalProperties:{$ref:`#/definitions/PathItem`},patternProperties:{"^x-":{}}},Encoding:{type:`object`,properties:{contentType:{type:`string`},headers:{type:`object`,additionalProperties:{oneOf:[{$ref:`#/definitions/Header`},{$ref:`#/definitions/Reference`}]}},style:{type:`string`,enum:[`form`,`spaceDelimited`,`pipeDelimited`,`deepObject`]},explode:{type:`boolean`},allowReserved:{type:`boolean`,default:!1}},additionalProperties:!1}}},pr={$id:`https://spec.openapis.org/oas/3.1/schema/2022-10-07`,$schema:`https://json-schema.org/draft/2020-12/schema`,description:`The description of OpenAPI v3.1.x documents without schema validation, as defined by https://spec.openapis.org/oas/v3.1.0`,type:`object`,properties:{openapi:{type:`string`,pattern:`^3\\.1\\.\\d+(-.+)?$`},info:{$ref:`#/$defs/info`},jsonSchemaDialect:{type:`string`,format:`uri-reference`,default:`https://spec.openapis.org/oas/3.1/dialect/base`},servers:{type:`array`,items:{$ref:`#/$defs/server`},default:[{url:`/`}]},paths:{$ref:`#/$defs/paths`},webhooks:{type:`object`,additionalProperties:{$ref:`#/$defs/path-item-or-reference`}},components:{$ref:`#/$defs/components`},security:{type:`array`,items:{$ref:`#/$defs/security-requirement`}},tags:{type:`array`,items:{$ref:`#/$defs/tag`}},externalDocs:{$ref:`#/$defs/external-documentation`}},required:[`openapi`,`info`],anyOf:[{required:[`paths`]},{required:[`components`]},{required:[`webhooks`]}],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1,$defs:{info:{$comment:`https://spec.openapis.org/oas/v3.1.0#info-object`,type:`object`,properties:{title:{type:`string`},summary:{type:`string`},description:{type:`string`},termsOfService:{type:`string`,format:`uri-reference`},contact:{$ref:`#/$defs/contact`},license:{$ref:`#/$defs/license`},version:{type:`string`}},required:[`title`,`version`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},contact:{$comment:`https://spec.openapis.org/oas/v3.1.0#contact-object`,type:`object`,properties:{name:{type:`string`},url:{type:`string`,format:`uri-reference`},email:{type:`string`,format:`email`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},license:{$comment:`https://spec.openapis.org/oas/v3.1.0#license-object`,type:`object`,properties:{name:{type:`string`},identifier:{type:`string`},url:{type:`string`,format:`uri-reference`}},required:[`name`],dependentSchemas:{identifier:{not:{required:[`url`]}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},server:{$comment:`https://spec.openapis.org/oas/v3.1.0#server-object`,type:`object`,properties:{url:{type:`string`},description:{type:`string`},variables:{type:`object`,additionalProperties:{$ref:`#/$defs/server-variable`}}},required:[`url`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"server-variable":{$comment:`https://spec.openapis.org/oas/v3.1.0#server-variable-object`,type:`object`,properties:{enum:{type:`array`,items:{type:`string`},minItems:1},default:{type:`string`},description:{type:`string`}},required:[`default`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},components:{$comment:`https://spec.openapis.org/oas/v3.1.0#components-object`,type:`object`,properties:{schemas:{type:`object`,additionalProperties:{$ref:`#/$defs/schema`}},responses:{type:`object`,additionalProperties:{$ref:`#/$defs/response-or-reference`}},parameters:{type:`object`,additionalProperties:{$ref:`#/$defs/parameter-or-reference`}},examples:{type:`object`,additionalProperties:{$ref:`#/$defs/example-or-reference`}},requestBodies:{type:`object`,additionalProperties:{$ref:`#/$defs/request-body-or-reference`}},headers:{type:`object`,additionalProperties:{$ref:`#/$defs/header-or-reference`}},securitySchemes:{type:`object`,additionalProperties:{$ref:`#/$defs/security-scheme-or-reference`}},links:{type:`object`,additionalProperties:{$ref:`#/$defs/link-or-reference`}},callbacks:{type:`object`,additionalProperties:{$ref:`#/$defs/callbacks-or-reference`}},pathItems:{type:`object`,additionalProperties:{$ref:`#/$defs/path-item-or-reference`}}},patternProperties:{"^(schemas|responses|parameters|examples|requestBodies|headers|securitySchemes|links|callbacks|pathItems)$":{$comment:`Enumerating all of the property names in the regex above is necessary for unevaluatedProperties to work as expected`,propertyNames:{pattern:`^[a-zA-Z0-9._-]+$`}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},paths:{$comment:`https://spec.openapis.org/oas/v3.1.0#paths-object`,type:`object`,patternProperties:{"^/":{$ref:`#/$defs/path-item-or-reference`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"path-item":{$comment:`https://spec.openapis.org/oas/v3.1.0#path-item-object`,type:`object`,properties:{summary:{type:`string`},description:{type:`string`},servers:{type:`array`,items:{$ref:`#/$defs/server`}},parameters:{type:`array`,items:{$ref:`#/$defs/parameter-or-reference`}},get:{$ref:`#/$defs/operation`},put:{$ref:`#/$defs/operation`},post:{$ref:`#/$defs/operation`},delete:{$ref:`#/$defs/operation`},options:{$ref:`#/$defs/operation`},head:{$ref:`#/$defs/operation`},patch:{$ref:`#/$defs/operation`},trace:{$ref:`#/$defs/operation`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"path-item-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/path-item`}},operation:{$comment:`https://spec.openapis.org/oas/v3.1.0#operation-object`,type:`object`,properties:{tags:{type:`array`,items:{type:`string`}},summary:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/$defs/external-documentation`},operationId:{type:`string`},parameters:{type:`array`,items:{$ref:`#/$defs/parameter-or-reference`}},requestBody:{$ref:`#/$defs/request-body-or-reference`},responses:{$ref:`#/$defs/responses`},callbacks:{type:`object`,additionalProperties:{$ref:`#/$defs/callbacks-or-reference`}},deprecated:{default:!1,type:`boolean`},security:{type:`array`,items:{$ref:`#/$defs/security-requirement`}},servers:{type:`array`,items:{$ref:`#/$defs/server`}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"external-documentation":{$comment:`https://spec.openapis.org/oas/v3.1.0#external-documentation-object`,type:`object`,properties:{description:{type:`string`},url:{type:`string`,format:`uri-reference`}},required:[`url`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},parameter:{$comment:`https://spec.openapis.org/oas/v3.1.0#parameter-object`,type:`object`,properties:{name:{type:`string`},in:{enum:[`query`,`header`,`path`,`cookie`]},description:{type:`string`},required:{default:!1,type:`boolean`},deprecated:{default:!1,type:`boolean`},schema:{$ref:`#/$defs/schema`},content:{$ref:`#/$defs/content`,minProperties:1,maxProperties:1}},required:[`name`,`in`],oneOf:[{required:[`schema`]},{required:[`content`]}],if:{properties:{in:{const:`query`}},required:[`in`]},then:{properties:{allowEmptyValue:{default:!1,type:`boolean`}}},dependentSchemas:{schema:{properties:{style:{type:`string`},explode:{type:`boolean`}},allOf:[{$ref:`#/$defs/examples`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-path`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-header`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-query`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-cookie`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-form`}],$defs:{"styles-for-path":{if:{properties:{in:{const:`path`}},required:[`in`]},then:{properties:{name:{pattern:`[^/#?]+$`},style:{default:`simple`,enum:[`matrix`,`label`,`simple`]},required:{const:!0}},required:[`required`]}},"styles-for-header":{if:{properties:{in:{const:`header`}},required:[`in`]},then:{properties:{style:{default:`simple`,const:`simple`}}}},"styles-for-query":{if:{properties:{in:{const:`query`}},required:[`in`]},then:{properties:{style:{default:`form`,enum:[`form`,`spaceDelimited`,`pipeDelimited`,`deepObject`]},allowReserved:{default:!1,type:`boolean`}}}},"styles-for-cookie":{if:{properties:{in:{const:`cookie`}},required:[`in`]},then:{properties:{style:{default:`form`,const:`form`}}}},"styles-for-form":{if:{properties:{style:{const:`form`}},required:[`style`]},then:{properties:{explode:{default:!0}}},else:{properties:{explode:{default:!1}}}}}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"parameter-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/parameter`}},"request-body":{$comment:`https://spec.openapis.org/oas/v3.1.0#request-body-object`,type:`object`,properties:{description:{type:`string`},content:{$ref:`#/$defs/content`},required:{default:!1,type:`boolean`}},required:[`content`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"request-body-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/request-body`}},content:{$comment:`https://spec.openapis.org/oas/v3.1.0#fixed-fields-10`,type:`object`,additionalProperties:{$ref:`#/$defs/media-type`},propertyNames:{format:`media-range`}},"media-type":{$comment:`https://spec.openapis.org/oas/v3.1.0#media-type-object`,type:`object`,properties:{schema:{$ref:`#/$defs/schema`},encoding:{type:`object`,additionalProperties:{$ref:`#/$defs/encoding`}}},allOf:[{$ref:`#/$defs/specification-extensions`},{$ref:`#/$defs/examples`}],unevaluatedProperties:!1},encoding:{$comment:`https://spec.openapis.org/oas/v3.1.0#encoding-object`,type:`object`,properties:{contentType:{type:`string`,format:`media-range`},headers:{type:`object`,additionalProperties:{$ref:`#/$defs/header-or-reference`}},style:{default:`form`,enum:[`form`,`spaceDelimited`,`pipeDelimited`,`deepObject`]},explode:{type:`boolean`},allowReserved:{default:!1,type:`boolean`}},allOf:[{$ref:`#/$defs/specification-extensions`},{$ref:`#/$defs/encoding/$defs/explode-default`}],unevaluatedProperties:!1,$defs:{"explode-default":{if:{properties:{style:{const:`form`}},required:[`style`]},then:{properties:{explode:{default:!0}}},else:{properties:{explode:{default:!1}}}}}},responses:{$comment:`https://spec.openapis.org/oas/v3.1.0#responses-object`,type:`object`,properties:{default:{$ref:`#/$defs/response-or-reference`}},patternProperties:{"^[1-5](?:[0-9]{2}|XX)$":{$ref:`#/$defs/response-or-reference`}},minProperties:1,$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},response:{$comment:`https://spec.openapis.org/oas/v3.1.0#response-object`,type:`object`,properties:{description:{type:`string`},headers:{type:`object`,additionalProperties:{$ref:`#/$defs/header-or-reference`}},content:{$ref:`#/$defs/content`},links:{type:`object`,additionalProperties:{$ref:`#/$defs/link-or-reference`}}},required:[`description`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"response-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/response`}},callbacks:{$comment:`https://spec.openapis.org/oas/v3.1.0#callback-object`,type:`object`,$ref:`#/$defs/specification-extensions`,additionalProperties:{$ref:`#/$defs/path-item-or-reference`}},"callbacks-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/callbacks`}},example:{$comment:`https://spec.openapis.org/oas/v3.1.0#example-object`,type:`object`,properties:{summary:{type:`string`},description:{type:`string`},value:!0,externalValue:{type:`string`,format:`uri-reference`}},not:{required:[`value`,`externalValue`]},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"example-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/example`}},link:{$comment:`https://spec.openapis.org/oas/v3.1.0#link-object`,type:`object`,properties:{operationRef:{type:`string`,format:`uri-reference`},operationId:{type:`string`},parameters:{$ref:`#/$defs/map-of-strings`},requestBody:!0,description:{type:`string`},body:{$ref:`#/$defs/server`}},oneOf:[{required:[`operationRef`]},{required:[`operationId`]}],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"link-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/link`}},header:{$comment:`https://spec.openapis.org/oas/v3.1.0#header-object`,type:`object`,properties:{description:{type:`string`},required:{default:!1,type:`boolean`},deprecated:{default:!1,type:`boolean`},schema:{$ref:`#/$defs/schema`},content:{$ref:`#/$defs/content`,minProperties:1,maxProperties:1}},oneOf:[{required:[`schema`]},{required:[`content`]}],dependentSchemas:{schema:{properties:{style:{default:`simple`,const:`simple`},explode:{default:!1,type:`boolean`}},$ref:`#/$defs/examples`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"header-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/header`}},tag:{$comment:`https://spec.openapis.org/oas/v3.1.0#tag-object`,type:`object`,properties:{name:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/$defs/external-documentation`}},required:[`name`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},reference:{$comment:`https://spec.openapis.org/oas/v3.1.0#reference-object`,type:`object`,properties:{$ref:{type:`string`,format:`uri-reference`},summary:{type:`string`},description:{type:`string`}},unevaluatedProperties:!1},schema:{$comment:`https://spec.openapis.org/oas/v3.1.0#schema-object`,$dynamicAnchor:`meta`,type:[`object`,`boolean`]},"security-scheme":{$comment:`https://spec.openapis.org/oas/v3.1.0#security-scheme-object`,type:`object`,properties:{type:{enum:[`apiKey`,`http`,`mutualTLS`,`oauth2`,`openIdConnect`]},description:{type:`string`}},required:[`type`],allOf:[{$ref:`#/$defs/specification-extensions`},{$ref:`#/$defs/security-scheme/$defs/type-apikey`},{$ref:`#/$defs/security-scheme/$defs/type-http`},{$ref:`#/$defs/security-scheme/$defs/type-http-bearer`},{$ref:`#/$defs/security-scheme/$defs/type-oauth2`},{$ref:`#/$defs/security-scheme/$defs/type-oidc`}],unevaluatedProperties:!1,$defs:{"type-apikey":{if:{properties:{type:{const:`apiKey`}},required:[`type`]},then:{properties:{name:{type:`string`},in:{enum:[`query`,`header`,`cookie`]}},required:[`name`,`in`]}},"type-http":{if:{properties:{type:{const:`http`}},required:[`type`]},then:{properties:{scheme:{type:`string`}},required:[`scheme`]}},"type-http-bearer":{if:{properties:{type:{const:`http`},scheme:{type:`string`,pattern:`^[Bb][Ee][Aa][Rr][Ee][Rr]$`}},required:[`type`,`scheme`]},then:{properties:{bearerFormat:{type:`string`}}}},"type-oauth2":{if:{properties:{type:{const:`oauth2`}},required:[`type`]},then:{properties:{flows:{$ref:`#/$defs/oauth-flows`}},required:[`flows`]}},"type-oidc":{if:{properties:{type:{const:`openIdConnect`}},required:[`type`]},then:{properties:{openIdConnectUrl:{type:`string`,format:`uri-reference`}},required:[`openIdConnectUrl`]}}}},"security-scheme-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/security-scheme`}},"oauth-flows":{type:`object`,properties:{implicit:{$ref:`#/$defs/oauth-flows/$defs/implicit`},password:{$ref:`#/$defs/oauth-flows/$defs/password`},clientCredentials:{$ref:`#/$defs/oauth-flows/$defs/client-credentials`},authorizationCode:{$ref:`#/$defs/oauth-flows/$defs/authorization-code`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1,$defs:{implicit:{type:`object`,properties:{authorizationUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`authorizationUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},password:{type:`object`,properties:{tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"client-credentials":{type:`object`,properties:{tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"authorization-code":{type:`object`,properties:{authorizationUrl:{type:`string`,format:`uri-reference`},tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`authorizationUrl`,`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1}}},"security-requirement":{$comment:`https://spec.openapis.org/oas/v3.1.0#security-requirement-object`,type:`object`,additionalProperties:{type:`array`,items:{type:`string`}}},"specification-extensions":{$comment:`https://spec.openapis.org/oas/v3.1.0#specification-extensions`,patternProperties:{"^x-":!0}},examples:{properties:{example:!0,examples:{type:`object`,additionalProperties:{$ref:`#/$defs/example-or-reference`}}}},"map-of-strings":{type:`object`,additionalProperties:{type:`string`}}}},mr={$id:`https://spec.openapis.org/oas/3.2/schema/2025-09-17`,$schema:`https://json-schema.org/draft/2020-12/schema`,description:`The description of OpenAPI v3.2.x Documents without Schema Object validation`,type:`object`,properties:{openapi:{type:`string`,pattern:`^3\\.2\\.\\d+(-.+)?$`},$self:{type:`string`,format:`uri-reference`,$comment:`MUST NOT contain a fragment`,pattern:`^[^#]*$`},info:{$ref:`#/$defs/info`},jsonSchemaDialect:{type:`string`,format:`uri-reference`,default:`https://spec.openapis.org/oas/3.2/dialect/2025-09-17`},servers:{type:`array`,items:{$ref:`#/$defs/server`},default:[{url:`/`}]},paths:{$ref:`#/$defs/paths`},webhooks:{type:`object`,additionalProperties:{$ref:`#/$defs/path-item-or-reference`}},components:{$ref:`#/$defs/components`},security:{type:`array`,items:{$ref:`#/$defs/security-requirement`}},tags:{type:`array`,items:{$ref:`#/$defs/tag`}},externalDocs:{$ref:`#/$defs/external-documentation`}},required:[`openapi`,`info`],anyOf:[{required:[`paths`]},{required:[`components`]},{required:[`webhooks`]}],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1,$defs:{info:{$comment:`https://spec.openapis.org/oas/v3.2#info-object`,type:`object`,properties:{title:{type:`string`},summary:{type:`string`},description:{type:`string`},termsOfService:{type:`string`,format:`uri-reference`},contact:{$ref:`#/$defs/contact`},license:{$ref:`#/$defs/license`},version:{type:`string`}},required:[`title`,`version`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},contact:{$comment:`https://spec.openapis.org/oas/v3.2#contact-object`,type:`object`,properties:{name:{type:`string`},url:{type:`string`,format:`uri-reference`},email:{type:`string`,format:`email`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},license:{$comment:`https://spec.openapis.org/oas/v3.2#license-object`,type:`object`,properties:{name:{type:`string`},identifier:{type:`string`},url:{type:`string`,format:`uri-reference`}},required:[`name`],dependentSchemas:{identifier:{not:{required:[`url`]}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},server:{$comment:`https://spec.openapis.org/oas/v3.2#server-object`,type:`object`,properties:{url:{type:`string`},description:{type:`string`},name:{type:`string`},variables:{type:`object`,additionalProperties:{$ref:`#/$defs/server-variable`}}},required:[`url`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"server-variable":{$comment:`https://spec.openapis.org/oas/v3.2#server-variable-object`,type:`object`,properties:{enum:{type:`array`,items:{type:`string`},minItems:1},default:{type:`string`},description:{type:`string`}},required:[`default`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},components:{$comment:`https://spec.openapis.org/oas/v3.2#components-object`,type:`object`,properties:{schemas:{type:`object`,additionalProperties:{$ref:`#/$defs/schema`}},responses:{type:`object`,additionalProperties:{$ref:`#/$defs/response-or-reference`}},parameters:{type:`object`,additionalProperties:{$ref:`#/$defs/parameter-or-reference`}},examples:{type:`object`,additionalProperties:{$ref:`#/$defs/example-or-reference`}},requestBodies:{type:`object`,additionalProperties:{$ref:`#/$defs/request-body-or-reference`}},headers:{type:`object`,additionalProperties:{$ref:`#/$defs/header-or-reference`}},securitySchemes:{type:`object`,additionalProperties:{$ref:`#/$defs/security-scheme-or-reference`}},links:{type:`object`,additionalProperties:{$ref:`#/$defs/link-or-reference`}},callbacks:{type:`object`,additionalProperties:{$ref:`#/$defs/callbacks-or-reference`}},pathItems:{type:`object`,additionalProperties:{$ref:`#/$defs/path-item-or-reference`}},mediaTypes:{type:`object`,additionalProperties:{$ref:`#/$defs/media-type-or-reference`}}},patternProperties:{"^(?:schemas|responses|parameters|examples|requestBodies|headers|securitySchemes|links|callbacks|pathItems|mediaTypes)$":{$comment:`Enumerating all of the property names in the regex above is necessary for unevaluatedProperties to work as expected`,propertyNames:{pattern:`^[a-zA-Z0-9._-]+$`}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},paths:{$comment:`https://spec.openapis.org/oas/v3.2#paths-object`,type:`object`,patternProperties:{"^/":{$ref:`#/$defs/path-item-or-reference`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"path-item":{$comment:`https://spec.openapis.org/oas/v3.2#path-item-object`,type:`object`,properties:{summary:{type:`string`},description:{type:`string`},servers:{type:`array`,items:{$ref:`#/$defs/server`}},parameters:{$ref:`#/$defs/parameters`},additionalOperations:{type:`object`,additionalProperties:{$ref:`#/$defs/operation`},propertyNames:{$comment:`RFC9110 restricts methods to "1*tchar" in ABNF`,pattern:"^[a-zA-Z0-9!#$%&'*+.^_`|~-]+$",not:{enum:[`GET`,`PUT`,`POST`,`DELETE`,`OPTIONS`,`HEAD`,`PATCH`,`TRACE`,`QUERY`]}}},get:{$ref:`#/$defs/operation`},put:{$ref:`#/$defs/operation`},post:{$ref:`#/$defs/operation`},delete:{$ref:`#/$defs/operation`},options:{$ref:`#/$defs/operation`},head:{$ref:`#/$defs/operation`},patch:{$ref:`#/$defs/operation`},trace:{$ref:`#/$defs/operation`},query:{$ref:`#/$defs/operation`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"path-item-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/path-item`}},operation:{$comment:`https://spec.openapis.org/oas/v3.2#operation-object`,type:`object`,properties:{tags:{type:`array`,items:{type:`string`}},summary:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/$defs/external-documentation`},operationId:{type:`string`},parameters:{$ref:`#/$defs/parameters`},requestBody:{$ref:`#/$defs/request-body-or-reference`},responses:{$ref:`#/$defs/responses`},callbacks:{type:`object`,additionalProperties:{$ref:`#/$defs/callbacks-or-reference`}},deprecated:{default:!1,type:`boolean`},security:{type:`array`,items:{$ref:`#/$defs/security-requirement`}},servers:{type:`array`,items:{$ref:`#/$defs/server`}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"external-documentation":{$comment:`https://spec.openapis.org/oas/v3.2#external-documentation-object`,type:`object`,properties:{description:{type:`string`},url:{type:`string`,format:`uri-reference`}},required:[`url`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},parameters:{type:`array`,items:{$ref:`#/$defs/parameter-or-reference`},not:{allOf:[{contains:{type:`object`,properties:{in:{const:`query`}},required:[`in`]}},{contains:{type:`object`,properties:{in:{const:`querystring`}},required:[`in`]}}]},contains:{type:`object`,properties:{in:{const:`querystring`}},required:[`in`]},minContains:0,maxContains:1},parameter:{$comment:`https://spec.openapis.org/oas/v3.2#parameter-object`,type:`object`,properties:{name:{type:`string`},in:{enum:[`query`,`querystring`,`header`,`path`,`cookie`]},description:{type:`string`},required:{default:!1,type:`boolean`},deprecated:{default:!1,type:`boolean`},schema:{$ref:`#/$defs/schema`},content:{$ref:`#/$defs/content`,minProperties:1,maxProperties:1}},required:[`name`,`in`],oneOf:[{required:[`schema`]},{required:[`content`]}],allOf:[{$ref:`#/$defs/examples`},{$ref:`#/$defs/specification-extensions`},{if:{properties:{in:{const:`query`}}},then:{properties:{allowEmptyValue:{default:!1,type:`boolean`}}}},{if:{properties:{in:{const:`querystring`}}},then:{required:[`content`]}}],dependentSchemas:{schema:{properties:{style:{type:`string`},explode:{type:`boolean`},allowReserved:{default:!1,type:`boolean`}},allOf:[{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-path`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-header`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-query`},{$ref:`#/$defs/parameter/dependentSchemas/schema/$defs/styles-for-cookie`},{$ref:`#/$defs/styles-for-form`}],$defs:{"styles-for-path":{if:{properties:{in:{const:`path`}}},then:{properties:{style:{default:`simple`,enum:[`matrix`,`label`,`simple`]},required:{const:!0}},required:[`required`]}},"styles-for-header":{if:{properties:{in:{const:`header`}}},then:{properties:{style:{default:`simple`,const:`simple`}}}},"styles-for-query":{if:{properties:{in:{const:`query`}}},then:{properties:{style:{default:`form`,enum:[`form`,`spaceDelimited`,`pipeDelimited`,`deepObject`]}}}},"styles-for-cookie":{if:{properties:{in:{const:`cookie`}}},then:{properties:{style:{default:`form`,enum:[`form`,`cookie`]}}}}}}},unevaluatedProperties:!1},"parameter-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/parameter`}},"request-body":{$comment:`https://spec.openapis.org/oas/v3.2#request-body-object`,type:`object`,properties:{description:{type:`string`},content:{$ref:`#/$defs/content`},required:{default:!1,type:`boolean`}},required:[`content`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"request-body-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/request-body`}},content:{$comment:`https://spec.openapis.org/oas/v3.2#fixed-fields-10`,type:`object`,additionalProperties:{$ref:`#/$defs/media-type-or-reference`},propertyNames:{format:`media-range`}},"media-type":{$comment:`https://spec.openapis.org/oas/v3.2#media-type-object`,type:`object`,properties:{description:{type:`string`},schema:{$ref:`#/$defs/schema`},itemSchema:{$ref:`#/$defs/schema`},encoding:{type:`object`,additionalProperties:{$ref:`#/$defs/encoding`}},prefixEncoding:{type:`array`,items:{$ref:`#/$defs/encoding`}},itemEncoding:{$ref:`#/$defs/encoding`}},dependentSchemas:{encoding:{properties:{prefixEncoding:!1,itemEncoding:!1}}},allOf:[{$ref:`#/$defs/examples`},{$ref:`#/$defs/specification-extensions`}],unevaluatedProperties:!1},"media-type-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/media-type`}},encoding:{$comment:`https://spec.openapis.org/oas/v3.2#encoding-object`,type:`object`,properties:{contentType:{type:`string`,format:`media-range`},headers:{type:`object`,additionalProperties:{$ref:`#/$defs/header-or-reference`}},style:{enum:[`form`,`spaceDelimited`,`pipeDelimited`,`deepObject`]},explode:{type:`boolean`},allowReserved:{type:`boolean`},encoding:{type:`object`,additionalProperties:{$ref:`#/$defs/encoding`}},prefixEncoding:{type:`array`,items:{$ref:`#/$defs/encoding`}},itemEncoding:{$ref:`#/$defs/encoding`}},dependentSchemas:{encoding:{properties:{prefixEncoding:!1,itemEncoding:!1}},style:{properties:{allowReserved:{default:!1}}},explode:{properties:{style:{default:`form`},allowReserved:{default:!1}}},allowReserved:{properties:{style:{default:`form`}}}},allOf:[{$ref:`#/$defs/specification-extensions`},{$ref:`#/$defs/styles-for-form`}],unevaluatedProperties:!1},responses:{$comment:`https://spec.openapis.org/oas/v3.2#responses-object`,type:`object`,properties:{default:{$ref:`#/$defs/response-or-reference`}},patternProperties:{"^[1-5](?:[0-9]{2}|XX)$":{$ref:`#/$defs/response-or-reference`}},minProperties:1,$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1,if:{$comment:`either default, or at least one response code property must exist`,patternProperties:{"^[1-5](?:[0-9]{2}|XX)$":!1}},then:{required:[`default`]}},response:{$comment:`https://spec.openapis.org/oas/v3.2#response-object`,type:`object`,properties:{summary:{type:`string`},description:{type:`string`},headers:{type:`object`,additionalProperties:{$ref:`#/$defs/header-or-reference`}},content:{$ref:`#/$defs/content`},links:{type:`object`,additionalProperties:{$ref:`#/$defs/link-or-reference`}}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"response-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/response`}},callbacks:{$comment:`https://spec.openapis.org/oas/v3.2#callback-object`,type:`object`,$ref:`#/$defs/specification-extensions`,additionalProperties:{$ref:`#/$defs/path-item`}},"callbacks-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/callbacks`}},example:{$comment:`https://spec.openapis.org/oas/v3.2#example-object`,type:`object`,properties:{summary:{type:`string`},description:{type:`string`},dataValue:!0,serializedValue:{type:`string`},value:!0,externalValue:{type:`string`,format:`uri-reference`}},allOf:[{not:{required:[`value`,`externalValue`]}},{not:{required:[`value`,`dataValue`]}},{not:{required:[`value`,`serializedValue`]}},{not:{required:[`serializedValue`,`externalValue`]}}],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"example-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/example`}},link:{$comment:`https://spec.openapis.org/oas/v3.2#link-object`,type:`object`,properties:{operationRef:{type:`string`,format:`uri-reference`},operationId:{type:`string`},parameters:{$ref:`#/$defs/map-of-strings`},requestBody:!0,description:{type:`string`},server:{$ref:`#/$defs/server`}},oneOf:[{required:[`operationRef`]},{required:[`operationId`]}],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"link-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/link`}},header:{$comment:`https://spec.openapis.org/oas/v3.2#header-object`,type:`object`,properties:{description:{type:`string`},required:{default:!1,type:`boolean`},deprecated:{default:!1,type:`boolean`},schema:{$ref:`#/$defs/schema`},content:{$ref:`#/$defs/content`,minProperties:1,maxProperties:1}},oneOf:[{required:[`schema`]},{required:[`content`]}],dependentSchemas:{schema:{properties:{style:{default:`simple`,const:`simple`},explode:{default:!1,type:`boolean`},allowReserved:{default:!1,type:`boolean`}}}},allOf:[{$ref:`#/$defs/examples`},{$ref:`#/$defs/specification-extensions`}],unevaluatedProperties:!1},"header-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/header`}},tag:{$comment:`https://spec.openapis.org/oas/v3.2#tag-object`,type:`object`,properties:{name:{type:`string`},summary:{type:`string`},description:{type:`string`},externalDocs:{$ref:`#/$defs/external-documentation`},parent:{type:`string`},kind:{type:`string`}},required:[`name`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},reference:{$comment:`https://spec.openapis.org/oas/v3.2#reference-object`,type:`object`,properties:{$ref:{type:`string`,format:`uri-reference`},summary:{type:`string`},description:{type:`string`}}},schema:{$comment:`https://spec.openapis.org/oas/v3.2#schema-object`,$dynamicAnchor:`meta`,type:[`object`,`boolean`]},"security-scheme":{$comment:`https://spec.openapis.org/oas/v3.2#security-scheme-object`,type:`object`,properties:{type:{enum:[`apiKey`,`http`,`mutualTLS`,`oauth2`,`openIdConnect`]},description:{type:`string`},deprecated:{default:!1,type:`boolean`}},required:[`type`],allOf:[{$ref:`#/$defs/specification-extensions`},{$ref:`#/$defs/security-scheme/$defs/type-apikey`},{$ref:`#/$defs/security-scheme/$defs/type-http`},{$ref:`#/$defs/security-scheme/$defs/type-http-bearer`},{$ref:`#/$defs/security-scheme/$defs/type-oauth2`},{$ref:`#/$defs/security-scheme/$defs/type-oidc`}],unevaluatedProperties:!1,$defs:{"type-apikey":{if:{properties:{type:{const:`apiKey`}}},then:{properties:{name:{type:`string`},in:{enum:[`query`,`header`,`cookie`]}},required:[`name`,`in`]}},"type-http":{if:{properties:{type:{const:`http`}}},then:{properties:{scheme:{type:`string`}},required:[`scheme`]}},"type-http-bearer":{if:{properties:{type:{const:`http`},scheme:{type:`string`,pattern:`^[Bb][Ee][Aa][Rr][Ee][Rr]$`}},required:[`type`,`scheme`]},then:{properties:{bearerFormat:{type:`string`}}}},"type-oauth2":{if:{properties:{type:{const:`oauth2`}}},then:{properties:{flows:{$ref:`#/$defs/oauth-flows`},oauth2MetadataUrl:{type:`string`,format:`uri-reference`}},required:[`flows`]}},"type-oidc":{if:{properties:{type:{const:`openIdConnect`}}},then:{properties:{openIdConnectUrl:{type:`string`,format:`uri-reference`}},required:[`openIdConnectUrl`]}}}},"security-scheme-or-reference":{if:{type:`object`,required:[`$ref`]},then:{$ref:`#/$defs/reference`},else:{$ref:`#/$defs/security-scheme`}},"oauth-flows":{type:`object`,properties:{implicit:{$ref:`#/$defs/oauth-flows/$defs/implicit`},password:{$ref:`#/$defs/oauth-flows/$defs/password`},clientCredentials:{$ref:`#/$defs/oauth-flows/$defs/client-credentials`},authorizationCode:{$ref:`#/$defs/oauth-flows/$defs/authorization-code`},deviceAuthorization:{$ref:`#/$defs/oauth-flows/$defs/device-authorization`}},$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1,$defs:{implicit:{type:`object`,properties:{authorizationUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`authorizationUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},password:{type:`object`,properties:{tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"client-credentials":{type:`object`,properties:{tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"authorization-code":{type:`object`,properties:{authorizationUrl:{type:`string`,format:`uri-reference`},tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`authorizationUrl`,`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1},"device-authorization":{type:`object`,properties:{deviceAuthorizationUrl:{type:`string`,format:`uri-reference`},tokenUrl:{type:`string`,format:`uri-reference`},refreshUrl:{type:`string`,format:`uri-reference`},scopes:{$ref:`#/$defs/map-of-strings`}},required:[`deviceAuthorizationUrl`,`tokenUrl`,`scopes`],$ref:`#/$defs/specification-extensions`,unevaluatedProperties:!1}}},"security-requirement":{$comment:`https://spec.openapis.org/oas/v3.2#security-requirement-object`,type:`object`,additionalProperties:{type:`array`,items:{type:`string`}}},"specification-extensions":{$comment:`https://spec.openapis.org/oas/v3.2#specification-extensions`,patternProperties:{"^x-":!0}},examples:{properties:{example:!0,examples:{type:`object`,additionalProperties:{$ref:`#/$defs/example-or-reference`}}},not:{required:[`example`,`examples`]}},"map-of-strings":{type:`object`,additionalProperties:{type:`string`}},"styles-for-form":{if:{properties:{style:{const:`form`}},required:[`style`]},then:{properties:{explode:{default:!0}}},else:{properties:{explode:{default:!1}}}}}},hr=Object.keys({"2.0":dr,"3.0":fr,"3.1":pr,"3.2":mr}),Y={EMPTY_OR_INVALID:`Can't find JSON, YAML or filename in data.`,OPENAPI_VERSION_NOT_SUPPORTED:`Can't find supported Swagger/OpenAPI version in the provided document, version must be a string.`,INVALID_REFERENCE:`Can't resolve reference: %s`,EXTERNAL_REFERENCE_NOT_FOUND:`Can't resolve external reference: %s`,SELF_REFERENCE:`Can't resolve reference to itself: %s`,FILE_DOES_NOT_EXIST:`File does not exist: %s`,NO_CONTENT:`No content found`};function gr(e){if(e===null)return{version:void 0,specificationType:void 0,specificationVersion:void 0};if(N(e))for(let t of new Set(hr)){let n=t===`2.0`?`swagger`:`openapi`,r=e[n];if(typeof r==`string`&&r.startsWith(t))return{version:t,specificationType:n,specificationVersion:r}}return{version:void 0,specificationType:void 0,specificationVersion:void 0}}function X(e){return e?.find(e=>e.isEntrypoint)}function _r(e,t,n=[]){let r={};for(let[i,a]of Object.entries(e)){let e=[...n,i];if(Array.isArray(a)){r[i]=a.map((n,r)=>typeof n==`object`&&!Array.isArray(n)&&n!==null?_r(n,t,[...e,r.toString()]):n);continue}if(typeof a==`object`&&a){r[i]=_r(a,t,e);continue}r[i]=a}return t(r,n)}function vr(e){let t=[];return!e||typeof e!=`object`?t:(_r(e,e=>(e.$ref&&typeof e.$ref==`string`&&!e.$ref.startsWith(`#`)&&t.push(e.$ref.split(`#`)[0]),e)),[...new Set(t)])}function Z(e){return e!==void 0&&Array.isArray(e)&&e.length>0&&e.some(e=>e.isEntrypoint===!0)}function Q(e){if(e!==null){if(typeof e==`string`){if(e.trim()===``)return;try{return JSON.parse(e)}catch{let t=/^[^:]+:/.test(e),n=e.slice(0,50).trimStart().startsWith(`{`);return!t||n?void 0:te(e,{maxAliasCount:1e4,merge:!0})}}return Z(e),e}}function yr(e,t={}){if(Z(e))return e;let n=Q(e);return[{isEntrypoint:!0,specification:n,filename:null,dir:`./`,references:vr(n),...t}]}function br(e){return decodeURI(e.replace(/~1/g,`/`).replace(/~0/g,`~`))}function xr(e){return e.split(`/`).slice(1).map(br)}function Sr(e,t,n,r=[]){let i=yr(structuredClone(e)),a=X(i),o=n?.specification??a.specification;if(!N(o)){if(t?.throwOnError)throw Error(Y.NO_CONTENT);return{valid:!1,errors:r,schema:o}}return Cr(o,i,n??a,new WeakSet,r,t),r=r.filter((e,t,n)=>t===n.findIndex(t=>t.message===e.message&&t.code===e.code)),{valid:r.length===0,errors:r,schema:o}}function Cr(e,t,n,r,i,a){if(e===null||r.has(e))return;r.add(e);function o(e){return Cr(e.specification,t,e,r,i,a),e}let s=new Set;for(;e.$ref!==void 0;){if(s.has(e.$ref)){i.push({code:`SELF_REFERENCE`,message:Y.SELF_REFERENCE.replace(`%s`,e.$ref)}),delete e.$ref;break}s.add(e.$ref);let r=wr(e.$ref,a,n,t,o,i);if(typeof r!=`object`||!r)break;let c=e.$ref;delete e.$ref;for(let t of Object.keys(r))e[t]===void 0&&(e[t]=r[t]);c&&a?.onDereference?.({schema:e,ref:c})}for(let o of Object.values(e))typeof o==`object`&&o&&Cr(o,t,n,r,i,a)}function wr(e,t,n,r,i,a){if(typeof e!=`string`){if(t?.throwOnError)throw Error(Y.INVALID_REFERENCE.replace(`%s`,e));a.push({code:`INVALID_REFERENCE`,message:Y.INVALID_REFERENCE.replace(`%s`,e)});return}let[o,s]=e.split(`#`,2),c=o!==n.filename;if(o&&c){let e=r.find(e=>e.filename===o);if(!e){if(t?.throwOnError)throw Error(Y.EXTERNAL_REFERENCE_NOT_FOUND.replace(`%s`,o));a.push({code:`EXTERNAL_REFERENCE_NOT_FOUND`,message:Y.EXTERNAL_REFERENCE_NOT_FOUND.replace(`%s`,o)});return}return s===void 0?e.specification:wr(`#${s}`,t,i(e),r,i,a)}let l=xr(s);try{return l.reduce((e,t)=>e[t],n.specification)}catch{if(t?.throwOnError)throw Error(Y.INVALID_REFERENCE.replace(`%s`,e));a.push({code:`INVALID_REFERENCE`,message:Y.INVALID_REFERENCE.replace(`%s`,e)})}}function Tr(e,t){let n=yr(e),r=X(n),i=Sr(n,t);return{specification:r.specification,errors:i.errors,schema:i.schema,...gr(r.specification)}}function Er(e){if(!(typeof e!=`object`||!e)){if(Array.isArray(e)){for(let t of e)Er(t);return}if(e.xml&&typeof e.xml==`object`){if(e.xml.wrapped===!0&&e.xml.attribute===!0)throw Error(`Invalid XML configuration: wrapped and attribute cannot be true at the same time.`);e.xml.wrapped===!0&&(delete e.xml.wrapped,e.xml.nodeType=`element`),e.xml.attribute===!0&&(delete e.xml.attribute,e.xml.nodeType=`attribute`)}for(let t in e)Object.hasOwn(e,t)&&Er(e[t])}}function Dr(e){if(e[`x-tagGroups`]&&Array.isArray(e[`x-tagGroups`])){let t=e[`x-tagGroups`];e.tags||=[];let n=new Map;for(let e of t)for(let t of e.tags)n.set(t,e.name);if(Array.isArray(e.tags)){for(let t of e.tags)if(typeof t==`object`&&t&&`name`in t){let e=n.get(t.name);e&&(e.toLowerCase().includes(`nav`)||e.toLowerCase().includes(`navigation`)?t.kind=`nav`:e.toLowerCase().includes(`audience`)?t.kind=`audience`:e.toLowerCase().includes(`badge`)?t.kind=`badge`:t.kind=`nav`)}}delete e[`x-tagGroups`]}}function Or(e){let t=e;if(typeof t==`object`&&t&&typeof t.openapi==`string`&&t.openapi?.startsWith(`3.1`))t.openapi=`3.2.0`;else return t;return Dr(t),Er(t),t}function kr(e,t){let n=Yn(e);if(t===`3.0`)return n;let r=lr(n);return t===`3.1`?r:Or(r)}function Ar(e){return e?{specification:kr(Z(e)?X(e).specification:Q(e),`3.1`),version:`3.1`}:{specification:null,version:`3.1`}}async function jr(e,t){let n=[];if(t?.filesystem?.find(t=>t.filename===e))return{specification:X(t.filesystem)?.specification,filesystem:t.filesystem,errors:n};let r=t?.plugins?.find(t=>t.check(e)),i;if(r)try{i=Q(await r.get(e))}catch{if(t?.throwOnError)throw Error(Y.EXTERNAL_REFERENCE_NOT_FOUND.replace(`%s`,e));return n.push({code:`EXTERNAL_REFERENCE_NOT_FOUND`,message:Y.EXTERNAL_REFERENCE_NOT_FOUND.replace(`%s`,e)}),{specification:null,filesystem:[],errors:n}}else i=Q(e);if(i===void 0){if(t?.throwOnError)throw Error(`No content to load`);return n.push({code:`NO_CONTENT`,message:Y.NO_CONTENT}),{specification:null,filesystem:[],errors:n}}let a=yr(i,{filename:t?.filename??null}),o=(t?.filename?a.find(e=>e.filename===t?.filename):X(a)).references??vr(i);if(o.length===0)return{specification:X(a)?.specification,filesystem:a,errors:n};for(let r of o){let i=t?.plugins?.find(e=>e.check(r));if(!i)continue;let o=i.check(r)&&i.resolvePath?i.resolvePath(e,r):r;if(a.find(e=>e.filename===r))continue;let{filesystem:s,errors:c}=await jr(o,{...t,filename:r});n.push(...c),a=[...a,...s.map(e=>({...e,isEntrypoint:!1}))]}return{specification:X(a)?.specification,filesystem:a,errors:n}}var Mr=async(e,{shouldLoad:t=!0}={})=>{if(e===null||typeof e==`string`&&e.trim()===``)return console.warn(`[@scalar/oas-utils] Empty OpenAPI document provided.`),{schema:{},errors:[]};let n=e,r=[];if(t){let t=await jr(e).catch(e=>({errors:[{code:e.code,message:e.message}],filesystem:[]}));n=t.filesystem,r=t.errors??[]}let{specification:i}=Ar(n),{schema:a,errors:o=[]}=Tr(i);return{schema:a,errors:[...r,...o]}},Nr=async(e,{shouldLoad:t=!0,dereferencedDocument:n=void 0}={})=>{let{schema:r,errors:i}=n?{schema:n,errors:[]}:await Mr(e??``,{shouldLoad:t});return r||console.warn(`[@scalar/oas-utils] OpenAPI Parser Warning: Schema is undefined`),{schema:Array.isArray(r)?{}:r,errors:i}},Pr={};function Fr(){return{request:Mn.parse({method:`get`,parameters:[],path:``,summary:`My First Request`,examples:[]})}}var Ir=`/* Inter (--scalar-font) */
/* cyrillic-ext */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-cyrillic-ext.woff2) format("woff2");
  unicode-range: U+0460-052F, U+1C80-1C88, U+20B4, U+2DE0-2DFF, U+A640-A69F, U+FE2E-FE2F;
}
/* cyrillic */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-cyrillic.woff2) format("woff2");
  unicode-range: U+0301, U+0400-045F, U+0490-0491, U+04B0-04B1, U+2116;
}
/* greek-ext */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-greek-ext.woff2) format("woff2");
  unicode-range: U+1F00-1FFF;
}
/* greek */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-greek.woff2) format("woff2");
  unicode-range: U+0370-0377, U+037A-037F, U+0384-038A, U+038C, U+038E-03A1, U+03A3-03FF;
}
/* vietnamese */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-vietnamese.woff2) format("woff2");
  unicode-range:
    U+0102-0103, U+0110-0111, U+0128-0129, U+0168-0169,
    U+01A0-01A1, U+01AF-01B0, U+0300-0301, U+0303-0304, U+0308-0309, U+0323,
    U+0329, U+1EA0-1EF9, U+20AB;
}
/* latin-ext */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-latin-ext.woff2) format("woff2");
  unicode-range:
    U+0100-02AF, U+0304, U+0308, U+0329, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C0, U+2113, U+2C60-2C7F,
    U+A720-A7FF;
}
/* latin */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-latin.woff2) format("woff2");
  unicode-range:
    U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA,
    U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+2074, U+20AC, U+2122, U+2191,
    U+2193, U+2212, U+2215, U+FEFF, U+FFFD;
}
/* keyboard symbols (←↑→↓↵⇧⇪⌘⌥) */
@font-face {
  font-family: "Inter";
  font-style: normal;
  font-weight: 100 900;
  font-display: swap;
  src: url(https://fonts.scalar.com/inter-symbols.woff2) format("woff2");
  unicode-range: U+2190-2193, U+21B5, U+21E7, U+21EA, U+2318, U+2325;
}
/* JetBrains Mono (--scalar-font-code) */
/* cyrillic-ext */
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.scalar.com/mono-cyrillic-ext.woff2) format("woff2");
  unicode-range: U+0460-052F, U+1C80-1C88, U+20B4, U+2DE0-2DFF, U+A640-A69F, U+FE2E-FE2F;
}
/* cyrillic */
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.scalar.com/mono-cyrillic.woff2) format("woff2");
  unicode-range: U+0301, U+0400-045F, U+0490-0491, U+04B0-04B1, U+2116;
}
/* greek */
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.scalar.com/mono-greek.woff2) format("woff2");
  unicode-range: U+0370-0377, U+037A-037F, U+0384-038A, U+038C, U+038E-03A1, U+03A3-03FF;
}
/* vietnamese */
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.scalar.com/mono-vietnamese.woff2) format("woff2");
  unicode-range:
    U+0102-0103, U+0110-0111, U+0128-0129, U+0168-0169,
    U+01A0-01A1, U+01AF-01B0, U+0300-0301, U+0303-0304, U+0308-0309, U+0323,
    U+0329, U+1EA0-1EF9, U+20AB;
}
/* latin-ext */
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.scalar.com/mono-latin-ext.woff2) format("woff2");
  unicode-range:
    U+0100-02AF, U+0304, U+0308, U+0329, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C0, U+2113, U+2C60-2C7F,
    U+A720-A7FF;
}
/* latin */
@font-face {
  font-family: "JetBrains Mono";
  font-style: normal;
  font-weight: 400;
  src: url(https://fonts.scalar.com/mono-latin.woff2) format("woff2");
  unicode-range:
    U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA,
    U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+2074, U+20AC, U+2122, U+2191,
    U+2193, U+2212, U+2215, U+FEFF, U+FFFD;
}
`,Lr=`/* basic theme */
:root {
  --scalar-text-decoration: underline;
  --scalar-text-decoration-hover: underline;
}

.dark-mode {
  --scalar-background-1: #131313;
  --scalar-background-2: #1d1d1d;
  --scalar-background-3: #272727;
  --scalar-background-card: #1d1d1d;

  --scalar-color-1: rgba(255, 255, 255, 0.9);
  --scalar-color-2: rgba(255, 255, 255, 0.62);
  --scalar-color-3: rgba(255, 255, 255, 0.44);

  --scalar-color-accent: var(--scalar-color-1);
  --scalar-background-accent: var(--scalar-background-3);

  --scalar-border-color: #2a2b2a;
}

.light-mode,
.light-mode .dark-mode {
  --scalar-background-1: #f9f9f9;
  --scalar-background-2: #f1f1f1;
  --scalar-background-3: #e7e7e7;
  --scalar-background-card: #fff;

  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;

  --scalar-color-accent: var(--scalar-color-1);
  --scalar-background-accent: var(--scalar-background-3);

  --scalar-border-color: rgba(0, 0, 0, 0.1);
}

/* Document Sidebar */
.t-doc__sidebar {
  --scalar-color-green: var(--scalar-color-1);
  --scalar-color-red: var(--scalar-color-1);
  --scalar-color-yellow: var(--scalar-color-1);
  --scalar-color-blue: var(--scalar-color-1);
  --scalar-color-orange: var(--scalar-color-1);
  --scalar-color-purple: var(--scalar-color-1);
}

.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: currentColor;

  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-color-active: var(--scalar-color-accent);

  --scalar-sidebar-search-background: transparent;
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
}
/* advanced */
.light-mode .dark-mode,
.light-mode {
  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #fb892c;
  --scalar-color-purple: #5203d1;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: #00b648;
  --scalar-color-red: #dd2f2c;
  --scalar-color-yellow: #ffc90d;
  --scalar-color-blue: #4eb3ec;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}

.scalar-api-client__item,
.scalar-card,
.dark-mode .dark-mode.scalar-card {
  --scalar-background-1: var(--scalar-background-card);
  --scalar-background-2: var(--scalar-background-1);
  --scalar-background-3: var(--scalar-background-1);
}
.dark-mode .dark-mode.scalar-card {
  --scalar-background-3: var(--scalar-background-3);
}

.light-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-blue), transparent 70%);
}
.dark-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-blue), transparent 50%);
}
`,Rr=`/* basic theme */
:root {
  --scalar-text-decoration: underline;
  --scalar-text-decoration-hover: underline;
}
.light-mode {
  --scalar-background-1: #f0f2f5;
  --scalar-background-2: #eaecf0;
  --scalar-background-3: #e0e2e6;
  --scalar-border-color: rgb(213 213 213);

  --scalar-color-1: rgb(9, 9, 11);
  --scalar-color-2: rgb(113, 113, 122);
  --scalar-color-3: rgba(25, 25, 28, 0.5);

  --scalar-color-accent: var(--scalar-color-1);
  --scalar-background-accent: #8ab4f81f;
}
.light-mode .scalar-card.dark-mode,
.dark-mode {
  --scalar-background-1: #000e23;
  --scalar-background-2: #01132e;
  --scalar-background-3: #03193b;
  --scalar-border-color: #2e394c;

  --scalar-color-1: #fafafa;
  --scalar-color-2: rgb(161, 161, 170);
  --scalar-color-3: rgba(255, 255, 255, 0.533);

  --scalar-color-accent: var(--scalar-color-1);
  --scalar-background-accent: #8ab4f81f;

  --scalar-code-language-color-supersede: var(--scalar-color-1);
}
/* Document Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: currentColor;

  --scalar-sidebar-item-active-background: var(--scalar-background-3);
  --scalar-sidebar-color-active: var(--scalar-color-accent);

  --scalar-sidebar-search-background: rgba(255, 255, 255, 0.1);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
  --scalar-sidebar-search-color: var(--scalar-color-3);
  z-index: 1;
}
.light-mode .t-doc__sidebar {
  --scalar-sidebar-search-background: white;
}
/* advanced */
.light-mode {
  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #fb892c;
  --scalar-color-purple: #5203d1;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: rgba(69, 255, 165, 0.823);
  --scalar-color-red: #ff8589;
  --scalar-color-yellow: #ffcc4d;
  --scalar-color-blue: #6bc1fe;
  --scalar-color-orange: #f98943;
  --scalar-color-purple: #b191f9;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}
/* Custom theme */
/* Document header */
@keyframes headerbackground {
  from {
    background: transparent;
    backdrop-filter: none;
  }
  to {
    background: var(--scalar-header-background-1);
    backdrop-filter: blur(12px);
  }
}

.light-mode .t-doc__header,
.dark-mode .t-doc__header {
  animation: headerbackground forwards;
  animation-timeline: scroll();
  animation-range: 0px 200px;
}

/* Document Layout */
.dark-mode .t-doc .layout-content {
  background: transparent;
}

.dark-mode h2.t-editor__heading,
.dark-mode .t-editor__page-title h1,
.dark-mode h1.section-header:not(::selection),
.dark-mode .markdown h1,
.dark-mode .markdown h2,
.dark-mode .markdown h3,
.dark-mode .markdown h4,
.dark-mode .markdown h5,
.dark-mode .markdown h6 {
  -webkit-text-fill-color: transparent;
  background-image: linear-gradient(to right bottom, rgb(255, 255, 255) 30%, rgba(255, 255, 255, 0.38));
  -webkit-background-clip: text;
  background-clip: text;
}
/* Hero Section Flare */
.section-flare-item:nth-of-type(1) {
  --c1: #ffffff;
  --c2: #babfd8;
  --c3: #2e8bb2;
  --c4: #1a8593;
  --c5: #0a143e;
  --c6: #0a0f52;
  --c7: #2341b8;

  --solid: var(--c1), var(--c2), var(--c3), var(--c4), var(--c5), var(--c6), var(--c7);
  --solid-wrap: var(--solid), var(--c1);
  --trans:
    var(--c1), transparent, var(--c2), transparent, var(--c3),
    transparent, var(--c4), transparent, var(--c5), transparent, var(--c6),
    transparent, var(--c7);
  --trans-wrap: var(--trans), transparent, var(--c1);

  background:
    radial-gradient(circle, var(--trans)), conic-gradient(from 180deg, var(--trans-wrap)),
    radial-gradient(circle, var(--trans)), conic-gradient(var(--solid-wrap));
  width: 70vw;
  height: 700px;
  border-radius: 50%;
  filter: blur(100px);
  z-index: 0;
  right: 0;
  position: absolute;
  transform: rotate(-45deg);
  top: -300px;
  opacity: 0.3;
}
.section-flare-item:nth-of-type(3) {
  --star-color: #6b9acc;
  --star-color2: #446b8d;
  --star-color3: #3e5879;
  background-image:
    radial-gradient(2px 2px at 20px 30px, var(--star-color2), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 40px 70px, var(--star-color), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 50px 160px, var(--star-color3), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 90px 40px, var(--star-color), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 130px 80px, var(--star-color), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 160px 120px, var(--star-color3), rgba(0, 0, 0, 0));
  background-repeat: repeat;
  background-size: 200px 200px;
  width: 100%;
  height: 100%;
  mask-image: radial-gradient(ellipse at 100% 0%, black 40%, transparent 70%);
}
.section-flare {
  top: -150px !important;
  height: 100vh;
  background: linear-gradient(#000, var(--scalar-background-1));
  width: 100vw;
  overflow-x: hidden;
}
.light-mode .section-flare {
  display: none;
}
.light-mode .scalar-card {
  --scalar-background-1: #fff;
  --scalar-background-2: #fff;
  --scalar-background-3: #fff;
}

*::selection {
  background-color: color-mix(in srgb, var(--scalar-color-blue), transparent 60%);
}

@media (max-width: 1000px) {
  .light-mode .t-doc__sidebar,
  .dark-mode .t-doc__sidebar {
    --scalar-sidebar-background-1: var(--scalar-background-1);
  }
  .light-mode .t-doc__header,
  .dark-mode .t-doc__header {
    animation: none;
    background: var(--scalar-header-background-1);
    backdrop-filter: blur(12px);
  }
}
`,zr=`/* basic theme */
:root {
  --scalar-text-decoration: underline;
  --scalar-text-decoration-hover: underline;
}
.light-mode {
  --scalar-color-1: rgb(9, 9, 11);
  --scalar-color-2: rgb(113, 113, 122);
  --scalar-color-3: rgba(25, 25, 28, 0.5);
  --scalar-color-accent: var(--scalar-color-1);

  --scalar-background-1: #fff;
  --scalar-background-2: #f4f4f5;
  --scalar-background-3: #e3e3e6;
  --scalar-background-accent: #8ab4f81f;

  --scalar-border-color: rgb(228, 228, 231);
  --scalar-code-language-color-supersede: var(--scalar-color-1);
}
.dark-mode {
  --scalar-color-1: #fafafa;
  --scalar-color-2: rgb(161, 161, 170);
  --scalar-color-3: rgba(255, 255, 255, 0.533);
  --scalar-color-accent: var(--scalar-color-1);

  --scalar-background-1: #09090b;
  --scalar-background-2: #18181b;
  --scalar-background-3: #2c2c30;
  --scalar-background-accent: #8ab4f81f;

  --scalar-border-color: rgba(255, 255, 255, 0.16);
  --scalar-code-language-color-supersede: var(--scalar-color-1);
}

/* Document Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);

  --scalar-sidebar-item-active-background: var(--scalar-background-3);
  --scalar-sidebar-color-active: var(--scalar-color-accent);

  --scalar-sidebar-search-background: transparent;
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
  --scalar-sidebar-search-color: var(--scalar-color-3);
}
.light-mode .t-doc__sidebar {
  --scalar-sidebar-item-active-background: var(--scalar-background-2);
}
/* advanced */
.light-mode {
  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #fb892c;
  --scalar-color-purple: #5203d1;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: rgba(69, 255, 165, 0.823);
  --scalar-color-red: #ff8589;
  --scalar-color-yellow: #ffcc4d;
  --scalar-color-blue: #6bc1fe;
  --scalar-color-orange: #f98943;
  --scalar-color-purple: #b191f9;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}
/* Custom theme */
.dark-mode h2.t-editor__heading,
.dark-mode .t-editor__page-title h1,
.dark-mode h1.section-header:not(::selection),
.dark-mode .markdown h1,
.dark-mode .markdown h2,
.dark-mode .markdown h3,
.dark-mode .markdown h4,
.dark-mode .markdown h5,
.dark-mode .markdown h6 {
  -webkit-text-fill-color: transparent;
  background-image: linear-gradient(to right bottom, rgb(255, 255, 255) 30%, rgba(255, 255, 255, 0.38));
  -webkit-background-clip: text;
  background-clip: text;
}
.examples .scalar-card-footer {
  --scalar-background-3: transparent;
  padding-top: 0;
}
/* Hero section flare */
.section-flare {
  width: 100vw;
  height: 550px;
  position: absolute;
}
.section-flare-item:nth-of-type(1) {
  position: absolute;
  width: 100vw;
  height: 550px;
  --stripesDark: repeating-linear-gradient(100deg, #000 0%, #000 7%, transparent 10%, transparent 12%, #000 16%);
  --rainbow: repeating-linear-gradient(100deg, #fff 10%, #fff 16%, #fff 22%, #fff 30%);
  background-image: var(--stripesDark), var(--rainbow);
  background-size: 300%, 200%;
  background-position:
    50% 50%,
    50% 50%;
  filter: invert(100%);
  -webkit-mask-image: radial-gradient(ellipse at 100% 0%, black 40%, transparent 70%);
  mask-image: radial-gradient(ellipse at 100% 0%, black 40%, transparent 70%);
  pointer-events: none;
  opacity: 0.07;
}
.dark-mode .section-flare-item:nth-of-type(1) {
  background-image: var(--stripesDark), var(--rainbow);
  filter: opacity(50%) saturate(200%);
  opacity: 0.25;
  height: 350px;
}
.section-flare-item:nth-of-type(1):after {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background-image: var(--stripesDark), var(--rainbow);
  background-size: 200%, 100%;
  background-attachment: fixed;
  mix-blend-mode: difference;
}
.dark-mode .section-flare:after {
  background-image: var(--stripesDark), var(--rainbow);
}
.section-flare-item:nth-of-type(2) {
  --star-color: #fff;
  --star-color2: #fff;
  --star-color3: #fff;
  width: 100%;
  height: 100%;
  position: absolute;
  background-image:
    radial-gradient(2px 2px at 20px 30px, var(--star-color2), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 40px 70px, var(--star-color), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 50px 160px, var(--star-color3), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 90px 40px, var(--star-color), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 130px 80px, var(--star-color), rgba(0, 0, 0, 0)),
    radial-gradient(2px 2px at 160px 120px, var(--star-color3), rgba(0, 0, 0, 0));
  background-repeat: repeat;
  background-size: 200px 200px;
  mask-image: radial-gradient(ellipse at 100% 0%, black 40%, transparent 70%);
  opacity: 0.2;
}
.light-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-blue), transparent 70%);
}
.dark-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-blue), transparent 50%);
}

/* document header */
.light-mode .t-doc__header,
.dark-mode .t-doc__header {
  animation: headerbackground forwards;
  animation-timeline: scroll();
  animation-range: 0px 200px;
}
@keyframes headerbackground {
  from {
    background: transparent;
    backdrop-filter: none;
  }
  to {
    background: var(--scalar-header-background-1);
    backdrop-filter: blur(12px);
  }
}
/* remove flare on safari to prevent dropped frames on scroll */
@supports (-webkit-hyphens: none) {
  .section-flare {
    display: none;
  }
}

/* document background */
.light-mode .t-doc .layout-content,
.dark-mode .t-doc .layout-content {
  background: transparent;
}
`,Br=`/* basic theme */
:root {
  --scalar-text-decoration: underline;
  --scalar-text-decoration-hover: underline;
}
.light-mode {
  --scalar-background-1: #fff;
  --scalar-background-2: #f6f6f6;
  --scalar-background-3: #e7e7e7;
  --scalar-background-accent: #8ab4f81f;

  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;

  --scalar-color-accent: #0099ff;
  --scalar-border-color: #dfdfdf;
}
.dark-mode {
  --scalar-background-1: #0f0f0f;
  --scalar-background-2: #1a1a1a;
  --scalar-background-3: #272727;

  --scalar-color-1: #e7e7e7;
  --scalar-color-2: #a4a4a4;
  --scalar-color-3: #797979;

  --scalar-color-accent: #00aeff;
  --scalar-background-accent: #3ea6ff1f;

  --scalar-border-color: #2d2d2d;
}
/* Document Sidebar */
.light-mode,
.dark-mode {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: var(--scalar-sidebar-color-2);

  --scalar-sidebar-item-active-background: var(--scalar-background-2);
  --scalar-sidebar-color-active: var(--scalar-sidebar-color-1);

  --scalar-sidebar-indent-border: var(--scalar-sidebar-border-color);
  --scalar-sidebar-indent-border-hover: var(--scalar-sidebar-border-color);
  --scalar-sidebar-indent-border-active: var(--scalar-sidebar-border-color);

  --scalar-sidebar-search-background: color-mix(in srgb, var(--scalar-background-2), var(--scalar-background-1));
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
}
/* advanced */
.light-mode {
  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #ff5800;
  --scalar-color-purple: #5203d1;

  --scalar-link-color: var(--scalar-color-1);
  --scalar-link-color-hover: var(--scalar-link-color);

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);

  --scalar-tooltip-background: color-mix(in srgb, #1a1a1a, transparent 10%);
  --scalar-tooltip-color: color-mix(in srgb, #fff, transparent 15%);

  --scalar-color-alert: color-mix(in srgb, var(--scalar-color-orange), var(--scalar-color-1) 20%);
  --scalar-color-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-color-1) 20%);

  --scalar-background-alert: color-mix(in srgb, var(--scalar-color-orange), var(--scalar-background-1) 95%);
  --scalar-background-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-background-1) 95%);
}
.dark-mode {
  --scalar-color-green: #00b648;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #ffc90d;
  --scalar-color-blue: #4eb3ec;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;

  --scalar-link-color: var(--scalar-color-1);
  --scalar-link-color-hover: var(--scalar-link-color);

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;

  --scalar-tooltip-background: color-mix(in srgb, var(--scalar-background-1), #fff 10%);
  --scalar-tooltip-color: color-mix(in srgb, #fff, transparent 5%);

  --scalar-color-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-background-1) 20%);

  --scalar-background-alert: color-mix(in srgb, var(--scalar-color-orange), var(--scalar-background-1) 95%);
  --scalar-background-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-background-1) 95%);
}
@supports (color: color(display-p3 1 1 1)) {
  .light-mode {
    --scalar-color-accent: color(display-p3 0 0.6 1 / 1);
    --scalar-color-green: color(display-p3 0.023529 0.564706 0.380392 / 1);
    --scalar-color-red: color(display-p3 0.937255 0 0.023529 / 1);
    --scalar-color-yellow: color(display-p3 0.929412 0.745098 0.12549 / 1);
    --scalar-color-blue: color(display-p3 0 0.509804 0.815686 / 1);
    --scalar-color-orange: color(display-p3 1 0.4 0.02);
    --scalar-color-purple: color(display-p3 0.321569 0.011765 0.819608 / 1);
  }
  .dark-mode {
    --scalar-color-accent: color(display-p3 0.07 0.67 1);
    --scalar-color-green: color(display-p3 0 0.713725 0.282353 / 1);
    --scalar-color-red: color(display-p3 0.862745 0.105882 0.098039 / 1);
    --scalar-color-yellow: color(display-p3 1 0.788235 0.05098 / 1);
    --scalar-color-blue: color(display-p3 0.305882 0.701961 0.92549 / 1);
    --scalar-color-orange: color(display-p3 1 0.552941 0.301961 / 1);
    --scalar-color-purple: color(display-p3 0.694118 0.568627 0.976471 / 1);
  }
}
`,Vr=`.light-mode {
  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;
  --scalar-color-accent: #f06292;

  --scalar-background-1: #fff;
  --scalar-background-2: #f6f6f6;
  --scalar-background-3: #e7e7e7;

  --scalar-border-color: rgba(0, 0, 0, 0.1);
}
.dark-mode {
  --scalar-color-1: rgba(255, 255, 255, 0.9);
  --scalar-color-2: rgba(156, 163, 175, 1);
  --scalar-color-3: rgba(255, 255, 255, 0.44);
  --scalar-color-accent: #f06292;

  --scalar-background-1: #111728;
  --scalar-background-2: #1e293b;
  --scalar-background-3: #334155;
  --scalar-background-accent: #f062921f;

  --scalar-border-color: rgba(255, 255, 255, 0.1);
}

/* Document Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: currentColor;

  --scalar-sidebar-item-active-background: #f062921f;
  --scalar-sidebar-color-active: var(--scalar-color-accent);

  --scalar-sidebar-search-background: transparent;
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
}

/* advanced */
.light-mode {
  --scalar-button-1: rgb(49 53 56);
  --scalar-button-1-color: #fff;
  --scalar-button-1-hover: rgb(28 31 33);

  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #fb892c;
  --scalar-color-purple: #5203d1;

  --scalar-scrollbar-color: rgba(0, 0, 0, 0.18);
  --scalar-scrollbar-color-active: rgba(0, 0, 0, 0.36);
}
.dark-mode {
  --scalar-button-1: #f6f6f6;
  --scalar-button-1-color: #000;
  --scalar-button-1-hover: #e7e7e7;

  --scalar-color-green: #a3ffa9;
  --scalar-color-red: #ffa3a3;
  --scalar-color-yellow: #fffca3;
  --scalar-color-blue: #a5d6ff;
  --scalar-color-orange: #e2ae83;
  --scalar-color-purple: #d2a8ff;

  --scalar-scrollbar-color: rgba(255, 255, 255, 0.24);
  --scalar-scrollbar-color-active: rgba(255, 255, 255, 0.48);
}
.section-flare {
  width: 100%;
  height: 400px;
  position: absolute;
}
.section-flare-item:first-of-type:before {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  --stripes: repeating-linear-gradient(100deg, #fff 0%, #fff 0%, transparent 2%, transparent 12%, #fff 17%);
  --stripesDark: repeating-linear-gradient(100deg, #000 0%, #000 0%, transparent 10%, transparent 12%, #000 17%);
  --rainbow: repeating-linear-gradient(100deg, #60a5fa 10%, #e879f9 16%, #5eead4 22%, #60a5fa 30%);
  contain: strict;
  contain-intrinsic-size: 100vw 40vh;
  background-image: var(--stripesDark), var(--rainbow);
  background-size: 300%, 200%;
  background-position:
    50% 50%,
    50% 50%;
  filter: opacity(20%) saturate(200%);
  -webkit-mask-image: radial-gradient(ellipse at 100% 0%, black 40%, transparent 70%);
  mask-image: radial-gradient(ellipse at 100% 0%, black 40%, transparent 70%);
  pointer-events: none;
}
.section-flare-item:first-of-type:after {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  background-image: var(--stripes), var(--rainbow);
  background-size: 200%, 100%;
  background-attachment: fixed;
  mix-blend-mode: difference;
  background-image: var(--stripesDark), var(--rainbow);
  pointer-events: none;
}
.light-mode .section-flare-item:first-of-type:after,
.light-mode .section-flare-item:first-of-type:before {
  background-image: var(--stripes), var(--rainbow);
  filter: opacity(4%) saturate(200%);
}
`,Hr=`.light-mode {
  color-scheme: light;
  --scalar-color-1: #1c1e21;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;
  --scalar-color-disabled: #b4b1b1;
  --scalar-color-ghost: #a7a7a7;
  --scalar-color-accent: #2f8555;
  --scalar-background-1: #fff;
  --scalar-background-2: #f5f5f5;
  --scalar-background-3: #ededed;
  --scalar-background-4: rgba(0, 0, 0, 0.06);
  --scalar-background-accent: #2f85551f;

  --scalar-border-color: rgba(0, 0, 0, 0.1);
  --scalar-scrollbar-color: rgba(0, 0, 0, 0.18);
  --scalar-scrollbar-color-active: rgba(0, 0, 0, 0.36);
  --scalar-lifted-brightness: 1;
  --scalar-backdrop-brightness: 1;

  --scalar-shadow-1: 0 1px 3px 0 rgba(0, 0, 0, 0.11);
  --scalar-shadow-2: rgba(0, 0, 0, 0.08) 0px 13px 20px 0px, rgba(0, 0, 0, 0.08) 0px 3px 8px 0px, #eeeeed 0px 0 0 1px;

  --scalar-button-1: rgb(49 53 56);
  --scalar-button-1-color: #fff;
  --scalar-button-1-hover: rgb(28 31 33);

  --scalar-color-green: #007300;
  --scalar-color-red: #af272b;
  --scalar-color-yellow: #b38200;
  --scalar-color-blue: #3b8ba5;
  --scalar-color-orange: #fb892c;
  --scalar-color-purple: #5203d1;
}

.dark-mode {
  color-scheme: dark;
  --scalar-color-1: rgba(255, 255, 255, 0.9);
  --scalar-color-2: rgba(255, 255, 255, 0.62);
  --scalar-color-3: rgba(255, 255, 255, 0.44);
  --scalar-color-disabled: rgba(255, 255, 255, 0.34);
  --scalar-color-ghost: rgba(255, 255, 255, 0.26);
  --scalar-color-accent: #27c2a0;
  --scalar-background-1: #1b1b1d;
  --scalar-background-2: #242526;
  --scalar-background-3: #3b3b3b;
  --scalar-background-4: rgba(255, 255, 255, 0.06);
  --scalar-background-accent: #27c2a01f;

  --scalar-border-color: rgba(255, 255, 255, 0.1);
  --scalar-scrollbar-color: rgba(255, 255, 255, 0.24);
  --scalar-scrollbar-color-active: rgba(255, 255, 255, 0.48);
  --scalar-lifted-brightness: 1.45;
  --scalar-backdrop-brightness: 0.5;

  --scalar-shadow-1: 0 1px 3px 0 rgb(0, 0, 0, 0.1);
  --scalar-shadow-2:
    rgba(15, 15, 15, 0.2) 0px 3px 6px, rgba(15, 15, 15, 0.4) 0px 9px 24px, 0 0 0 1px rgba(255, 255, 255, 0.1);

  --scalar-button-1: #f6f6f6;
  --scalar-button-1-color: #000;
  --scalar-button-1-hover: #e7e7e7;

  --scalar-color-green: #26b226;
  --scalar-color-red: #fb565b;
  --scalar-color-yellow: #ffc426;
  --scalar-color-blue: #6ecfef;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;
}
`,Ur=`/* basic theme */
.light-mode {
  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;
  --scalar-color-accent: #7070ff;

  --scalar-background-1: #fff;
  --scalar-background-2: #f6f6f6;
  --scalar-background-3: #e7e7e7;
  --scalar-background-accent: #7070ff1f;

  --scalar-border-color: rgba(0, 0, 0, 0.1);

  --scalar-code-language-color-supersede: var(--scalar-color-3);
}
.dark-mode {
  --scalar-color-1: #f7f8f8;
  --scalar-color-2: rgb(180, 188, 208);
  --scalar-color-3: #b4bcd099;
  --scalar-color-accent: #828fff;

  --scalar-background-1: #000212;
  --scalar-background-2: #0d0f1e;
  --scalar-background-3: #232533;
  --scalar-background-accent: #8ab4f81f;

  --scalar-border-color: #313245;
  --scalar-code-language-color-supersede: var(--scalar-color-3);
}
/* Document Sidebar */
.light-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-border-color: var(--scalar-border-color);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-color-active: var(--scalar-color-accent);
  --scalar-sidebar-search-background: rgba(0, 0, 0, 0.05);
  --scalar-sidebar-search-border-color: 1px solid rgba(0, 0, 0, 0.05);
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-background-2: rgba(0, 0, 0, 0.03);
}
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-active-background: rgba(255, 255, 255, 0.1);
  --scalar-sidebar-border-color: var(--scalar-border-color);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-color-active: var(--scalar-color-accent);
  --scalar-sidebar-search-background: rgba(255, 255, 255, 0.1);
  --scalar-sidebar-search-border-color: 1px solid rgba(255, 255, 255, 0.05);
  --scalar-sidebar-search-color: var(--scalar-color-3);
}
/* advanced */
.light-mode {
  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #fb892c;
  --scalar-color-purple: #5203d1;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: #00b648;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #ffc90d;
  --scalar-color-blue: #4eb3ec;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}
/* Custom Theme */
.dark-mode h2.t-editor__heading,
.dark-mode .t-editor__page-title h1,
.dark-mode h1.section-header:not(::selection),
.dark-mode .markdown h1,
.dark-mode .markdown h2,
.dark-mode .markdown h3,
.dark-mode .markdown h4,
.dark-mode .markdown h5,
.dark-mode .markdown h6 {
  -webkit-text-fill-color: transparent;
  background-image: linear-gradient(to right bottom, rgb(255, 255, 255) 30%, rgba(255, 255, 255, 0.38));
  -webkit-background-clip: text;
  background-clip: text;
}
.sidebar-search {
  backdrop-filter: blur(12px);
}
@keyframes headerbackground {
  from {
    background: transparent;
    backdrop-filter: none;
  }
  to {
    background: var(--scalar-header-background-1);
    backdrop-filter: blur(12px);
  }
}
.dark-mode .scalar-card {
  background: rgba(255, 255, 255, 0.05) !important;
}
.dark-mode .scalar-card * {
  --scalar-background-2: transparent !important;
  --scalar-background-1: transparent !important;
}
.light-mode .dark-mode.scalar-card *,
.light-mode .dark-mode.scalar-card {
  --scalar-background-1: #0d0f1e !important;
  --scalar-background-2: #0d0f1e !important;
  --scalar-background-3: #191b29 !important;
}
.light-mode .dark-mode.scalar-card {
  background: #191b29 !important;
}
.badge {
  box-shadow: 0 0 0 1px var(--scalar-border-color);
  margin-right: 6px;
}

.table-row.required-parameter .table-row-item:nth-of-type(2):after {
  background: transparent;
  box-shadow: none;
}
/* Hero Section Flare */
.section-flare {
  width: 100vw;
  background: radial-gradient(ellipse 80% 50% at 50% -20%, rgba(120, 119, 198, 0.3), transparent);
  height: 100vh;
}
.light-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 70%);
}
.dark-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 50%);
}

/* document layout */
.light-mode .t-doc .layout-content,
.dark-mode .t-doc .layout-content {
  background: transparent;
}
`,Wr=`/* basic theme */
.light-mode {
  color-scheme: light;
  --scalar-color-1: #322b3b;
  --scalar-color-2: #645676;
  --scalar-color-3: #9789a9;
  --scalar-color-accent: #40b4c4;

  --scalar-background-1: #fff;
  --scalar-background-2: #f4f2f7;
  --scalar-background-3: #cfc7dc;
  --scalar-background-accent: #f3fafb;

  --scalar-border-color: #e4e0eb;
}
.dark-mode {
  color-scheme: dark;
  --scalar-color-1: #fff;
  --scalar-color-2: #b8b6ba;
  --scalar-color-3: #706c74;
  --scalar-color-accent: #ed78c2;

  --scalar-background-1: #27212e;
  --scalar-background-2: #322c39;
  --scalar-background-3: #4c4059;
  --scalar-background-accent: #eb64b91f;

  --scalar-border-color: rgba(255, 255, 255, 0.1);
}

/* Sidebar */
.light-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-border-color: var(--scalar-border-color);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-color-active: var(--scalar-color-accent);
  --scalar-sidebar-search-background: var(--scalar-background-2);
  --scalar-sidebar-search-border-color: var(--scalar-sidebar-border-color);
  --scalar-sidebar-search--color: var(--scalar-color-3);
}
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-border-color: var(--scalar-border-color);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-color-active: var(--scalar-color-accent);
  --scalar-sidebar-search-background: var(--scalar-background-2);
  --scalar-sidebar-search-border-color: #514c56;
  --scalar-sidebar-search--color: var(--scalar-color-3);
}
/* advanced */
.light-mode {
  --scalar-button-1: rgb(49 53 56);
  --scalar-button-1-color: #fff;
  --scalar-button-1-hover: rgb(28 31 33);

  --scalar-color-green: #74dfc4;
  --scalar-color-red: #d887f5;
  --scalar-color-yellow: #ffe261;
  --scalar-color-blue: #40b4c4;
  --scalar-color-orange: #ff52bf;
  --scalar-color-purple: #91889b;

  --scalar-scrollbar-color: rgba(0, 0, 0, 0.18);
  --scalar-scrollbar-color-active: rgba(0, 0, 0, 0.36);
}
.dark-mode {
  --scalar-button-1: #f6f6f6;
  --scalar-button-1-color: #27212e;
  --scalar-button-1-hover: #e7e7e7;

  --scalar-color-green: #74dfc4;
  --scalar-color-red: #d887f5;
  --scalar-color-yellow: #ffe261;
  --scalar-color-blue: #40b4c4;
  --scalar-color-orange: #ff52bf;
  --scalar-color-purple: #91889b;

  --scalar-scrollbar-color: rgba(255, 255, 255, 0.24);
  --scalar-scrollbar-color-active: rgba(255, 255, 255, 0.48);
}
/* Radius */
:root {
  --scalar-radius: 2px;
  --scalar-radius-lg: 3px;
  --scalar-radius-xl: 4px;
}
/* P3 color support */
@supports (color: color(display-p3 1 1 1)) {
  .light-mode {
    --scalar-color-accent: color(display-p3 0.25098 0.705882 0.768627 / 1);
    --scalar-color-green: color(display-p3 0.454902 0.87451 0.768627 / 1);
    --scalar-color-red: color(display-p3 0.847059 0.529412 0.960784 / 1);
    --scalar-color-yellow: color(display-p3 1 0.886275 0.380392 / 1);
    --scalar-color-blue: color(display-p3 0.25098 0.705882 0.768627 / 1);
    --scalar-color-orange: color(display-p3 1 0.321569 0.74902 / 1);
    --scalar-color-purple: color(display-p3 0.568627 0.533333 0.607843 / 1);
  }
  .dark-mode {
    --scalar-color-accent: color(display-p3 0.929412 0.470588 0.760784 / 1);
    --scalar-color-green: color(display-p3 0.454902 0.87451 0.768627 / 1);
    --scalar-color-red: color(display-p3 0.847059 0.529412 0.960784 / 1);
    --scalar-color-yellow: color(display-p3 1 0.886275 0.380392 / 1);
    --scalar-color-blue: color(display-p3 0.25098 0.705882 0.768627 / 1);
    --scalar-color-orange: color(display-p3 1 0.321569 0.74902 / 1);
    --scalar-color-purple: color(display-p3 0.568627 0.533333 0.607843 / 1);
  }
}
`,Gr=`/* basic theme */
:root {
  --scalar-text-decoration: underline;
  --scalar-text-decoration-hover: underline;
}
.light-mode {
  --scalar-background-1: #f9f6f0;
  --scalar-background-2: #f2efe8;
  --scalar-background-3: #e9e7e2;
  --scalar-border-color: rgba(203, 165, 156, 0.6);

  --scalar-color-1: #c75549;
  --scalar-color-2: #c75549;
  --scalar-color-3: #c75549;

  --scalar-color-accent: #c75549;
  --scalar-background-accent: #dcbfa81f;

  --scalar-code-language-color-supersede: var(--scalar-color-1);
}
.dark-mode {
  --scalar-background-1: #140507;
  --scalar-background-2: #20090c;
  --scalar-background-3: #321116;
  --scalar-border-color: #3c3031;

  --scalar-color-1: rgba(255, 255, 255, 0.9);
  --scalar-color-2: rgba(255, 255, 255, 0.62);
  --scalar-color-3: rgba(255, 255, 255, 0.44);

  --scalar-color-accent: rgba(255, 255, 255, 0.9);
  --scalar-background-accent: #441313;

  --scalar-code-language-color-supersede: var(--scalar-color-1);
}

/* Document Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);

  --scalar-sidebar-item-active-background: var(--scalar-background-3);
  --scalar-sidebar-color-active: var(--scalar-color-accent);

  --scalar-sidebar-search-background: rgba(255, 255, 255, 0.1);
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
  z-index: 1;
}
/* advanced */
.light-mode {
  --scalar-color-green: #09533a;
  --scalar-color-red: #aa181d;
  --scalar-color-yellow: #ab8d2b;
  --scalar-color-blue: #19689a;
  --scalar-color-orange: #b26c34;
  --scalar-color-purple: #4c2191;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: rgba(69, 255, 165, 0.823);
  --scalar-color-red: #ff8589;
  --scalar-color-yellow: #ffcc4d;
  --scalar-color-blue: #6bc1fe;
  --scalar-color-orange: #f98943;
  --scalar-color-purple: #b191f9;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}
/* Custom Theme */
.dark-mode h2.t-editor__heading,
.dark-mode .t-editor__page-title h1,
.dark-mode h1.section-header:not(::selection),
.dark-mode .markdown h1,
.dark-mode .markdown h2,
.dark-mode .markdown h3,
.dark-mode .markdown h4,
.dark-mode .markdown h5,
.dark-mode .markdown h6 {
  -webkit-text-fill-color: transparent;
  background-image: linear-gradient(to right bottom, rgb(255, 255, 255) 30%, rgba(255, 255, 255, 0.38));
  -webkit-background-clip: text;
  background-clip: text;
}
.light-mode .t-doc__sidebar {
  --scalar-sidebar-search-background: white;
}
.examples .scalar-card-footer {
  --scalar-background-3: transparent;
  padding-top: 0;
}
/* Hero section flare */
.section-flare {
  overflow-x: hidden;
  height: 100vh;
  left: initial;
}
.section-flare-item:nth-of-type(1) {
  background: #d25019;
  position: relative;
  top: -150px;
  right: -400px;
  width: 80vw;
  height: 500px;
  margin-top: -150px;
  border-radius: 50%;
  filter: blur(100px);
  z-index: 0;
}
.light-mode .section-flare {
  display: none;
}
*::selection {
  background-color: color-mix(in srgb, var(--scalar-color-red), transparent 75%);
}

/* document layout */
.dark-mode .t-doc .layout-content {
  background: transparent;
}
`,Kr=`.light-mode {
  color-scheme: light;
  --scalar-color-1: #000000;
  --scalar-color-2: #000000;
  --scalar-color-3: #000000;
  --scalar-color-accent: #645b0f;
  --scalar-background-1: #ccc9b3;
  --scalar-background-2: #c2bfaa;
  --scalar-background-3: #b8b5a1;
  --scalar-background-accent: #000000;

  --scalar-border-color: rgba(0, 0, 0, 0.2);
  --scalar-scrollbar-color: rgba(0, 0, 0, 0.18);
  --scalar-scrollbar-color-active: rgba(0, 0, 0, 0.36);
  --scalar-lifted-brightness: 1;
  --scalar-backdrop-brightness: 1;

  --scalar-shadow-1: 0 1px 3px 0 rgba(0, 0, 0, 0.11);
  --scalar-shadow-2:
    rgba(0, 0, 0, 0.08) 0px 13px 20px 0px, rgba(0, 0, 0, 0.08) 0px 3px 8px 0px, var(--scalar-border-color) 0px 0 0 1px;

  --scalar-button-1: rgb(49 53 56);
  --scalar-button-1-color: #fff;
  --scalar-button-1-hover: rgb(28 31 33);

  --scalar-color-red: #b91c1c;
  --scalar-color-orange: #a16207;
  --scalar-color-green: #047857;
  --scalar-color-blue: #1d4ed8;
  --scalar-color-orange: #c2410c;
  --scalar-color-purple: #6d28d9;
}

.dark-mode {
  color-scheme: dark;
  --scalar-color-1: #fffef3;
  --scalar-color-2: #fffef3;
  --scalar-color-3: #fffef3;
  --scalar-color-accent: #c3b531;
  --scalar-background-1: #313332;
  --scalar-background-2: #393b3a;
  --scalar-background-3: #414342;
  --scalar-background-accent: #fffef3;

  --scalar-border-color: #505452;
  --scalar-scrollbar-color: rgba(255, 255, 255, 0.24);
  --scalar-scrollbar-color-active: rgba(255, 255, 255, 0.48);
  --scalar-lifted-brightness: 1.45;
  --scalar-backdrop-brightness: 0.5;

  --scalar-shadow-1: 0 1px 3px 0 rgba(0, 0, 0, 0.11);
  --scalar-shadow-2:
    rgba(15, 15, 15, 0.2) 0px 3px 6px, rgba(15, 15, 15, 0.4) 0px 9px 24px, 0 0 0 1px rgba(255, 255, 255, 0.1);

  --scalar-button-1: #f6f6f6;
  --scalar-button-1-color: #000;
  --scalar-button-1-hover: #e7e7e7;

  --scalar-color-green: #00b648;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #ffc90d;
  --scalar-color-blue: #4eb3ec;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;
}

/* Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: currentColor;

  --scalar-sidebar-item-active-background: var(--scalar-background-3);
  --scalar-sidebar-color-active: var(--scalar-color-1);

  --scalar-sidebar-search-background: transparent;
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
}
*::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 80%);
}
`,qr=`/* basic theme */
.light-mode {
  --scalar-background-1: #fff;
  --scalar-background-2: #f5f6f8;
  --scalar-background-3: #eceef1;

  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;

  --scalar-color-accent: #5469d4;
  --scalar-background-accent: #5469d41f;

  --scalar-border-color: rgba(215, 215, 206, 0.68);
}
.dark-mode {
  --scalar-background-1: #15171c;
  --scalar-background-2: #1c1e24;
  --scalar-background-3: #22252b;

  --scalar-color-1: #fafafa;
  --scalar-color-2: #c9ced8;
  --scalar-color-3: #8c99ad;

  --scalar-color-accent: #5469d4;
  --scalar-background-accent: #5469d41f;

  --scalar-border-color: #3f4145;
}
/* Document Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-3);

  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-color-active: var(--scalar-color-accent);

  --scalar-sidebar-search-background: var(--scalar-background-1);
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
}

/* advanced */
.light-mode {
  --scalar-color-green: #17803d;
  --scalar-color-red: #e10909;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #1763a6;
  --scalar-color-orange: #e25b09;
  --scalar-color-purple: #5c3993;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: #30a159;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #eec644;
  --scalar-color-blue: #2b7abf;
  --scalar-color-orange: #f07528;
  --scalar-color-purple: #7a59b1;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}
.light-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 70%);
}
.dark-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 50%);
}
`,Jr=`/* basic theme */
.light-mode {
  --scalar-background-1: #f3f3ee;
  --scalar-background-2: #e8e8e3;
  --scalar-background-3: #e4e4df;
  --scalar-border-color: rgba(215, 215, 206, 0.85);

  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;

  --scalar-color-accent: #1763a6;
  --scalar-background-accent: #1f648e1f;
}
.dark-mode {
  --scalar-background-1: #09090b;
  --scalar-background-2: #18181b;
  --scalar-background-3: #2c2c30;
  --scalar-border-color: rgba(255, 255, 255, 0.17);

  --scalar-color-1: #fafafa;
  --scalar-color-2: rgb(161, 161, 170);
  --scalar-color-3: rgba(255, 255, 255, 0.533);

  --scalar-color-accent: #4eb3ec;
  --scalar-background-accent: #8ab4f81f;
}
/* Document Sidebar */
.light-mode .t-doc__sidebar,
.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: currentColor;

  --scalar-sidebar-item-active-background: var(--scalar-background-3);
  --scalar-sidebar-color-active: var(--scalar-color-1);

  --scalar-sidebar-search-background: var(--scalar-background-1);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
  --scalar-sidebar-search-color: var(--scalar-color-3);
}

/* advanced */
.light-mode {
  --scalar-color-green: #17803d;
  --scalar-color-red: #e10909;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #1763a6;
  --scalar-color-orange: #e25b09;
  --scalar-color-purple: #5c3993;

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);
}
.dark-mode {
  --scalar-color-green: #30a159;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #eec644;
  --scalar-color-blue: #2b7abf;
  --scalar-color-orange: #f07528;
  --scalar-color-purple: #7a59b1;

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;
}
.dark-mode h2.t-editor__heading,
.dark-mode .t-editor__page-title h1,
.dark-mode h1.section-header:not(::selection),
.dark-mode .markdown h1,
.dark-mode .markdown h2,
.dark-mode .markdown h3,
.dark-mode .markdown h4,
.dark-mode .markdown h5,
.dark-mode .markdown h6 {
  -webkit-text-fill-color: transparent;
  background-image: linear-gradient(to right bottom, rgb(255, 255, 255) 30%, rgba(255, 255, 255, 0.38));
  -webkit-background-clip: text;
  background-clip: text;
}
.light-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 70%);
}
.dark-mode *::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 50%);
}
`,Yr=`.light-mode {
  color-scheme: light;
  --scalar-color-1: #584c27;
  --scalar-color-2: #616161;
  --scalar-color-3: #a89f84;
  --scalar-color-accent: #b58900;
  --scalar-background-1: #fdf6e3;
  --scalar-background-2: #eee8d5;
  --scalar-background-3: #ddd6c1;
  --scalar-background-accent: #b589001f;

  --scalar-border-color: #ded8c8;
  --scalar-scrollbar-color: rgba(0, 0, 0, 0.18);
  --scalar-scrollbar-color-active: rgba(0, 0, 0, 0.36);
  --scalar-lifted-brightness: 1;
  --scalar-backdrop-brightness: 1;

  --scalar-shadow-1: 0 1px 3px 0 rgba(0, 0, 0, 0.11);
  --scalar-shadow-2: rgba(0, 0, 0, 0.08) 0px 13px 20px 0px, rgba(0, 0, 0, 0.08) 0px 3px 8px 0px, #eeeeed 0px 0 0 1px;

  --scalar-button-1: rgb(49 53 56);
  --scalar-button-1-color: #fff;
  --scalar-button-1-hover: rgb(28 31 33);

  --scalar-color-red: #b91c1c;
  --scalar-color-orange: #a16207;
  --scalar-color-green: #047857;
  --scalar-color-blue: #1d4ed8;
  --scalar-color-orange: #c2410c;
  --scalar-color-purple: #6d28d9;
}

.dark-mode {
  color-scheme: dark;
  --scalar-color-1: #fff;
  --scalar-color-2: #cccccc;
  --scalar-color-3: #6d8890;
  --scalar-color-accent: #007acc;
  --scalar-background-1: #00212b;
  --scalar-background-2: #012b36;
  --scalar-background-3: #004052;
  --scalar-background-accent: #015a6f;

  --scalar-border-color: #2f4851;
  --scalar-scrollbar-color: rgba(255, 255, 255, 0.24);
  --scalar-scrollbar-color-active: rgba(255, 255, 255, 0.48);
  --scalar-lifted-brightness: 1.45;
  --scalar-backdrop-brightness: 0.5;

  --scalar-shadow-1: 0 1px 3px 0 rgb(0, 0, 0, 0.1);
  --scalar-shadow-2:
    rgba(15, 15, 15, 0.2) 0px 3px 6px, rgba(15, 15, 15, 0.4) 0px 9px 24px, 0 0 0 1px rgba(255, 255, 255, 0.1);

  --scalar-button-1: #f6f6f6;
  --scalar-button-1-color: #000;
  --scalar-button-1-hover: #e7e7e7;

  --scalar-color-green: #00b648;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #ffc90d;
  --scalar-color-blue: #4eb3ec;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;
}

/* Sidebar */
.light-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-border-color: var(--scalar-border-color);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-color-active: var(--scalar-color-accent);
  --scalar-sidebar-search-background: var(--scalar-background-2);
  --scalar-sidebar-search-border-color: var(--scalar-sidebar-search-background);
  --scalar-sidebar-search--color: var(--scalar-color-3);
}

.dark-mode .t-doc__sidebar {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-item-hover-color: currentColor;
  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-active-background: var(--scalar-background-accent);
  --scalar-sidebar-border-color: var(--scalar-border-color);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-color-active: var(--scalar-sidebar-color-1);
  --scalar-sidebar-search-background: var(--scalar-background-2);
  --scalar-sidebar-search-border-color: var(--scalar-sidebar-search-background);
  --scalar-sidebar-search--color: var(--scalar-color-3);
}
*::selection {
  background-color: color-mix(in srgb, var(--scalar-color-accent), transparent 70%);
}
`;function Xr(){if(typeof window>`u`)return!1;let e=document.createElement(`div`);e.setAttribute(`style`,`width:30px;height:30px;overflow-y:scroll;`),e.classList.add(`scrollbar-test`);let t=document.createElement(`div`);t.setAttribute(`style`,`width:100%;height:40px`),e.appendChild(t),document.body.appendChild(e);let n=30-e.firstChild.clientWidth;return document.body.removeChild(e),!!n}var Zr=[`alternate`,`default`,`moon`,`purple`,`solarized`,`bluePlanet`,`deepSpace`,`saturn`,`kepler`,`elysiajs`,`fastify`,`mars`,`laserwave`,`none`],Qr={default:`Default`,alternate:`Alternate`,moon:`Moon`,purple:`Purple`,solarized:`Solarized`,elysiajs:`Elysia.js`,fastify:`Fastify`,bluePlanet:`Blue Planet`,saturn:`Saturn`,kepler:`Kepler-11e`,mars:`Mars`,deepSpace:`Deep Space`,laserwave:`Laserwave`,none:`None`},$={default:{uid:`qTQR9jSM8E-LihpyZzPOi`,name:`Default`,description:`Default Scalar theme`,theme:Br,slug:`default`},alternate:{uid:`2skUDSH4S8HYFF9yXysr-`,name:`Alternate`,description:`Alternate Scalar theme`,theme:Lr,slug:`alternate`},moon:{uid:`DG9ZUNp5lJhDeX_kPX4Bl`,name:`Moon`,description:`Lunar styles`,theme:Kr,slug:`moon`},purple:{uid:`pE_1ysxcZ-y2LM1GGNBUv`,name:`Purple`,description:`Purple Scalar theme`,theme:qr,slug:`purple`},solarized:{uid:`BdGVG1vf-4nYl3wJKyj8l`,name:`Solarized`,description:`Solarized Scalar theme`,theme:Yr,slug:`solarized`},bluePlanet:{uid:`X12IfAvl7ue-42V2lW40S`,name:`Blue Planet`,description:`Blue Planet Scalar theme`,theme:Rr,slug:`blue-planet`},deepSpace:{uid:`K8b38NWQiicq4-zXGXKdI`,name:`Deep Space`,description:`Deep Space Scalar theme`,theme:zr,slug:`deep-space`},saturn:{uid:`1jyAjmbIZQG-RUU4Ugk9o`,name:`Saturn`,description:`Saturn Scalar theme`,theme:Jr,slug:`saturn`},kepler:{uid:`jZ6dnWbtqQ0Hz3s9jLPH0`,name:`Kepler-11e`,description:`Kepler-11e Scalar theme`,theme:Ur,slug:`kepler-11e`},mars:{uid:`YY4LQgwiXix55-TmMz9qd`,name:`Mars`,description:`Mars Scalar theme`,theme:Gr,slug:`mars`},laserwave:{uid:`c5fZEi-K-hP-xXf885dkf`,name:`Laserwave`,description:`Laserwave Scalar theme`,theme:Wr,slug:`laserwave`},elysiajs:{uid:`nEVZkRmCylPkT0o9YJa7y`,name:`Elysia.js`,description:`Elysia.js theme`,theme:Vr,slug:`elysiajs`},fastify:{uid:`nTZcdcM2_yHFZFxTQe9Kk`,name:`Fastify`,description:`Fastify theme`,theme:Hr,slug:`fastify`}};Object.values($);var $r=(e,t)=>{let{fonts:n=!0,layer:r=`scalar-theme`}=t??{},i=[$[e||`default`]?.theme??`/* basic theme */
:root {
  --scalar-text-decoration: underline;
  --scalar-text-decoration-hover: underline;
}
.light-mode {
  --scalar-background-1: #fff;
  --scalar-background-2: #f6f6f6;
  --scalar-background-3: #e7e7e7;
  --scalar-background-accent: #8ab4f81f;

  --scalar-color-1: #1b1b1b;
  --scalar-color-2: #757575;
  --scalar-color-3: #8e8e8e;

  --scalar-color-accent: #0099ff;
  --scalar-border-color: #dfdfdf;
}
.dark-mode {
  --scalar-background-1: #0f0f0f;
  --scalar-background-2: #1a1a1a;
  --scalar-background-3: #272727;

  --scalar-color-1: #e7e7e7;
  --scalar-color-2: #a4a4a4;
  --scalar-color-3: #797979;

  --scalar-color-accent: #00aeff;
  --scalar-background-accent: #3ea6ff1f;

  --scalar-border-color: #2d2d2d;
}
/* Document Sidebar */
.light-mode,
.dark-mode {
  --scalar-sidebar-background-1: var(--scalar-background-1);
  --scalar-sidebar-color-1: var(--scalar-color-1);
  --scalar-sidebar-color-2: var(--scalar-color-2);
  --scalar-sidebar-border-color: var(--scalar-border-color);

  --scalar-sidebar-item-hover-background: var(--scalar-background-2);
  --scalar-sidebar-item-hover-color: var(--scalar-sidebar-color-2);

  --scalar-sidebar-item-active-background: var(--scalar-background-2);
  --scalar-sidebar-color-active: var(--scalar-sidebar-color-1);

  --scalar-sidebar-indent-border: var(--scalar-sidebar-border-color);
  --scalar-sidebar-indent-border-hover: var(--scalar-sidebar-border-color);
  --scalar-sidebar-indent-border-active: var(--scalar-sidebar-border-color);

  --scalar-sidebar-search-background: color-mix(in srgb, var(--scalar-background-2), var(--scalar-background-1));
  --scalar-sidebar-search-color: var(--scalar-color-3);
  --scalar-sidebar-search-border-color: var(--scalar-border-color);
}
/* advanced */
.light-mode {
  --scalar-color-green: #069061;
  --scalar-color-red: #ef0006;
  --scalar-color-yellow: #edbe20;
  --scalar-color-blue: #0082d0;
  --scalar-color-orange: #ff5800;
  --scalar-color-purple: #5203d1;

  --scalar-link-color: var(--scalar-color-1);
  --scalar-link-color-hover: var(--scalar-link-color);

  --scalar-button-1: rgba(0, 0, 0, 1);
  --scalar-button-1-hover: rgba(0, 0, 0, 0.8);
  --scalar-button-1-color: rgba(255, 255, 255, 0.9);

  --scalar-tooltip-background: color-mix(in srgb, #1a1a1a, transparent 10%);
  --scalar-tooltip-color: color-mix(in srgb, #fff, transparent 15%);

  --scalar-color-alert: color-mix(in srgb, var(--scalar-color-orange), var(--scalar-color-1) 20%);
  --scalar-color-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-color-1) 20%);

  --scalar-background-alert: color-mix(in srgb, var(--scalar-color-orange), var(--scalar-background-1) 95%);
  --scalar-background-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-background-1) 95%);
}
.dark-mode {
  --scalar-color-green: #00b648;
  --scalar-color-red: #dc1b19;
  --scalar-color-yellow: #ffc90d;
  --scalar-color-blue: #4eb3ec;
  --scalar-color-orange: #ff8d4d;
  --scalar-color-purple: #b191f9;

  --scalar-link-color: var(--scalar-color-1);
  --scalar-link-color-hover: var(--scalar-link-color);

  --scalar-button-1: rgba(255, 255, 255, 1);
  --scalar-button-1-hover: rgba(255, 255, 255, 0.9);
  --scalar-button-1-color: black;

  --scalar-tooltip-background: color-mix(in srgb, var(--scalar-background-1), #fff 10%);
  --scalar-tooltip-color: color-mix(in srgb, #fff, transparent 5%);

  --scalar-color-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-background-1) 20%);

  --scalar-background-alert: color-mix(in srgb, var(--scalar-color-orange), var(--scalar-background-1) 95%);
  --scalar-background-danger: color-mix(in srgb, var(--scalar-color-red), var(--scalar-background-1) 95%);
}
@supports (color: color(display-p3 1 1 1)) {
  .light-mode {
    --scalar-color-accent: color(display-p3 0 0.6 1 / 1);
    --scalar-color-green: color(display-p3 0.023529 0.564706 0.380392 / 1);
    --scalar-color-red: color(display-p3 0.937255 0 0.023529 / 1);
    --scalar-color-yellow: color(display-p3 0.929412 0.745098 0.12549 / 1);
    --scalar-color-blue: color(display-p3 0 0.509804 0.815686 / 1);
    --scalar-color-orange: color(display-p3 1 0.4 0.02);
    --scalar-color-purple: color(display-p3 0.321569 0.011765 0.819608 / 1);
  }
  .dark-mode {
    --scalar-color-accent: color(display-p3 0.07 0.67 1);
    --scalar-color-green: color(display-p3 0 0.713725 0.282353 / 1);
    --scalar-color-red: color(display-p3 0.862745 0.105882 0.098039 / 1);
    --scalar-color-yellow: color(display-p3 1 0.788235 0.05098 / 1);
    --scalar-color-blue: color(display-p3 0.305882 0.701961 0.92549 / 1);
    --scalar-color-orange: color(display-p3 1 0.552941 0.301961 / 1);
    --scalar-color-purple: color(display-p3 0.694118 0.568627 0.976471 / 1);
  }
}
`,n?Ir:``].join(``);return r?`@layer ${r} {\n${i}}`:i},ei=[`addTopNav`,`closeModal`,`closeTopNav`,`createNew`,`executeRequest`,`focusAddressBar`,`focusRequestSearch`,`jumpToLastTab`,`jumpToTab`,`navigateSearchResultsDown`,`navigateSearchResultsUp`,`navigateTopNavLeft`,`navigateTopNavRight`,`openCommandPalette`,`selectSearchResult`,`toggleSidebar`],ti="Space(Backspace(Tab(Enter(Escape(ArrowDown(ArrowLeft(ArrowRight(ArrowUp(End(Home(PageDown(PageUp(Delete(0(1(2(3(4(5(6(7(8(9(a(b(c(d(e(f(g(h(i(j(k(l(m(n(o(p(q(r(s(t(u(v(w(x(y(z(0(1(2(3(4(5(6(7(8(9(*(+(-(.(/(F1(F2(F3(F4(F5(F6(F7(F8(F9(F10(F11(F12(;(=(,(-(.(/(`([(\\(](".split(`(`),ni=_(r([g(`Meta`),g(`Control`),g(`Shift`),g(`Alt`),g(`default`)])).optional().default([`default`]),ri=t({modifiers:ni,hotKeys:a(m(ti),t({modifiers:ni.optional(),event:m(ei)})).optional()}).optional();t({uid:w.brand(),name:f().default(`Default Workspace`),description:f().default(`Basic Scalar Workspace`),collections:_(f().brand()).default([]),environments:p(f(),f()).default({}),hotKeyConfig:ri,activeEnvironmentId:f().optional().default(`default`),cookies:_(f().brand()).default([]),proxyUrl:f().optional(),themeId:m(Zr).optional().default(`default`).catch(`default`),selectedHttpClient:t({targetKey:f(),clientKey:f()}).optional().default({targetKey:`shell`,clientKey:`curl`})});var ii=`WORKSPACE_SYMBOL`,ai=()=>{let t=e(ii);if(!t)throw Error(`Workspace store not provided`);return t};export{x as $,ht as A,We as B,I as C,kt as D,M as E,Je as F,je as G,Ie as H,qe as I,Te as J,Ee as K,w as L,Ze as M,$e as N,Et as O,Xe as P,xe as Q,Ge as R,F as S,N as T,Pe as U,He as V,Ae as W,S as X,we as Y,Ce as Z,Gt as _,Zr as a,b as at,Ut as b,Pr as c,le as ct,W as d,se as dt,be as et,Ln as f,ae as ft,H as g,En as h,$ as i,pe as it,Qe as j,dt as k,Nr as l,ce as lt,Mn as m,v as mt,$r as n,he as nt,Qr as o,fe as ot,Nn as p,re as pt,De as q,Xr as r,me as rt,Fr as s,de as st,ai as t,ge as tt,kr as u,oe as ut,qt as v,P as w,Lt as x,Vt as y,Ue as z};