import type { FullConfig } from '@playwright/test';
import { getModuleStack, teardown } from './harness';

export default async function globalTeardown(_config: FullConfig): Promise<void> {
  const stack = getModuleStack();
  if (!stack) return;
  await teardown(stack);
}
