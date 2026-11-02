import {expect,it} from 'vitest'
import {readFileSync} from 'node:fs'
import postcss from 'postcss'
const css=postcss.parse(readFileSync(new URL('../src/modules/stores/store-workbench.css',import.meta.url),'utf8'))
const rules=[];css.walkRules(rule=>rules.push(rule))
const declarations=selector=>Object.fromEntries((rules.find(rule=>rule.selector===selector)?.nodes||[]).filter(n=>n.type==='decl').map(n=>[n.prop,n.value]))
const light="[data-theme='light'] .store-workbench"
const luminance=hex=>{const rgb=hex.slice(1).match(/../g).map(s=>parseInt(s,16)/255).map(x=>x<=.04045?x/12.92:((x+.055)/1.055)**2.4);return rgb[0]*.2126+rgb[1]*.7152+rgb[2]*.0722}
const contrast=(a,b)=>{const x=luminance(a),y=luminance(b);return(Math.max(x,y)+.05)/(Math.min(x,y)+.05)}
it('limits palette overrides to the production light Store scope',()=>{
 const palette=declarations(light);expect(palette['--color-text-muted']).toBe('#56657a');expect(palette['--color-accent']).toBe('#1259e4')
 for(const rule of rules)for(const node of rule.nodes)if(node.type==='decl'&&node.prop.startsWith('--color-'))expect(rule.selector).toBe(light)
 expect(css.toString()).not.toContain(':root:not(')
})
it('declares native schemes without replacing the production theme owner',()=>{
 expect(declarations(light)['color-scheme']).toBe('light');expect(declarations("[data-theme='dark'] .store-workbench")['color-scheme']).toBe('dark')
 for(const theme of ['cream','mint','ocean'])expect(css.toString()).toContain(`[data-theme='${theme}'] .store-workbench`)
})
it('keeps light secondary, status and disabled labels readable in declared color pairs',()=>{
 const p=declarations(light);for(const background of ['#ffffff','#f3f6fb','#edf2f8'])expect(contrast(p['--color-text-muted'],background)).toBeGreaterThanOrEqual(4.5)
 expect(contrast(p['--color-accent'],'#ffffff')).toBeGreaterThanOrEqual(4.5)
 const disabled=declarations(light+' button:disabled');expect(disabled.opacity).toBe('1');expect(contrast(disabled.color,disabled.background)).toBeGreaterThanOrEqual(4.5)
})
it('keeps visible light input borders and placeholder text without changing other pages',()=>{
 expect(declarations(light+' :is(input, select, textarea)')['border-color']).toBe('#8795a8')
 expect(declarations(light+' :is(input, select, textarea)[class*="border-[var(--color-error)]"]')['border-color']).toBe('var(--color-error)')
 const placeholder=declarations(light+' :is(input, textarea)::placeholder');expect(placeholder.opacity).toBe('1');expect(placeholder.color).toBe('var(--color-text-muted)')
})
