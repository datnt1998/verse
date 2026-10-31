"use client";

export type AnnouncementPoliteness = "polite" | "assertive";
type Listener = (text: string, politeness: AnnouncementPoliteness) => void;
const listeners = new Set<Listener>();

export function announce(text: string, politeness: AnnouncementPoliteness = "polite") {
  listeners.forEach((listener) => listener(text, politeness));
}

export function subscribeAnnouncements(listener: Listener) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
