import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import ts from 'typescript'
const allowedWrites: Record<string,string> = {
  'features/timeline/stores/composition-navigation-store.ts':'Context binding swaps live canonical domain and flushes composition records; navigation commands use this binding.',
  'features/timeline/stores/timeline-persistence.ts':'Project load/hydration boundary; replaces the entire project and resets history.',
  'features/timeline/stores/actions/composition-actions.ts':'Freecut compound mutation procedures invoked inside typed command apply or executeAnonymous.',
  'features/timeline/stores/actions/shared.ts':'Transition cascade helper invoked during executor-owned mutation.',
  'features/timeline/stores/actions/settings-actions.ts':'Legacy settings mutations enter through shared.execute, which routes into the command executor.',
  'features/timeline/stores/actions/split-bookkeeping.ts':'Command-owned split cascade helper updates transition references.',
  'features/timeline/stores/actions/legacy-av-mutations.ts':'Pure mutation procedure reused by item migration command.',
  'features/timeline/stores/actions/source-edit-mutations.ts':'Command-owned insert/overwrite compound mutations.',
  'features/timeline/stores/actions/edit/freeze-frame-mutations.ts':'Command-owned split/freeze compound mutation.',
  'features/timeline/stores/actions/edit/join-mutations.ts':'Command-owned join cascade helper.',
  'features/export/workers/audio-measurement.worker.ts':'Worker-local hydration boundary: the measurement worker owns a separate module instance and loads a saved project snapshot read-only.',
}
function files(directory:string):string[] {
  return readdirSync(directory,{withFileTypes:true}).flatMap(entry=>entry.isDirectory()?files(join(directory,entry.name)):[join(directory,entry.name)])
}
describe('canonical mutation ownership',()=>{
  it('permits canonical setters only in commands, bindings or documented Freecut command procedures',()=>{
    const root=join(process.cwd(),'src/editor/engine')
    const violations:string[]=[]
    const setters=new Set(['setItems','setTracks','setTransitions','setKeyframes','setMarkers','setCompositions','setTopLevelSequenceIds','setBusAudioEq','setMasterBusDb'])
    for(const file of files(root)) {
      const path=relative(root,file).replaceAll('\\','/')
      if(!/\.tsx?$/.test(path)||/\.test\.|test-helpers|test-utils|^test\//.test(path)||path.startsWith('commands/')||allowedWrites[path]) continue
      const source=ts.createSourceFile(file,readFileSync(file,'utf8'),ts.ScriptTarget.Latest,true,path.endsWith('.tsx')?ts.ScriptKind.TSX:ts.ScriptKind.TS)
      const visit=(node:ts.Node):void=>{
        if(ts.isCallExpression(node)&&ts.isPropertyAccessExpression(node.expression)&&setters.has(node.expression.name.text)) {
          const line=source.getLineAndCharacterOfPosition(node.getStart(source)).line+1
          violations.push(`${path}:${line} ${node.expression.name.text}`)
        }
        ts.forEachChild(node,visit)
      }
      visit(source)
    }
    expect(violations).toEqual([])
    expect(Object.values(allowedWrites).every(reason=>reason.length>30)).toBe(true)
  })
})
