// Manual browser companion to the two-app smoke suite. Reuses exactly the
// same isolated provisioning and cleanup; never attaches to a live deployment.
import path from 'node:path';
import { provision, getModuleStack, teardown, redact } from './harness';

async function main(): Promise<void> {
  const stopped = Promise.withResolvers<void>();
  const stop = () => stopped.resolve();
  process.once('SIGINT', stop);
  process.once('SIGTERM', stop);
  try {
    const stack = await provision();
    // This private file contains disposable credentials; stdout prints only
    // its path, never the credentials themselves.
    console.log(`SSO_FIXTURE_READY ${path.join(stack.dir, 'state.json')}`);
    await stopped.promise;
  } finally {
    const stack = getModuleStack();
    if (stack) await teardown(stack);
    process.removeListener('SIGINT', stop);
    process.removeListener('SIGTERM', stop);
  }
}

main().catch((error: unknown) => {
  console.error(redact(error instanceof Error ? error.message : String(error)));
  process.exitCode = 1;
});
