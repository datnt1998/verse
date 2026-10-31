/** Uniform center-anchored resize, projecting screen movement onto the local diagonal. */
export function gizmoScaleFactor(width: number, height: number, rotation: number, deltaX: number, deltaY: number): number {
  const angle = rotation * Math.PI / 180;
  const localX = deltaX * Math.cos(angle) + deltaY * Math.sin(angle);
  const localY = -deltaX * Math.sin(angle) + deltaY * Math.cos(angle);
  const diagonalSquared = width * width + height * height;
  return diagonalSquared > 0 ? Math.max(.02, 1 + 2 * (localX * width + localY * height) / diagonalSquared) : 1;
}
