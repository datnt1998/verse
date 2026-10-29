// node render.js in.svg out.png width [height] [bg]
// Vẽ SVG ra PNG bằng Chromium (giữ filter, gradient). bg mặc định trong suốt.
const { chromium } = require('playwright');
const fs = require('fs');
(async () => {
  const jobs = JSON.parse(process.argv[2]);
  const b = await chromium.launch();
  const p = await b.newPage({ deviceScaleFactor: 1 });
  for (const j of jobs) {
    const svg = fs.readFileSync(j.in, 'utf8');
    const w = j.w, h = j.h || j.w;
    await p.setViewportSize({ width: w, height: h });
    const bg = j.bg || 'transparent';
    await p.setContent(`<html><body style="margin:0;background:${bg}">` +
      `<img src="data:image/svg+xml;base64,${Buffer.from(svg).toString('base64')}" style="display:block;width:${w}px;height:${h}px"></body></html>`);
    await p.waitForTimeout(60);
    await p.screenshot({ path: j.out, omitBackground: bg === 'transparent', clip: { x: 0, y: 0, width: w, height: h } });
  }
  await b.close();
})();
