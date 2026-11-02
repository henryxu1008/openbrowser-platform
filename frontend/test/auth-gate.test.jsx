import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { act, create } from 'react-test-renderer';
import { useEffect } from 'react';

const native=vi.hoisted(()=>({AuthStatus:vi.fn(),AuthLogin:vi.fn(),AuthLogout:vi.fn(),AuthCancel:vi.fn(),AuthGrantAPI:vi.fn(),QuitAppOnly:vi.fn()}));
vi.mock('../src/wailsjs/go/main/App',()=>native);
vi.mock('../src/shared/components',()=>({Button:({children,loading,...props})=><button {...props} disabled={props.disabled||loading}>{children}</button>,Loading:({text})=><span>{text}</span>}));
let Gate,tree,mounted,unmounted;
const status=(state,generation=7)=>({state,generation,configured:true,subject:'synthetic',expiresAt:new Date(Date.now()+60000).toISOString()});
function Business(){useEffect(()=>{mounted();return unmounted;},[]);return <div>Original business</div>;}
beforeEach(async()=>{vi.resetModules();vi.useFakeTimers();for(const fn of Object.values(native))fn.mockReset();globalThis.window={setInterval,clearInterval};Gate=(await import('../src/auth/AuthGate')).AuthGate;mounted=vi.fn();unmounted=vi.fn();});
afterEach(()=>{if(tree)act(()=>tree.unmount());tree=null;delete globalThis.window;vi.useRealTimers();});
async function mount(){await act(async()=>{tree=create(<Gate><Business /></Gate>);await Promise.resolve();});}
const button=label=>tree.root.findAllByType('button').find(node=>node.children.join('')===label);

it('keeps original business effects unmounted when configuration is missing',async()=>{
 native.AuthStatus.mockResolvedValue({generation:0,state:'unconfigured',configured:false});await mount();
 expect(mounted).not.toHaveBeenCalled();expect(button('通过浏览器登录').props.disabled).toBe(true);
 expect(JSON.stringify(tree.toJSON())).toContain('联系应用提供方');expect(JSON.stringify(tree.toJSON())).not.toContain('提供公开客户端配置');
 act(()=>button('仅退出应用').props.onClick());expect(native.QuitAppOnly).toHaveBeenCalledTimes(1);
});
it('opens one login attempt under duplicate input and shows the existing cancellation path',async()=>{
 native.AuthStatus.mockResolvedValue(status('signed_out'));let release;native.AuthLogin.mockImplementation(()=>new Promise(resolve=>{release=resolve;}));await mount();
 act(()=>{button('通过浏览器登录').props.onClick();button('通过浏览器登录').props.onClick();});
 expect(native.AuthLogin).toHaveBeenCalledExactlyOnceWith(7);expect(mounted).not.toHaveBeenCalled();
 await act(async()=>{release(status('signing_in',8));await Promise.resolve();});
 expect(button('取消登录')).toBeDefined();
});
it('unmounts original business immediately on logout and does not replay it',async()=>{
 native.AuthStatus.mockResolvedValue(status('signed_in'));let release;native.AuthLogout.mockImplementation(()=>new Promise(resolve=>{release=resolve;}));await mount();expect(mounted).toHaveBeenCalledTimes(1);
 act(()=>{button('退出登录').props.onClick();});expect(unmounted).toHaveBeenCalledTimes(1);expect(native.AuthLogout).toHaveBeenCalledExactlyOnceWith(7);
 await act(async()=>{release(status('signed_out',8));await Promise.resolve();});expect(mounted).toHaveBeenCalledTimes(1);
});
it('halts status polling on an unknown result and retains app-only exit',async()=>{
 native.AuthStatus.mockRejectedValue(new Error('synthetic transport failure'));await mount();
 await act(async()=>{vi.advanceTimersByTime(5000);await Promise.resolve();});expect(native.AuthStatus).toHaveBeenCalledTimes(1);expect(mounted).not.toHaveBeenCalled();expect(button('仅退出应用')).toBeDefined();
});
it.each([false,true])('shows the configuration failure once when configured=%s without admitting business',async configured=>{
 native.AuthStatus.mockResolvedValue({generation:0,state:'unconfigured',configured,errorCode:'identity_configuration_required'});await mount();
 const notices=tree.root.findAllByProps({role:'status'}).filter(node=>node.children.join('')==='安装包的登录配置不可用，请联系应用提供方。');
 expect(notices).toHaveLength(1);expect(mounted).not.toHaveBeenCalled();expect(native.AuthLogin).not.toHaveBeenCalled();
 if(!configured)expect(button('通过浏览器登录').props.disabled).toBe(true);
 expect(button('仅退出应用')).toBeDefined();
});
it('preserves a distinct native error alongside missing configuration guidance',async()=>{
 native.AuthStatus.mockResolvedValue({generation:0,state:'unconfigured',configured:false,errorCode:'identity_discovery_request_failed'});await mount();
 const output=JSON.stringify(tree.toJSON());expect(output).toContain('安装包的登录配置不可用');expect(output).toContain('无法读取登录服务配置');expect(mounted).not.toHaveBeenCalled();
});
