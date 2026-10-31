/*
MIT License

Copyright (c) 2024-2026 Augustus Otu and Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

*/
"use strict";
(() => {
  // src/editor/engine/donor/openreel/audio/fft.ts
  var FFT = class {
    constructor(size) {
      this.size = size;
      if (!Number.isInteger(size) || size < 2 || (size & size - 1) !== 0) {
        throw new Error("FFT size must be a power of two");
      }
      this.cosine = new Float64Array(size / 2);
      this.sine = new Float64Array(size / 2);
      this.reverse = new Uint32Array(size);
      for (let i = 0; i < size / 2; i++) {
        this.cosine[i] = Math.cos(-2 * Math.PI * i / size);
        this.sine[i] = Math.sin(-2 * Math.PI * i / size);
      }
      const bits = Math.log2(size);
      for (let i = 0; i < size; i++) {
        let reversed = 0;
        for (let bit = 0; bit < bits; bit++) reversed = reversed << 1 | i >> bit & 1;
        this.reverse[i] = reversed;
      }
    }
    size;
    cosine;
    sine;
    reverse;
    transform(real, imag, inverse = false) {
      const n = this.size;
      for (let i = 0; i < n; i++) {
        const j = this.reverse[i];
        if (j <= i) continue;
        const r = real[i];
        const im = imag[i];
        real[i] = real[j];
        imag[i] = imag[j];
        real[j] = r;
        imag[j] = im;
      }
      for (let length = 2; length <= n; length *= 2) {
        const half = length / 2;
        const step = n / length;
        for (let i = 0; i < n; i += length) {
          for (let j = 0; j < half; j++) {
            const cos = this.cosine[j * step];
            const sin = this.sine[j * step] * (inverse ? -1 : 1);
            const even = i + j;
            const odd = even + half;
            const r = real[odd] * cos - imag[odd] * sin;
            const im = real[odd] * sin + imag[odd] * cos;
            real[odd] = real[even] - r;
            imag[odd] = imag[even] - im;
            real[even] = real[even] + r;
            imag[even] = imag[even] + im;
          }
        }
      }
      if (inverse) {
        for (let i = 0; i < n; i++) {
          real[i] = real[i] / n;
          imag[i] = imag[i] / n;
        }
      }
    }
  };

  // src/editor/engine/donor/openreel/audio/contracts.ts
  function createReverbImpulse(sampleRate2, params, channel) {
    const duration = 0.5 + params.roomSize * 3.5;
    const length = Math.floor(sampleRate2 * duration);
    const preDelay = Math.round(sampleRate2 * params.preDelay / 1e3);
    const impulse = new Float32Array(length + preDelay);
    let seed = 1831565813 + channel * 2654435769 >>> 0;
    let energy = 0;
    for (let i = 0; i < length; i++) {
      seed ^= seed << 13;
      seed ^= seed >>> 17;
      seed ^= seed << 5;
      const noise = (seed >>> 0) / 4294967295 * 2 - 1;
      const t = i / sampleRate2;
      const value = noise * Math.exp(-3 * t / duration) * (1 - params.damping * t / duration);
      impulse[i + preDelay] = value;
      energy += value * value;
    }
    const gain = 1 / Math.sqrt(energy || 1);
    for (let i = preDelay; i < impulse.length; i++) impulse[i] = impulse[i] * gain;
    return impulse;
  }

  // src/editor/engine/features/audio-fx/stages.ts
  var AudioStage = class {
    scratch = [];
    output(length, channel) {
      let buffer = this.scratch[channel];
      if (!buffer || buffer.length < length) this.scratch[channel] = buffer = new Float32Array(length);
      return buffer.subarray(0, length);
    }
  };
  var CompressorStage = class extends AudioStage {
    constructor(params, channels, sampleRate2) {
      super();
      this.params = params;
      this.envelopes = new Float64Array(channels);
      this.retainedBytes = this.envelopes.byteLength;
      this.attack = Math.exp(-1 / (sampleRate2 * params.attack));
      this.release = Math.exp(-1 / (sampleRate2 * params.release));
      this.makeup = Math.pow(10, params.makeupGain / 20);
    }
    params;
    tailFrames = 0;
    latencyFrames = 0;
    retainedBytes;
    envelopes;
    attack;
    release;
    makeup;
    processChunk(input, channel) {
      const output = this.output(input.length, channel);
      let envelope = this.envelopes[channel];
      const { threshold, ratio, knee } = this.params;
      const slope = 1 - 1 / ratio;
      for (let i = 0; i < input.length; i++) {
        const sample = input[i];
        const over = 20 * Math.log10(Math.max(1e-12, Math.abs(sample))) - threshold;
        const reduction = knee > 0 && over > -knee / 2 && over < knee / 2 ? slope * (over + knee / 2) ** 2 / (2 * knee) : over >= knee / 2 ? slope * over : 0;
        const coefficient = reduction > envelope ? this.attack : this.release;
        envelope = coefficient * envelope + (1 - coefficient) * reduction;
        output[i] = sample * Math.pow(10, -envelope / 20) * this.makeup;
      }
      this.envelopes[channel] = envelope;
      return output;
    }
  };
  var DelayStage = class extends AudioStage {
    constructor(params, channels, sampleRate2) {
      super();
      this.params = params;
      const length = Math.max(1, Math.round(sampleRate2 * params.time));
      this.lines = Array.from({ length: channels }, () => new Float32Array(length));
      this.positions = new Uint32Array(channels);
      const repeats = params.feedback > 0 ? Math.max(1, Math.ceil(Math.log(1e-4 / Math.max(params.wetLevel, 1e-4)) / Math.log(params.feedback))) : 1;
      this.tailFrames = params.wetLevel === 0 ? 0 : length * repeats;
      this.retainedBytes = length * channels * 4 + this.positions.byteLength;
    }
    params;
    latencyFrames = 0;
    tailFrames;
    retainedBytes;
    lines;
    positions;
    processChunk(input, channel) {
      const output = this.output(input.length, channel);
      const line = this.lines[channel];
      let position = this.positions[channel];
      for (let i = 0; i < input.length; i++) {
        const delayed = line[position];
        const sample = input[i];
        line[position] = sample + delayed * this.params.feedback;
        output[i] = sample * (1 - this.params.wetLevel) + delayed * this.params.wetLevel;
        if (++position === line.length) position = 0;
      }
      this.positions[channel] = position;
      return output;
    }
  };
  var NoiseReductionStage = class extends AudioStage {
    constructor(params, channels, sampleRate2) {
      super();
      this.params = params;
      if (!params.profile) throw new Error("C\u1EA7n l\u1EA5y m\u1EABu nhi\u1EC5u tr\u01B0\u1EDBc khi b\u1EADt gi\u1EA3m nhi\u1EC5u");
      const profile = params.profile;
      const n = profile.fftSize;
      this.fft = new FFT(n);
      this.latencyFrames = n;
      this.tailFrames = n;
      this.window = Float32Array.from({ length: n }, (_, i) => 0.5 * (1 - Math.cos(2 * Math.PI * i / (n - 1))));
      const bins = n / 2 + 1;
      this.noise = new Float32Array(bins);
      this.deviation = new Float32Array(bins);
      for (let i = 0; i < bins; i++) {
        const sourceBin = i * sampleRate2 / profile.sampleRate;
        const low = Math.floor(sourceBin);
        const fraction = sourceBin - low;
        this.noise[i] = (profile.magnitudes[low] ?? 0) * (1 - fraction) + (profile.magnitudes[low + 1] ?? 0) * fraction;
        this.deviation[i] = (profile.standardDeviations[low] ?? 0) * (1 - fraction) + (profile.standardDeviations[low + 1] ?? 0) * fraction;
      }
      this.states = Array.from({ length: channels }, () => ({
        input: new Float32Array(n),
        overlap: new Float32Array(n * 2),
        normalization: new Float32Array(n * 2),
        real: new Float32Array(n),
        imag: new Float32Array(n),
        consumed: 0
      }));
      this.retainedBytes = (n + bins * 2 + channels * n * 7) * 4;
    }
    params;
    tailFrames;
    latencyFrames;
    retainedBytes;
    fft;
    window;
    states;
    noise;
    deviation;
    frame(state, start) {
      const n = this.fft.size;
      for (let i = 0; i < n; i++) {
        const index = start + i;
        state.real[i] = index < 0 ? 0 : state.input[index % n] * this.window[i];
      }
      state.imag.fill(0);
      this.fft.transform(state.real, state.imag);
      const threshold = Math.pow(10, this.params.threshold / 20);
      for (let i = 0; i <= n / 2; i++) {
        const magnitude = Math.hypot(state.real[i], state.imag[i]);
        const subtracted = magnitude - (1 + this.params.reduction) * (this.noise[i] + this.deviation[i]);
        const floor = this.noise[i] * (1 - this.params.reduction) * threshold;
        const result = Math.max(subtracted, floor) * (1 - this.params.smoothing) + magnitude * this.params.smoothing;
        const gain = magnitude > 1e-20 ? result / magnitude : 0;
        state.real[i] = state.real[i] * gain;
        state.imag[i] = state.imag[i] * gain;
        if (i > 0 && i < n / 2) {
          state.real[n - i] = state.real[i];
          state.imag[n - i] = -state.imag[i];
        }
      }
      this.fft.transform(state.real, state.imag, true);
      for (let i = 0; i < n; i++) {
        if (start + i < 0) continue;
        const index = (start + i) % (n * 2);
        const window = this.window[i];
        state.overlap[index] = state.overlap[index] + state.real[i] * window;
        state.normalization[index] = state.normalization[index] + window * window;
      }
    }
    processChunk(input, channel) {
      const output = this.output(input.length, channel);
      const state = this.states[channel];
      const n = this.fft.size;
      const hop = n / 4;
      for (let i = 0; i < input.length; i++) {
        const t = state.consumed++;
        state.input[t % n] = input[i];
        if (state.consumed % hop === 0) this.frame(state, state.consumed - n);
        if (t < n) {
          output[i] = 0;
          continue;
        }
        const index = (t - n) % (n * 2);
        const weight = state.normalization[index];
        output[i] = weight > 1e-8 ? state.overlap[index] / weight : 0;
        state.overlap[index] = 0;
        state.normalization[index] = 0;
      }
      return output;
    }
  };
  var CONVOLUTION_BLOCK = 256;
  var ReverbStage = class extends AudioStage {
    constructor(params, channels, sampleRate2) {
      super();
      this.params = params;
      const impulses = Array.from({ length: channels }, (_, channel) => createReverbImpulse(sampleRate2, params, channel));
      const partitions = Math.ceil(impulses[0].length / CONVOLUTION_BLOCK);
      for (const impulse of impulses) {
        const reals = [];
        const imags = [];
        for (let p = 0; p < partitions; p++) {
          const real = new Float32Array(this.fft.size);
          const imag = new Float32Array(this.fft.size);
          real.set(impulse.subarray(p * CONVOLUTION_BLOCK, (p + 1) * CONVOLUTION_BLOCK));
          this.fft.transform(real, imag);
          reals.push(real);
          imags.push(imag);
        }
        this.kernelsReal.push(reals);
        this.kernelsImag.push(imags);
      }
      this.states = Array.from({ length: channels }, () => ({
        input: new Float32Array(CONVOLUTION_BLOCK),
        output: new Float32Array(CONVOLUTION_BLOCK),
        overlap: new Float32Array(CONVOLUTION_BLOCK),
        dry: new Float32Array(CONVOLUTION_BLOCK),
        historyReal: Array.from({ length: partitions }, () => new Float32Array(this.fft.size)),
        historyImag: Array.from({ length: partitions }, () => new Float32Array(this.fft.size)),
        sumReal: new Float32Array(this.fft.size),
        sumImag: new Float32Array(this.fft.size),
        position: 0,
        block: 0
      }));
      this.tailFrames = this.latencyFrames + (params.wetLevel > 0 ? impulses[0].length - 1 : 0);
      this.retainedBytes = channels * (partitions * this.fft.size * 4 + CONVOLUTION_BLOCK * 8) * 4;
    }
    params;
    latencyFrames = CONVOLUTION_BLOCK;
    tailFrames;
    retainedBytes;
    fft = new FFT(CONVOLUTION_BLOCK * 2);
    kernelsReal = [];
    kernelsImag = [];
    states;
    block(state, channel) {
      const kernelsReal = this.kernelsReal[channel];
      const kernelsImag = this.kernelsImag[channel];
      const count = kernelsReal.length;
      const head = state.block % count;
      const real = state.historyReal[head];
      const imag = state.historyImag[head];
      real.fill(0);
      real.set(state.input);
      imag.fill(0);
      this.fft.transform(real, imag);
      state.sumReal.fill(0);
      state.sumImag.fill(0);
      for (let p = 0; p < Math.min(count, state.block + 1); p++) {
        const index = (head - p + count) % count;
        const xr = state.historyReal[index];
        const xi = state.historyImag[index];
        const hr = kernelsReal[p];
        const hi = kernelsImag[p];
        for (let i = 0; i < this.fft.size; i++) {
          state.sumReal[i] = state.sumReal[i] + xr[i] * hr[i] - xi[i] * hi[i];
          state.sumImag[i] = state.sumImag[i] + xr[i] * hi[i] + xi[i] * hr[i];
        }
      }
      this.fft.transform(state.sumReal, state.sumImag, true);
      for (let i = 0; i < CONVOLUTION_BLOCK; i++) {
        state.output[i] = state.sumReal[i] + state.overlap[i];
        state.overlap[i] = state.sumReal[i + CONVOLUTION_BLOCK];
      }
      state.block++;
    }
    processChunk(input, channel) {
      const output = this.output(input.length, channel);
      const state = this.states[channel];
      for (let i = 0; i < input.length; i++) {
        const p = state.position;
        output[i] = state.dry[p] * this.params.dryLevel + state.output[p] * this.params.wetLevel;
        state.dry[p] = input[i];
        state.input[p] = input[i];
        if (++state.position === CONVOLUTION_BLOCK) {
          this.block(state, channel);
          state.position = 0;
        }
      }
      return output;
    }
  };
  function createAudioFxStages(effects, channels, sampleRate2) {
    return (effects ?? []).filter((effect) => effect.enabled).map((effect) => {
      switch (effect.type) {
        case "compressor":
          return new CompressorStage(effect.params, channels, sampleRate2);
        case "noiseReduction":
          return new NoiseReductionStage(effect.params, channels, sampleRate2);
        case "reverb":
          return new ReverbStage(effect.params, channels, sampleRate2);
        case "delay":
          return new DelayStage(effect.params, channels, sampleRate2);
      }
    });
  }
  var AudioFxChain = class {
    constructor(stages) {
      this.stages = stages;
      this.tailFrames = stages.reduce((sum, stage) => sum + stage.tailFrames, 0);
      this.latencyFrames = stages.reduce((sum, stage) => sum + stage.latencyFrames, 0);
      this.retainedBytes = stages.reduce((sum, stage) => sum + stage.retainedBytes, 0);
    }
    stages;
    tailFrames;
    latencyFrames;
    retainedBytes;
    processChunk(input, channel) {
      let output = input;
      for (const stage of this.stages) output = stage.processChunk(output, channel);
      return output;
    }
  };
  function createAudioFxChain(effects, channels, sampleRate2) {
    return new AudioFxChain(createAudioFxStages(effects, channels, sampleRate2));
  }

  // src/editor/engine/runtime/composition-runtime/worklets/audio-fx-processor.worklet.ts
  var AudioFxProcessor = class extends AudioWorkletProcessor {
    effects = [];
    chain = createAudioFxChain([], 2, sampleRate);
    zero = new Float32Array(128);
    channels = 2;
    recording = null;
    constructor() {
      super();
      this.port.onmessage = (event) => {
        if (event.data.type === "record") {
          this.recording = { startFrame: event.data.startFrame, frames: event.data.frames, sent: 0 };
          return;
        }
        if (event.data.type === "configure") this.effects = event.data.effects ?? [];
        this.chain = createAudioFxChain(this.effects, this.channels, sampleRate);
      };
    }
    process(inputs, outputs) {
      const output = outputs[0];
      if (!output?.length) return true;
      if (this.recording && currentFrame === this.recording.startFrame) {
        this.chain = createAudioFxChain(this.effects, this.channels, sampleRate);
      }
      if (this.channels !== output.length) {
        this.channels = output.length;
        this.chain = createAudioFxChain(this.effects, this.channels, sampleRate);
      }
      const input = inputs[0];
      for (let channel = 0; channel < output.length; channel++) {
        const target = output[channel];
        if (this.zero.length !== target.length) this.zero = new Float32Array(target.length);
        const source = input?.[channel] ?? input?.[0] ?? this.zero;
        target.set(this.chain.processChunk(source, channel));
      }
      const recording = this.recording;
      if (recording && currentFrame >= recording.startFrame && recording.sent < recording.frames) {
        const length = Math.min(output[0].length, recording.frames - recording.sent);
        const channels = output.map((channel) => channel.slice(0, length).buffer);
        recording.sent += length;
        this.port.postMessage({ type: "record-chunk", channels, done: recording.sent === recording.frames }, channels);
        if (recording.sent === recording.frames) this.recording = null;
      }
      return true;
    }
  };
  registerProcessor("ytf-audio-fx", AudioFxProcessor);
})();
