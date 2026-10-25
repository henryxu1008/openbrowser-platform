import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { act, create } from 'react-test-renderer';

// Keep the actual generated SDK: only the Wails host boundary is synthetic.
vi.mock('../src/shared/components',()=>({Button:({children,loading,...props})=><button {...props} disabled={props.disabled||loading}>{children}</button>,Loading:({text})=><span>{text}</span>}));
let Gate, tree, native, current;
beforeEach(async()=>{
 vi.resetModules();vi.useFakeTimers();
 current={generation:7,state:'signed_out',configured:true};
 native={AuthStatus:vi.fn(async()=>current),AuthLogin:vi.fn(),QuitAppOnly:vi.fn()};
 globalThis.window={setInterval,clearInterval,go:{main:{App:native}}};
 Gate=(await import('../src/auth/AuthGate')).AuthGate;
});
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null;delete globalThis.window;vi.useRealTimers();});
const button=label=>tree.root.findAllByType('button').find(node=>node.children.join('')===label);

it.each([
 ['identity_discovery_request_failed','无法读取登录服务配置'],
 ['identity_discovery_document_invalid','登录服务配置格式无效'],
 ['identity_discovery_metadata_invalid','登录服务配置不兼容'],
 ['identity_login_failed','登录未完成'],
 ['private-status-detail','登录未完成'],
])('keeps %s distinct from expiry through one real SDK call',async(code,message)=>{
 native.AuthLogin.mockImplementation(async generation=>{
  expect(generation).toBe(7);
  current={generation:8,state:'signed_out',configured:true,errorCode:code};
  throw new Error('private-transport-detail');
 });
 await act(async()=>{tree=create(<Gate><div>Original business</div></Gate>);await Promise.resolve();});
 await act(async()=>{button('通过浏览器登录').props.onClick();await Promise.resolve();});
 await act(async()=>{vi.advanceTimersByTime(5000);await Promise.resolve();});
 const text=JSON.stringify(tree.toJSON());
 expect(native.AuthLogin).toHaveBeenCalledExactlyOnceWith(7);
 expect(text).toContain(message);
 expect(text).not.toContain('登录状态已失效');
 expect(text).not.toContain('private-transport-detail');
 expect(text).not.toContain('private-status-detail');
 expect(text).not.toContain('Original business');
 act(()=>button('仅退出应用').props.onClick());
 expect(native.QuitAppOnly).toHaveBeenCalledTimes(1);
});

it('does not let a late pre-login poll unlock business after failure',async()=>{
 let releaseOldPoll;
 native.AuthStatus.mockImplementationOnce(async()=>current)
  .mockImplementationOnce(()=>new Promise(resolve=>{releaseOldPoll=resolve;}));
 native.AuthLogin.mockImplementation(async()=>{
  current={generation:8,state:'signed_out',configured:true,errorCode:'identity_discovery_request_failed'};
  throw new Error('private-transport-detail');
 });
 await act(async()=>{tree=create(<Gate><div>Original business</div></Gate>);await Promise.resolve();});
 await act(async()=>{vi.advanceTimersByTime(1000);await Promise.resolve();});
 expect(releaseOldPoll).toBeTypeOf('function');
 await act(async()=>{button('通过浏览器登录').props.onClick();await Promise.resolve();});
 await act(async()=>{
  releaseOldPoll({generation:7,state:'signed_in',configured:true,expiresAt:new Date(Date.now()+60000).toISOString()});
  await Promise.resolve();
  vi.advanceTimersByTime(1000);await Promise.resolve();
 });
 expect(native.AuthLogin).toHaveBeenCalledExactlyOnceWith(7);
 expect(JSON.stringify(tree.toJSON())).toContain('无法读取登录服务配置');
 expect(JSON.stringify(tree.toJSON())).not.toContain('Original business');
});

it('keeps actual expiry distinct without starting a login',async()=>{
 let lateStatus;
 native.AuthStatus.mockImplementationOnce(()=>new Promise(resolve=>{lateStatus=resolve;}));
 await act(async()=>{tree=create(<Gate><div>Original business</div></Gate>);await Promise.resolve();});
 await act(async()=>{lateStatus({generation:7,state:'signed_out',configured:true,errorCode:'identity_login_expired'});await Promise.resolve();});
 expect(JSON.stringify(tree.toJSON())).toContain('登录状态已失效');
 expect(native.AuthLogin).not.toHaveBeenCalled();
});
