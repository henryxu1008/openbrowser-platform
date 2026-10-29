import {afterEach,beforeEach,expect,it,vi} from 'vitest'
import {act,create} from 'react-test-renderer'
import {MemoryRouter} from 'react-router-dom'
const old=vi.hoisted(()=>({fetchBrowserCores:vi.fn(),fetchBrowserProxies:vi.fn(),fetchAllTags:vi.fn(),fetchGroups:vi.fn(),fetchBrowserSettings:vi.fn(),fetchBrowserProfiles:vi.fn(),fetchBrowserProfileFingerprintMatrix:vi.fn(),fetchBrowserTabs:vi.fn(),validateProxyConfig:vi.fn(),createBrowserProfile:vi.fn(),updateBrowserProfile:vi.fn()}))
vi.mock('../src/modules/browser/api',()=>old)
vi.mock('../src/modules/browser/components/FingerprintPanel',()=>({FingerprintPanel:()=> <div>Fingerprint fields</div>}))
vi.mock('../src/modules/browser/components/ProxyPickerModal',()=>({ProxyPickerModal:()=>{throw new Error('managed mounted old proxy picker')}}))
vi.mock('../src/shared/components',()=>({Card:({children,title,actions})=><section>{title}{actions}{children}</section>,Button:({children,loading,...p})=><button {...p}>{children}</button>,Badge:({children})=><span>{children}</span>,Input:p=><input {...p}/>,Select:p=><select {...p}/>,Textarea:p=><textarea {...p}/>,FormItem:({children,label})=><label>{label}{children}</label>,Modal:()=>null,ConfirmModal:()=>null,Table:()=>null,toast:{}}))
let tree
beforeEach(()=>{for(const fn of Object.values(old))fn.mockClear()})
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null})
it('managed editor uses its narrow load/save adapter without old profile/settings/proxy or live fingerprint effects',async()=>{
 const {BrowserEditPage}=await import('../src/modules/browser/pages/BrowserEditPage')
 const managed={title:'Shop environment',load:vi.fn().mockResolvedValue({input:{profileName:'Shop',userDataDir:'',coreId:'core',restoreLastSession:'enabled',fingerprintArgs:[],proxyId:'',proxyConfig:'',memoryLimitMb:0,launchArgs:[],tags:[],keywords:[],groupId:''},cores:[]}),save:vi.fn().mockResolvedValue(undefined),back:vi.fn()}
 await act(async()=>{tree=create(<MemoryRouter><BrowserEditPage managed={managed}/></MemoryRouter>);await Promise.resolve()})
 for(const fn of Object.values(old))expect(fn).not.toHaveBeenCalled()
 expect(managed.load).toHaveBeenCalledTimes(1)
 const text=JSON.stringify(tree.toJSON());expect(text).not.toContain('高级启动参数');expect(text).not.toContain('用户数据目录');expect(text).not.toContain('自测当前实例')
 const save=tree.root.findAllByType('button').find(n=>n.children.join('')==='保存配置')
 await act(async()=>{save.props.onClick();save.props.onClick();await Promise.resolve()})
 expect(managed.save).toHaveBeenCalledTimes(1);expect(managed.back).toHaveBeenCalledTimes(1)
 for(const fn of Object.values(old))expect(fn).not.toHaveBeenCalled()
})
it('managed detail does not mount standalone tabs/CDP/global profile loaders',async()=>{
 const {BrowserDetailPage}=await import('../src/modules/browser/pages/BrowserDetailPage')
 await act(async()=>{tree=create(<MemoryRouter><BrowserDetailPage managed={{name:'Shop',running:false,known:false,core:'core',proxy:'direct',observedAt:'',pending:false,canUse:false,onEdit:vi.fn(),onOpen:vi.fn(),onClose:vi.fn()}}/></MemoryRouter>)})
 for(const fn of Object.values(old))expect(fn).not.toHaveBeenCalled()
 expect(JSON.stringify(tree.toJSON())).toContain('结果待核对')
 expect(tree.root.findAllByType('button').every(n=>n.props.disabled)).toBe(true)
})
