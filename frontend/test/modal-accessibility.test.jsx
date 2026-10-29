import {afterEach,expect,it,vi} from 'vitest'
import {act,create} from 'react-test-renderer'
vi.mock('react-dom',()=>({createPortal:children=>children}))
import {Modal} from '../src/shared/components/Modal'
let tree
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null;delete globalThis.document;delete globalThis.window})
it('names the dialog and close button, traps keyboard focus, handles Escape and restores the trigger',()=>{
 const trigger={focus:vi.fn(),isConnected:true},first={focus:vi.fn()},last={focus:vi.fn()},listeners={}
 const surface={focus:vi.fn(),querySelectorAll:()=>[first,last],addEventListener:(name,fn)=>{listeners[name]=fn},removeEventListener:vi.fn()}
 globalThis.document={body:{style:{}},activeElement:trigger};globalThis.window={setTimeout,clearTimeout}
 const close=vi.fn()
 act(()=>{tree=create(<Modal open title="操作记录详情" onClose={close}><p>Record</p></Modal>,{createNodeMock:el=>el.props.role==='dialog'?surface:null})})
 const dialog=tree.root.findByProps({role:'dialog'});expect(dialog.props['aria-labelledby']).toBeTruthy();expect(tree.root.findByProps({'aria-label':'关闭对话框'})).toBeTruthy();expect(first.focus).toHaveBeenCalledTimes(1)
 document.activeElement=last;const prevent=vi.fn();listeners.keydown({key:'Tab',preventDefault:prevent});expect(prevent).toHaveBeenCalled();expect(first.focus).toHaveBeenCalledTimes(2)
 listeners.keydown({key:'Escape',preventDefault:vi.fn(),stopPropagation:vi.fn()});expect(close).toHaveBeenCalledTimes(1)
 act(()=>{tree.update(<Modal open={false} title="操作记录详情" onClose={close}><p>Record</p></Modal>)});expect(trigger.focus).toHaveBeenCalledTimes(1)
})
