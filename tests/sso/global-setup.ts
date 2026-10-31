import type { FullConfig } from '@playwright/test';
import { provision, setModuleStack, getModuleStack, teardown, STATE_ENV } from './harness';

export default async function globalSetup(_config: FullConfig): Promise<void> {
  const stack = await provision().catch(async err => {
    const partial = getModuleStack();
    if (partial) await teardown(partial);
    throw err;
  });
  setModuleStack(stack);
  process.env[STATE_ENV] = stack.dir;
  // Last-resort kill on process exit: global-teardown does the graceful stop;
  // this only fires when the runner dies before teardown ran.
  process.on('exit', () => {
    for (const srv of [stack.noverse, stack.viderse, stack.hostedNoverse, stack.hostedViderse, stack.proxy, ...stack.extraServers]) {
      try { if (srv?.proc.pid) process.kill(-srv.proc.pid, 'SIGKILL'); } catch { /* gone */ }
    }
  });
}
