import { HttpErrorResponse } from '@angular/common/http';
import { DestroyRef, Injectable, InjectionToken, inject, signal } from '@angular/core';
import { Subscription } from 'rxjs';
import { formatWait, retryAfterSeconds } from './story-plan-problem';
import { StoryPlanService } from './story-plan.service';
import { VoiceTake } from './story-plan.models';

/** What one scene card shows about its voice. */
export type VoiceView =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'playing' }
  /** `limit` is true when the day's allowance is used up, which is not a fault. */
  | { kind: 'problem'; message: string; limit: boolean };

export const IDLE_VOICE: VoiceView = { kind: 'idle' };

/** The part of an audio element the preview uses, so a test can stand in for it. */
export interface PlaybackElement {
  src: string;
  play(): Promise<void>;
  pause(): void;
  addEventListener(type: 'playing' | 'ended' | 'error', listener: () => void): void;
  removeEventListener(type: 'playing' | 'ended' | 'error', listener: () => void): void;
}

export const PLAYBACK_FACTORY = new InjectionToken<() => PlaybackElement>('story-voice-playback', {
  providedIn: 'root',
  factory: () => () => new Audio(),
});

export interface VoiceRequest {
  planId: string;
  revision: number;
  /** 0-based index of the scene. */
  scene: number;
  /** The narration as it is now, saved or not. */
  text: string;
  /** The voice chosen for the video; empty for the language's default. */
  voice: string;
}

/** Links last a few minutes; this much before the end a kept link is no longer reused. */
const LINK_MARGIN_MS = 30_000;

/**
 * Plays one scene's narration at a time (#326). The audio is made by the API,
 * once, and kept; this class asks for it, plays it, and remembers the signed
 * link for as long as it works, so playing a scene again costs no request.
 *
 * Provided by the editor page (not root), so it lives and dies with the page and
 * stops any sound when the page goes.
 */
@Injectable()
export class StoryVoicePreview {
  private readonly service = inject(StoryPlanService);
  private readonly makeAudio = inject(PLAYBACK_FACTORY);

  /** False once the API says previews are switched off (503), so the buttons go. */
  readonly available = signal(true);
  /** New takes left today, once a take has been made. */
  readonly remaining = signal<number | null>(null);
  readonly dailyLimit = signal<number | null>(null);

  private readonly current = signal<{ scene: number; view: VoiceView } | null>(null);
  private audio: PlaybackElement | null = null;
  private detach: (() => void) | null = null;
  private request: Subscription | null = null;
  private readonly links = new Map<string, { url: string; until: number }>();
  /** Counts attempts, so an answer that arrives after a stop or another scene is ignored. */
  private ticket = 0;

  constructor() {
    inject(DestroyRef).onDestroy(() => this.stop());
  }

  /** What the card for scene `index` should show. */
  view(index: number): VoiceView {
    const c = this.current();
    return c && c.scene === index ? c.view : IDLE_VOICE;
  }

  /** Plays the scene, or stops it when it is the one playing or being made. */
  toggle(req: VoiceRequest): void {
    const c = this.current();
    if (c && c.scene === req.scene && (c.view.kind === 'loading' || c.view.kind === 'playing')) {
      this.stop();
      return;
    }
    this.play(req, false);
  }

  /** Stops the sound and any request in flight. */
  stop(): void {
    this.ticket++;
    this.request?.unsubscribe();
    this.request = null;
    this.endAudio();
    this.current.set(null);
  }

  private play(req: VoiceRequest, retried: boolean): void {
    this.stop();
    const text = req.text.trim();
    if (!text) return;
    const ticket = this.ticket;
    const key = `${req.voice}\u0000${text}`;
    const kept = this.links.get(key);
    if (kept && kept.until > Date.now()) {
      this.current.set({ scene: req.scene, view: { kind: 'loading' } });
      this.start(req, ticket, key, kept.url, retried);
      return;
    }
    this.links.delete(key);
    this.current.set({ scene: req.scene, view: { kind: 'loading' } });
    const body = { revision: req.revision, scene: req.scene + 1, narration: text, ...(req.voice ? { voice: req.voice } : {}) };
    this.request = this.service.previewVoice(req.planId, body).subscribe({
      next: (take) => {
        if (ticket !== this.ticket) return;
        this.learn(take);
        this.links.set(key, { url: take.url, until: Date.now() + take.expiresInSeconds * 1000 - LINK_MARGIN_MS });
        this.start(req, ticket, key, take.url, retried);
      },
      error: (err: unknown) => {
        if (ticket !== this.ticket) return;
        this.fail(req.scene, err);
      },
    });
  }

