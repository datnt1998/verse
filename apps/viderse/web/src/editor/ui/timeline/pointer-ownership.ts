/**
 * React delivers events from portaled UI (context menus, dialogs) to the
 * component that rendered the portal. A timeline gesture must only start from
 * pointers that landed on the gesture surface itself, never from a menu item
 * that merely shares its React ancestors.
 */
export function ownsPointerEvent(event: { currentTarget: EventTarget; target: EventTarget }): boolean {
  return event.currentTarget instanceof Node && event.target instanceof Node && event.currentTarget.contains(event.target);
}
