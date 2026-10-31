// Run with Bun 1.3.11 from a clean archive of yt-factory 2c201e6 with its dependencies.
// Storage is in-memory; layout commands and waveform generation are the original sources.
import { plugin } from "bun";
import fs from "node:fs/promises";
import path from "node:path";
import { createHash } from "node:crypto";
const root = process.cwd();
const output = process.argv[2];
if (!output) throw new Error("Pass output directory");
const records = [];
let project;
let serial = 0;
const glue = {
  async createProject({name}) { project = {id: "standard-project", name, description: "", createdAt: 1793448000000, updatedAt: 1793448000000, duration: 0, schemaVersion: 15, metadata: {width: 1920, height: 1080, fps: 30}}; return {project, revision: "initial"}; },
  async deleteProject() {},
  async findMediaByContentHash(hash) { return records.find(r => r.contentHash === hash); },
  async importMedia({fileName, declaredMime}) { const bytes = await fs.readFile(path.join(root, "assets/standard-template", fileName)); const record = {id: `standard-${fileName.replace(/\.[^.]+$/, "")}`, fileName, mime: declaredMime, contentHash: `sha256:${createHash("sha256").update(bytes).digest("hex")}`, size: bytes.length}; records.push(record); return record; },
  async linkMedia() {},
  async loadProject() { return {project, revision: "generated"}; },
  async seedBuiltinOverlays() {},
  async sha256File(file) { return createHash("sha256").update(await fs.readFile(file)).digest("hex"); },
  async resolveFfmpeg() { return {ffmpeg: process.env.FFMPEG_PATH ?? "/opt/homebrew/bin/ffmpeg"}; },
  templateShape(p) { return {metadata: p.metadata, tracks: p.timeline?.tracks ?? [], items: p.timeline?.items ?? [], transitions: p.timeline?.transitions ?? []}; },
  getOwnerChannel() { throw new Error("Not used by project builder"); },
  addChannelTemplate() { throw new Error("Not used by project builder"); },
  async applyCommands(_id, commands, options) {
    if (options.dryRun) return;
    // Runtime-selected source root; plugins must replace server storage before loading.
    const {createNodeSession} = await import(path.join(root, "src/editor/engine/commands/index.ts"));
    const initial = {items: [], tracks: [], transitions: [], keyframes: [], markers: [], compositions: [], topLevelSequenceIds: [], inPoint: null, outPoint: null, fps: 30, scrollPosition: 0, snapEnabled: true, currentFrame: 0, masterBusDb: 0, projectName: project.name, projectId: project.id, projectMetadata: project.metadata, mediaItems: records.map(r => ({id: r.id, contentHash: r.contentHash, mimeType: r.mime, storageType: "server", fileName: r.fileName})), stashStack: [], mainHolder: null};
    const session = createNodeSession(initial, {mint: () => `editor-generated-${++serial}`});
    try {
      const result = session.executor.executeCommands(commands);
      if (!result.ok) throw new Error(JSON.stringify(result.issues));
      const state = session.getState();
      project = {...project, duration: Math.max(0, ...state.items.map(i => i.from + i.durationInFrames)) / 30, mediaItems: state.mediaItems.map(m => ({id: m.id})), timeline: {items: state.items, tracks: state.tracks, transitions: state.transitions, keyframes: state.keyframes, markers: state.markers, compositions: state.compositions, topLevelSequenceIds: state.topLevelSequenceIds, currentFrame: state.currentFrame, scrollPosition: state.scrollPosition, masterBusDb: state.masterBusDb}};
    } finally { session.dispose(); }
    await fs.writeFile(path.join(output, "standard-commands.json"), JSON.stringify(commands, null, 2) + "\n");
  },
};
globalThis.__standardGlue = glue;
const mockModule = `const g = globalThis.__standardGlue; export const {createProject,deleteProject,findMediaByContentHash,importMedia,linkMedia,loadProject,applyCommands,seedBuiltinOverlays,resolveFfmpeg,sha256File,templateShape,getOwnerChannel,addChannelTemplate} = g;`;
plugin({name: "standard-storage", setup(build) {
  build.onLoad({filter: /\/src\/server\/(?:editor\/(?:projects|apply-commands|overlay-library)|clone\/(?:template-snapshot|artifacts)|channels\/owner-channels|tools\/ffmpeg)\.ts$/}, () => ({loader: "js", contents: mockModule}));
}});
await fs.mkdir(output, {recursive: true});
// This module lives in the runtime-selected clean canonical archive, not the working tree.
const {buildStandardTemplateProject} = await import(path.join(root, "src/server/channels/standard-template.ts"));
await buildStandardTemplateProject("Mẫu chuẩn");
await fs.writeFile(path.join(output, "standard-project.json"), JSON.stringify(project, null, 2) + "\n");
await fs.writeFile(path.join(output, "standard-media.json"), JSON.stringify(records, null, 2) + "\n");