  private learn(take: VoiceTake): void {
    this.dailyLimit.set(take.dailyLimit);
    if (typeof take.remainingToday === 'number') this.remaining.set(take.remainingToday);
  }

  private start(req: VoiceRequest, ticket: number, key: string, url: string, retried: boolean): void {
    const audio = this.makeAudio();
    this.audio = audio;
    const onPlaying = () => {
      if (ticket === this.ticket) this.current.set({ scene: req.scene, view: { kind: 'playing' } });
    };
    const onEnded = () => {
      if (ticket === this.ticket) this.stop();
    };
    const onError = () => {
      if (ticket !== this.ticket) return;
      // A link that stopped working (it expired, or the take was removed): ask once more.
      this.links.delete(key);
      if (!retried) {
        this.play(req, true);
        return;
      }
      this.stop();
      this.current.set({ scene: req.scene, view: { kind: 'problem', message: 'The voice could not be played. Try again.', limit: false } });
    };
    audio.addEventListener('playing', onPlaying);
    audio.addEventListener('ended', onEnded);
    audio.addEventListener('error', onError);
    this.detach = () => {
      audio.removeEventListener('playing', onPlaying);
      audio.removeEventListener('ended', onEnded);
      audio.removeEventListener('error', onError);
    };
    audio.src = url;
    audio.play().catch((err: unknown) => {
      if (ticket !== this.ticket) return;
      const blocked = (err as { name?: string } | null)?.name === 'NotAllowedError';
      // An interrupted play (we stopped it ourselves) is not a problem to show.
      if ((err as { name?: string } | null)?.name === 'AbortError') return;
      this.stop();
      this.current.set({
        scene: req.scene,
        view: {
          kind: 'problem',
          message: blocked ? 'Your browser blocked the sound. Press the button again.' : 'The voice could not be played. Try again.',
          limit: false,
        },
      });
    });
  }

  private endAudio(): void {
    this.detach?.();
    this.detach = null;
    if (this.audio) {
      try {
        this.audio.pause();
      } catch {
        // Already gone.
      }
      this.audio = null;
    }
  }

  private fail(scene: number, err: unknown): void {
    this.request = null;
    const problem = describeVoiceProblem(err, this.dailyLimit());
    if (problem.disabled) this.available.set(false);
    this.current.set(problem.disabled ? null : { scene, view: { kind: 'problem', message: problem.message, limit: problem.limit } });
  }
}

/** What to tell the person when the voice could not be had. Never repeats their text. */
export function describeVoiceProblem(err: unknown, dailyLimit: number | null): { message: string; limit: boolean; disabled: boolean } {
  const e = err instanceof HttpErrorResponse ? err : null;
  const body = (e?.error ?? null) as { code?: unknown; message?: unknown } | null;
  const code = typeof body?.code === 'string' ? body.code : '';
  const serverMessage = typeof body?.message === 'string' ? body.message.trim() : '';
  const ok = (message: string, limit = false) => ({ message, limit, disabled: false });
  if (!e || e.status === 0) return ok('You seem to be offline. Check the connection and try again.');
  if (e.status === 503 && code === 'UNAVAILABLE') return { message: '', limit: false, disabled: true };
  if (e.status === 429 && code === 'QUOTA_EXCEEDED') {
    const n = dailyLimit ? `${dailyLimit} ` : '';
    return ok(`You have used today's ${n}new voice previews. Scenes you already heard still play. It resets at midnight UTC.`, true);
  }
  if (e.status === 429) {
    const wait = retryAfterSeconds(e);
    return ok(`The voice is busy. Try again in ${formatWait(wait ?? 10)}.`);
  }
  if (e.status === 409 && code === 'STALE_REVISION') return ok('This plan was changed somewhere else. Reload the page, then try again.');
  if (e.status === 409 && code === 'NOT_READY') return ok('This plan has no scenes to hear yet.');
  if (e.status === 404) return ok('This plan could not be found. Reload the page.');
  if ((e.status === 400 || e.status === 403) && serverMessage) return ok(serverMessage);
  return ok('The voice could not be made. Try again in a few minutes.');
}
