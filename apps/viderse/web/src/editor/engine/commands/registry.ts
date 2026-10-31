import type { CommandDefinition } from './types'
const registry = new Map<string, CommandDefinition<unknown>>()
export function registerCommand<P>(definition: CommandDefinition<P>): void {
  registry.set(definition.type, definition as CommandDefinition<unknown>)
}
export function getCommand(type: string): CommandDefinition<unknown> | undefined { return registry.get(type) }
export function listCommands(): CommandDefinition<unknown>[] { return [...registry.values()].sort((a, b) => a.type.localeCompare(b.type)) }
export function listCommandTypes(): string[] { return listCommands().map(command => command.type) }
