import assert from 'node:assert/strict'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import test from 'node:test'
import { open } from './browser.ts'

test('coordinate clicks follow layout changed by scrolling', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'prototype-click-'))
  const file = join(dir, 'scroll.html')
  // A scroll handler moves the target after scrollIntoView has returned.
  // Immediate coordinates can hit the old position or lose the mouse-up.
  writeFileSync(file, `<!doctype html><meta charset="utf-8">
    <style>body{margin:0} #box{width:600px;height:150px;overflow:auto}
    #space{height:1200px;padding-top:600px} button{width:100px;height:50px}</style>
    <div id="box"><div id="space"><button data-click="target">目标</button></div></div>
    <script>
    let hits=0;
    const box=document.querySelector('#box'),button=document.querySelector('button');
    box.addEventListener('scroll',()=>{button.style.marginLeft=box.scrollTop?'300px':'0px'});
    button.addEventListener('click',()=>hits++);
    </script>`)
  try {
    const page = await open(file, '#box')
    try {
      assert.deepEqual(await page.js('[innerWidth, innerHeight, devicePixelRatio]'), [1400, 1000, 1])
      for (let i = 0; i < 12; i++) {
        await page.js(`box.scrollTop=0;button.style.marginLeft='0px';hits=0`)
        await page.click('目标')
        assert.equal(await page.js('hits'), 1, `click ${i + 1} must reach the moved button exactly once`)
      }
    } finally { await page.close() }
  } finally { rmSync(dir, { recursive: true, force: true }) }
})
