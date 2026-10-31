// Run with Bun 1.3.11 from a clean yt-factory 2c201e6 archive with its dependencies:
// bun /path/to/generate-fill.mjs /path/to/testdata /separate/output/directory
// Inputs come from the fixtures; expected values are computed only by the
// canonical TypeScript fill and real node command engine, never by Go.
import fs from "node:fs/promises";
import path from "node:path";

const [fixturesDirectory, outputDirectory] = process.argv.slice(2);
if (!fixturesDirectory || !outputDirectory) throw new Error("Pass fixture and output directories");
const root = process.cwd();
// The canonical archive is runtime-selected; imports must resolve its aliases.
const {buildTemplateFill, prepareCaptionData} = await import(path.join(root, "src/editor/engine/features/editing-templates/template-fill.ts"));
const {createNodeSession} = await import(path.join(root, "src/editor/engine/commands/index.ts"));
const golden = JSON.parse(await fs.readFile(path.join(fixturesDirectory, "fill-golden.json"), "utf8"));
if (golden.source !== "yt-factory@2c201e6" || Bun.version !== golden.runtime) throw new Error("Wrong fixture provenance or Bun runtime");

for (const fixture of golden.fixtures) {
  const input = structuredClone(fixture.input);
  const project = input.project;
  const timeline = project.timeline ?? {};
  const background = (timeline.tracks ?? []).find(track => track.templateSlot === "background-video");
  for (const item of input.backdrops ?? []) {
    if (!background) throw new Error("Backdrop fixture is missing its slot");
    (timeline.items ??= []).push({...item, trackId: background.id});
  }
  const template = {metadata: project.metadata, tracks: timeline.tracks ?? [], items: timeline.items ?? [], transitions: timeline.transitions ?? []};
  const filled = buildTemplateFill({...input, template});
  const initial = {
    items: timeline.items ?? [], tracks: timeline.tracks ?? [], transitions: timeline.transitions ?? [],
    keyframes: timeline.keyframes ?? [], markers: timeline.markers ?? [], compositions: timeline.compositions ?? [],
    topLevelSequenceIds: timeline.topLevelSequenceIds ?? [], inPoint: timeline.inPoint ?? null, outPoint: timeline.outPoint ?? null,
    fps: project.metadata.fps, scrollPosition: timeline.scrollPosition ?? 0, snapEnabled: true,
    currentFrame: timeline.currentFrame ?? 0, busAudioEq: timeline.busAudioEq, masterBusDb: timeline.masterBusDb ?? 0,
    projectName: project.name, projectId: project.id, projectMetadata: project.metadata,
    mediaItems: project.mediaItems ?? [], stashStack: [], mainHolder: null,
  };
  let serial = 0;
  const used = new Set();
  const collect = value => {
    if (Array.isArray(value)) { for (const child of value) collect(child); }
    else if (value !== null && typeof value === "object") {
      if (typeof value.id === "string") used.add(value.id);
      for (const child of Object.values(value)) collect(child);
    }
  };
  collect(project);
  const session = createNodeSession(initial, {mint() {
    let id;
    do { id = `editor-generated-${++serial}`; } while (used.has(id));
    used.add(id);
    return id;
  }});
  try {
    const executed = session.executor.executeCommands(filled.commands);
    if (!executed.ok) throw new Error(`${fixture.name}: ${JSON.stringify(executed.issues)}`);
    const state = session.getState();
    const metadata = state.projectMetadata ?? project.metadata;
    // Same serialization as server/editor/apply-commands.ts projectFromState.
    const result = {
      ...project, name: state.projectName ?? project.name, metadata,
      duration: Math.max(0, ...state.items.map(item => item.from + item.durationInFrames)) / (metadata.fps || 30),
      mediaItems: (state.mediaItems ?? []).map(media => ({id: media.id})),
      timeline: {...timeline, items: state.items, tracks: state.tracks, transitions: state.transitions,
        keyframes: state.keyframes, markers: state.markers, compositions: state.compositions,
        topLevelSequenceIds: state.topLevelSequenceIds, inPoint: state.inPoint ?? undefined, outPoint: state.outPoint ?? undefined,
        currentFrame: state.currentFrame, scrollPosition: state.scrollPosition, busAudioEq: state.busAudioEq, masterBusDb: state.masterBusDb},
    };
    fixture.expected = {...filled, project: result};
  } finally {
    session.dispose();
  }
}
const captions = JSON.parse(await fs.readFile(path.join(fixturesDirectory, "fill-captions-golden.json"), "utf8"));
for (const fixture of captions) fixture.expected = prepareCaptionData(fixture.cues, 30, 180);
await fs.mkdir(outputDirectory, {recursive: true});
await fs.writeFile(path.join(outputDirectory, "fill-golden.json"), JSON.stringify(golden, null, 2) + "\n");
await fs.writeFile(path.join(outputDirectory, "fill-captions-golden.json"), JSON.stringify(captions, null, 2) + "\n");
