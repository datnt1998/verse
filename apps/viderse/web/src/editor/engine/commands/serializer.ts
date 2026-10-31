import { z } from 'zod'
import type { SerializedCommand } from './types'
const serializedCommand = z.object({ type: z.string().min(1), params: z.json() }).strict()
export function serializeCommand(command: SerializedCommand): string { return JSON.stringify(serializedCommand.parse(command)) }
export function deserializeCommand(json: string): SerializedCommand { return serializedCommand.parse(JSON.parse(json)) }
export function serializeCommands(commands: SerializedCommand[]): string { return JSON.stringify(z.array(serializedCommand).parse(commands)) }
export function deserializeCommands(json: string): SerializedCommand[] { return z.array(serializedCommand).parse(JSON.parse(json)) }
