/**
 * Class names from `timeline.css` shared by the panel and the clip, plus the track header
 * column width, which is the left edge of all time coordinates.
 */
export const TIMELINE_HEADER = 180;

export const timelineStyles = {
  panel: 'ytf-timeline-panel',
  toolbar: 'ytf-timeline-toolbar',
  scroll: 'ytf-timeline-scroll',
  surface: 'ytf-timeline-surface',
  ruler: 'ytf-timeline-ruler',
  rulerCorner: 'ytf-timeline-ruler-corner',
  tick: 'ytf-timeline-tick',
  track: 'ytf-timeline-track',
  header: 'ytf-timeline-header',
  trackName: 'ytf-timeline-track-name',
  trackButtons: 'ytf-timeline-track-buttons',
  lane: 'ytf-timeline-lane',
  clip: 'ytf-timeline-clip',
  clipButton: 'ytf-timeline-clip-button',
  label: 'ytf-timeline-label',
  handle: 'ytf-timeline-handle',
  start: 'ytf-timeline-start',
  end: 'ytf-timeline-end',
  fade: 'ytf-timeline-fade',
  keyframe: 'ytf-timeline-keyframe',
  playhead: 'ytf-timeline-playhead',
  guide: 'ytf-timeline-guide',
  range: 'ytf-timeline-range',
  rangeHandle: 'ytf-timeline-range-handle',
  marker: 'ytf-timeline-marker',
  marquee: 'ytf-timeline-marquee',
  footer: 'ytf-timeline-footer',
  transition: 'ytf-timeline-transition',
} as const;
