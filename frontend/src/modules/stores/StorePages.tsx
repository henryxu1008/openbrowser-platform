import { createContext, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { Link, Outlet, useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { Button, Card, ConfirmModal, Input, Modal, Select, Table } from '../../shared/components'
import { BrowserEditPage, type ManagedBrowserEditor } from '../browser/pages/BrowserEditPage'
import { BrowserDetailPage } from '../browser/pages/BrowserDetailPage'
import { storeAPI, storeError, storeOperationKey, type AuditItem, type AuditPage, type AuditQuery, type Catalog, type Environment, type Store, type StoreContext } from './api'

const Scope = createContext<StoreContext | null>(null)
const Dirty = createContext<(dirty:boolean)=>void>(()=>{})
const useScope = () => { const scope = useContext(Scope); if (!scope) throw new Error('store_context_required'); return scope }
const time = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '—'
const outcome = (value: string) => ({succeeded:'已确认', failed:'失败', unknown:'结果未知', result_unconfirmed:'结果未回传'}[value] || value)
const actions: Record<string,string> = { browser_operation_admitted:'操作获准', browser_configuration_reported:'环境配置报告', browser_open_reported:'环境打开报告', browser_close_reported:'环境关闭报告', store_access_granted:'店铺授权', store_access_revoked:'撤销授权' }
function ErrorNotice({message}: {message: string}) { return message ? <p role="alert" className="text-sm text-red-600 whitespace-pre-wrap">{message}</p> : null }

export function StoreWorkspace() {
  const navigate = useNavigate()
  const [scope,setScope] = useState<StoreContext | null>(null)
  const [error,setError] = useState(''), [busy,setBusy] = useState(true)
  const [dirty,setDirty]=useState(false),[pendingOrg,setPendingOrg]=useState('')
  const requestEpoch = useRef(0), requestPending = useRef(false)
  useEffect(() => {void load();return()=>{requestEpoch.current++}},[])
  async function load() {
    const epoch=++requestEpoch.current;requestPending.current=true
    setBusy(true);setScope(null);setError('')
    try {const value=await storeAPI().context();if(epoch===requestEpoch.current)setScope(value)} catch(e){if(epoch===requestEpoch.current)setError(storeError(e))} finally{if(epoch===requestEpoch.current){requestPending.current=false;setBusy(false)}}
  }
  async function select(id: string) {
    if(!scope||requestPending.current||id===scope.context.effectiveOrganizationId)return
    const epoch=++requestEpoch.current;requestPending.current=true
    const previous=scope;setBusy(true);setScope(null);setError('')
    setDirty(false);setPendingOrg('')
    // A store route is a selection within its enterprise. Leave it before a
    // replacement context can mount a detail/editor with the old store ID.
    navigate('/stores',{replace:true})
    try {const value=await storeAPI().select(id,previous.revision);if(epoch===requestEpoch.current)setScope(value)}catch(e){if(epoch===requestEpoch.current)setError(storeError(e))}finally{if(epoch===requestEpoch.current){requestPending.current=false;setBusy(false)}}
  }
  return <div className="space-y-4">
    <Card padding="none"><div className="p-4 flex items-center justify-between gap-4"><Link to="/stores" className="font-semibold">店铺工作台</Link>
      <Select aria-label="当前企业" disabled={busy||!scope} value={scope?.context.effectiveOrganizationId||''} onChange={e=>{if(e.target.value===scope?.context.effectiveOrganizationId)return;if(dirty)setPendingOrg(e.target.value);else void select(e.target.value)}} options={[{value:'',label:busy?'加载中…':'请选择企业'},...(scope?.context.organizations||[]).map(org=>({value:org.id,label:org.name}))]} />
    </div></Card>
    <ErrorNotice message={error}/>
    {error&&<Button variant="secondary" onClick={()=>void load()}>只读核对企业上下文</Button>}
    {scope?.context.selectionRequired&&<p role="status">请选择企业后查看获授权的店铺。</p>}
    {scope?.context.effectiveOrganizationId&&!busy&&<Scope.Provider value={scope}><Dirty.Provider value={setDirty}><Outlet key={scope.revision}/></Dirty.Provider></Scope.Provider>}
    <ConfirmModal open={!!pendingOrg} title="放弃未保存的更改并切换企业？" content="当前环境配置尚未保存。" onClose={()=>setPendingOrg('')} onConfirm={()=>{const org=pendingOrg;setPendingOrg('');setDirty(false);void select(org)}}/>
  </div>
}

export function StoreListPage() {
  const scope=useScope(),[params,setParams]=useSearchParams()
  const q=params.get('q')||'',platform=params.get('platform')||'',page=Math.max(1,Number(params.get('page'))||1)
  const [draft,setDraft]=useState(q),[platformDraft,setPlatformDraft]=useState(platform)
  const [catalog,setCatalog]=useState<Catalog|null>(null),[error,setError]=useState(''),[loading,setLoading]=useState(true)
  useEffect(()=>{let current=true;setLoading(true);setCatalog(null);setError('');void storeAPI().list(q,platform,page,scope.revision).then(value=>{if(current)setCatalog(value)}).catch(e=>{if(current)setError(storeError(e))}).finally(()=>{if(current)setLoading(false)});return()=>{current=false}},[q,platform,page,scope.revision])
  return <Card title="我的店铺">
    <form className="flex gap-3 mb-4" onSubmit={e=>{e.preventDefault();setParams({q:draft,platform:platformDraft,page:'1'})}}>
      <Input aria-label="搜索店铺" value={draft} onChange={e=>setDraft(e.target.value)} placeholder="搜索店铺名称"/>
      <Select aria-label="平台" value={platformDraft} onChange={e=>setPlatformDraft(e.target.value)} options={[{value:'',label:'全部平台'},...['amazon','temu','shein','tiktok','walmart'].map(value=>({value,label:value}))]}/>
      <Button type="submit">查询</Button>
    </form>
    <ErrorNotice message={error}/>
    {!error&&<Table<Store> data={catalog?.items||[]} loading={loading} rowKey="id" emptyText="当前企业没有可访问的店铺" columns={[
      {key:'name',title:'店铺',render:(_,store)=><Link to={`/stores/${encodeURIComponent(store.id)}?return=${encodeURIComponent(params.toString())}`}>{store.name}</Link>},
      {key:'platform',title:'平台'},{key:'region',title:'地区'},{key:'recordStatus',title:'店铺状态'},{key:'serviceStatus',title:'服务状态'},
      {key:'action',title:'操作',render:(_,store)=><Link to={`/stores/${store.id}?return=${encodeURIComponent(params.toString())}`}>进入工作台</Link>},
    ]}/>}
    {catalog&&<div className="flex gap-3 justify-end mt-4"><Button variant="secondary" disabled={page<=1||loading} onClick={()=>setParams({q,platform,page:String(page-1)})}>上一页</Button><span className="py-2">第 {page} 页</span><Button variant="secondary" disabled={page*20>=catalog.pagination.total||loading} onClick={()=>setParams({q,platform,page:String(page+1)})}>下一页</Button></div>}
  </Card>
}

export function StoreDetailPage(){const {storeId}=useParams();return <StoreDetail key={storeId}/>}
function StoreDetail() {
  const scope=useScope(),{storeId=''}=useParams(),navigate=useNavigate(),[params,setParams]=useSearchParams()
  const [store,setStore]=useState<Store|null>(null),[environment,setEnvironment]=useState<Environment|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false),[loaded,setLoaded]=useState(false)
  const [receipt,setReceipt]=useState('')
  const [unknownKey,setUnknownKey]=useState('')
  const actionBusy=useRef(false)
  const reading=useRef(false)
  const viewEpoch=useRef(0)
  useEffect(()=>{void read();return()=>{viewEpoch.current++}},[storeId,scope.revision])
  async function read() {
    if(actionBusy.current)return
    const epoch=++viewEpoch.current;reading.current=true
    setLoaded(false);setError('');setStore(null);setEnvironment(null);setReceipt('');setUnknownKey('')
    const api=storeAPI()
    try{const record=await api.store(storeId,scope.revision);if(epoch!==viewEpoch.current)return;const state=await api.environment(storeId,scope.revision);if(epoch===viewEpoch.current){setStore(record);setEnvironment(state)}}catch(e){if(epoch===viewEpoch.current)setError(storeError(e))}finally{if(epoch===viewEpoch.current){reading.current=false;setLoaded(true)}}
  }
  const org=scope.context.organizations.find(org=>org.id===scope.context.effectiveOrganizationId)
  const canUse=!!org?.permissions.includes('workbench.store.browser.use')&&store?.recordStatus==='active'
  async function action(kind:'open'|'close') {
    if(actionBusy.current||reading.current||!environment?.binding)return
    actionBusy.current=true
    const epoch=viewEpoch.current;setBusy(true);setError('');setReceipt('')
    try{
      const api=storeAPI()
      if(kind==='open'){const result=await api.open(storeId,environment.binding.bindingVersion,scope.revision);if(epoch===viewEpoch.current)setEnvironment(result)}
      else{const result=await api.close(storeId,scope.revision);if(epoch===viewEpoch.current)setEnvironment(prev=>prev?{...prev,running:false,runtimeKnown:result.outcome==='succeeded',observedAt:result.observedAt}:prev)}
    }catch(e){if(epoch===viewEpoch.current){setUnknownKey(storeOperationKey(e));setError(storeError(e));setEnvironment(prev=>prev?{...prev,runtimeKnown:false}:prev)}}finally{actionBusy.current=false;if(epoch===viewEpoch.current)setBusy(false)}
  }
  async function inspectReceipt(){const key=unknownKey||environment?.pending?.operationId||environment?.lastOperation?.operationId;if(!key)return;const epoch=viewEpoch.current;try{const value=await storeAPI().receipt(storeId,key,scope.revision);if(epoch===viewEpoch.current)setReceipt(JSON.stringify(value,null,2))}catch(e){if(epoch===viewEpoch.current)setError(storeError(e))}}
  const tab=params.get('tab')==='records'?'records':'environment'
  return <div className="space-y-4">
    <Link to={`/stores?${params.get('return')||''}`}>返回店铺列表</Link>
    <Card title={store?.name||'店铺详情'}><p className="text-sm">{store&&`${store.platform} · ${store.region} · ${store.recordStatus}`}</p><div className="flex gap-3 mt-3">{(['environment','records'] as const).map(value=><Button key={value} variant={tab===value?'primary':'secondary'} onClick={()=>{const next=new URLSearchParams(params);next.set('tab',value);setParams(next)}}>{value==='environment'?'店铺环境':'操作记录'}</Button>)}<Button variant="secondary" disabled={busy||!loaded} onClick={()=>void read()}>只读刷新</Button></div></Card>
    <ErrorNotice message={error}/>
    {!loaded&&<p role="status">加载中…</p>}
    {store&&environment&&tab==='environment'&&<>
      {!canUse&&<p role="status">当前角色未获店铺浏览器使用权限，或店铺已停用。已有窗口可通过本机归属证明安全关闭。</p>}
      <BrowserDetailPage managed={{name:environment.name,running:environment.running,known:!environment.binding||environment.runtimeKnown,core:environment.coreId,proxy:environment.proxySummary,observedAt:environment.observedAt,pending:busy,canUse:!!canUse&&!environment.pending,evidence:environment.pending?`操作 ${environment.pending.operationId} 结果待核对，已阻断新操作。`:environment.lastOperation?`原操作 ${environment.lastOperation.operationId} · ${outcome(environment.lastOperation.outcome)} · ${environment.lastOperation.reportState==='confirmed'?'报告已收到':'报告未确认'}`:undefined,onEdit:()=>navigate(`/stores/${storeId}/environment/edit?return=${encodeURIComponent(params.toString())}`),onOpen:()=>void action('open'),onClose:()=>void action('close')}}/>
      {(unknownKey||environment.lastOperation)&&<Button variant="secondary" onClick={()=>void inspectReceipt()}>只读核对原操作</Button>}
      {receipt&&<Modal open title="原操作回执" onClose={()=>setReceipt('')} width="760px"><pre className="whitespace-pre-wrap break-all text-xs">{receipt}</pre></Modal>}
    </>}
    {store&&tab==='records'&&<StoreRecords storeId={storeId}/>}
  </div>
}

export function StoreEnvironmentEditPage() {
  const {storeId=''}=useParams(),scope=useScope(),navigate=useNavigate(),[params]=useSearchParams()
  const setDirty=useContext(Dirty)
  const activeEditor=useRef<ManagedBrowserEditor|null>(null)
  const editor=useMemo<ManagedBrowserEditor>(()=>{
    const api=storeAPI();let bindingVersion: string | null=null
    const adapter:ManagedBrowserEditor={title:'店铺环境配置',onDirtyChange:setDirty,load:async()=>{const result=await api.options(storeId,scope.revision);bindingVersion=result.binding?.bindingVersion||'0';return result},save:async input=>{if(bindingVersion===null)throw new Error('store_configuration_unavailable');try{await api.save(storeId,bindingVersion,input,scope.revision)}catch(e){throw new Error(storeError(e))}},back:()=>{if(activeEditor.current===adapter)navigate(`/stores/${storeId}?${params.get('return')||''}`)}}
    return adapter
  },[storeId,scope.revision,navigate,params,setDirty])
  useEffect(()=>{activeEditor.current=editor;return()=>{if(activeEditor.current===editor)activeEditor.current=null}},[editor])
  return <BrowserEditPage key={`${storeId}:${scope.revision}`} managed={editor}/>
}

function StoreRecords({storeId}:{storeId:string}) {
  const scope=useScope(),[draft,setDraft]=useState<AuditQuery>({}),[query,setQuery]=useState<AuditQuery>({}),[page,setPage]=useState<AuditPage|null>(null),[error,setError]=useState(''),[loading,setLoading]=useState(true),[selected,setSelected]=useState<AuditItem|null>(null),[cursors,setCursors]=useState<string[]>([])
  useEffect(()=>{let current=true;setPage(null);setError('');setLoading(true);void storeAPI().audit(storeId,query,scope.revision).then(value=>{if(current)setPage(value)}).catch(e=>{if(current)setError(storeError(e))}).finally(()=>{if(current)setLoading(false)});return()=>{current=false}},[storeId,scope.revision,query])
  return <Card title="操作记录">
    <form className="flex flex-wrap gap-3 mb-4" onSubmit={e=>{e.preventDefault();setCursors([]);setQuery({...draft,cursor:undefined})}}>
      <Select aria-label="操作类型" value={draft.action||''} onChange={e=>setDraft({...draft,action:e.target.value})} options={[{value:'',label:'全部操作'},...Object.entries(actions).map(([value,label])=>({value,label}))]}/>
      <Select aria-label="结果" value={draft.outcome||''} onChange={e=>setDraft({...draft,outcome:e.target.value})} options={[{value:'',label:'全部结果'},...['succeeded','failed','unknown'].map(value=>({value,label:outcome(value)}))]}/>
      {page?.canFilterActor&&<Input aria-label="操作人身份" placeholder="操作人身份" value={draft.actor||''} onChange={e=>setDraft({...draft,actor:e.target.value})}/>}
      <label className="text-sm">开始时间<Input type="datetime-local" onChange={e=>setDraft({...draft,from:e.target.value?new Date(e.target.value).toISOString():undefined})}/></label>
      <label className="text-sm">结束时间<Input type="datetime-local" onChange={e=>setDraft({...draft,to:e.target.value?new Date(e.target.value).toISOString():undefined})}/></label>
      <Button type="submit">查询</Button>
    </form>
    <p className="mb-3 text-xs text-[var(--color-text-muted)]">记录店铺授权、应用准入和客户端结果报告；不覆盖商家页面内的操作。默认最近 30 天，最多查询 90 天。</p>
    <ErrorNotice message={error}/>
    {!error&&<Table<AuditItem> loading={loading} data={page?.items||[]} rowKey="eventId" emptyText="当前条件没有操作记录" columns={[
      {key:'serverTime',title:'服务器时间',render:value=>time(value)},{key:'actorSubject',title:'操作人'},{key:'action',title:'操作',render:value=>actions[value]||value},{key:'outcome',title:'结果',render:value=>outcome(value)},{key:'factSource',title:'来源',render:value=>value==='client_report'?'客户端声明':'服务器事实'},{key:'detail',title:'详情',render:(_,item)=><Button variant="secondary" size="sm" onClick={()=>setSelected(item)}>查看记录</Button>},
    ]}/>}
    {page&&<div className="flex justify-end gap-3 mt-4"><Button variant="secondary" disabled={loading||!cursors.length} onClick={()=>{const prev=cursors.slice(0,-1);setCursors(prev);setQuery({...query,cursor:prev[prev.length-1]||undefined})}}>上一页</Button><Button variant="secondary" disabled={loading||!page.nextCursor} onClick={()=>{if(page.nextCursor){setCursors([...cursors,page.nextCursor]);setQuery({...query,cursor:page.nextCursor})}}}>下一页</Button></div>}
    <Modal open={!!selected} title="操作记录详情" onClose={()=>setSelected(null)} width="700px">{selected&&<dl className="grid grid-cols-[150px_1fr] gap-3 text-sm break-all">
      {Object.entries({'操作编号':selected.operationId,'操作人':selected.actorSubject,'成员引用':selected.memberRef,'操作':actions[selected.action]||selected.action,'结果':outcome(selected.outcome),'原因':selected.reasonCode,'事实来源':selected.factSource,'服务器时间':time(selected.serverTime),'客户端声明时间':time(selected.clientObservedAt),'本机绑定引用':selected.bindingRef,'实例引用':selected.profileRef}).map(([label,value])=><div key={label} className="contents"><dt>{label}</dt><dd>{value||'—'}</dd></div>)}
    </dl>}</Modal>
  </Card>
}
