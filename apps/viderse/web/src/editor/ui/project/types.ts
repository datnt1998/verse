export interface ProjectCardData {
  id: string;
  name: string;
  width: number;
  height: number;
  fps: number;
  duration: number;
  updatedAt: number;
  revision: string;
  /** Clone job metadata when this project was built by the clone pipeline. */
  clone?: {
    jobId: string;
    status: string;
    wizardUrl: string;
    progress: { done: number; total: number };
    source: { title: string | null; thumbnailUrl: string | null; channelTitle: string | null };
  };
}
