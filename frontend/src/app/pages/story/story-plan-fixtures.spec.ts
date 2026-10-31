// Shared test data for the Story2Shorts screens. A .spec.ts file so it stays out
// of the production build; it holds no tests of its own.
import { Component } from '@angular/core';
import { HttpErrorResponse, HttpHeaders } from '@angular/common/http';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import {
  LucideCheck,
  LucideCircleX,
  LucideClock,
  LucideFilm,
  LucideImage,
  LucideLoader2,
  LucideSearch,
  LucideTrash2,
  LucideTriangleAlert,
  LucideX,
  provideLucideIcons,
} from '@lucide/angular';
import { of } from 'rxjs';
import { StoryPlanService } from './story-plan.service';
import {
  StockAsset,
  StockAttribution,
  StockSearchResponse,
  StoryPlanLimits,
  StoryPlanRecord,
} from './story-plan.models';

export const STORY_ICONS = provideLucideIcons(
  LucideCheck,
  LucideCircleX,
  LucideClock,
  LucideFilm,
  LucideImage,
  LucideLoader2,
  LucideSearch,
  LucideTrash2,
  LucideTriangleAlert,
  LucideX,
);

/** Stands in for the navbar, which needs the whole auth stack. */
@Component({ selector: 'app-navbar', standalone: true, template: '' })
export class NavbarStub {}

export function asset(over: Partial<StockAsset> = {}): StockAsset {
  const id = over.id ?? '101';
  const provider = over.provider ?? 'pixabay';
  const type = over.type ?? 'photo';
  return {
    provider,
    type,
    id,
    previewUrl: `https://cdn.example/${provider}/${id}.jpg`,
    pageUrl: `https://${provider}.example/${id}`,
    width: 1280,
    height: 1920,
    orientation: 'portrait',
    alt: `A picture numbered ${id}`,
    author: 'Jo Doe',
    credit: `${type === 'video' ? 'Video' : 'Photo'} ${id} by Jo Doe on ${provider === 'pexels' ? 'Pexels' : 'Pixabay'}`,
    ...(type === 'video' ? { playUrl: `https://cdn.example/${provider}/${id}.mp4`, duration: 9 } : {}),
    ...over,
  };
}

export const PEXELS_CREDIT: StockAttribution = { provider: 'pexels', text: 'Photos provided by Pexels', url: 'https://www.pexels.com' };
export const PIXABAY_CREDIT: StockAttribution = { provider: 'pixabay', text: 'Photos provided by Pixabay', url: 'https://pixabay.com' };

/** A ready plan with three scenes: a saved photo, a saved clip, and no picture. */
export function planRecord(over: Partial<StoryPlanRecord> = {}): StoryPlanRecord {
  return {
    planId: 'plan-1',
    status: 'ready',
    model: 'kimi',
    revision: 1,
    createdAt: '2026-10-30T10:00:00Z',
    updatedAt: '2026-10-30T10:00:00Z',
    plan: {
      language: 'en',
      hook: 'The lighthouse stood alone.',
      totalSeconds: 15.7,
      scenes: [
        {
          narration: 'The lighthouse stood alone on the cliff.',
          description: 'A white lighthouse on a stormy cliff',
          keywords: ['lighthouse', 'cliff', 'storm', 'ignored fourth'],
          mediaType: 'photo',
          duration: 6.2,
          media: { provider: 'pixabay', type: 'photo', id: '101' },
        },
        {
          narration: 'Every night the keeper climbed the stairs.',
          description: 'A spiral staircase inside a lighthouse',
          keywords: ['spiral staircase', 'lamp'],
          mediaType: 'video',
          duration: 5.1,
          media: { provider: 'pexels', type: 'video', id: '202' },
        },
        {
          narration: 'One morning the sea was calm again.',
          description: 'A calm sea at sunrise',
          keywords: ['calm sea', 'sunrise'],
          mediaType: 'photo',
          duration: 4.4,
        },
      ],
    },
    media: [
      { provider: 'pixabay', type: 'photo', id: '101', asset: asset({ id: '101' }) },
      { provider: 'pexels', type: 'video', id: '202', asset: asset({ id: '202', provider: 'pexels', type: 'video' }) },
      null,
    ],
    mediaAttributions: [PEXELS_CREDIT, PIXABAY_CREDIT],
    ...over,
  };
}

export function searchResponse(over: Partial<StockSearchResponse> = {}): StockSearchResponse {
  return {
    assets: [asset({ id: '301' }), asset({ id: '302' }), asset({ id: '303', provider: 'pexels' })],
    attributions: [PEXELS_CREDIT, PIXABAY_CREDIT],
    page: 1,
    perPage: 12,
    hasMore: false,
    ...over,
  };
}

export const LIMITS: StoryPlanLimits = {
  enabled: true,
  minStoryChars: 30,
  maxStoryChars: 5000,
  minScenes: 2,
  maxScenes: 10,
  maxTotalSeconds: 90,
  perHour: 20,
};

/** What the API returns for a failure. `status` 0 is no response at all. */
export function httpError(status: number, code = '', message = '', details?: unknown): HttpErrorResponse {
  return new HttpErrorResponse({
    status,
    error: status === 0 ? new ProgressEvent('error') : { code, message, ...(details ? { details } : {}) },
    headers: new HttpHeaders(),
  });
}

/** One failure per status and code the backend can answer, for table tests. */
export const API_FAILURES: { name: string; error: HttpErrorResponse; title: string }[] = [
  { name: 'rate limited', error: httpError(429, 'RATE_LIMITED', '', { retryAfter: 1740 }), title: "You've used this hour's plans" },
  { name: 'planner busy', error: httpError(503, 'PLAN_UNAVAILABLE'), title: 'The planner is busy' },
  { name: 'planner too slow', error: httpError(504, 'PLAN_TIMEOUT'), title: 'The planner took too long' },
  { name: 'unusable plan', error: httpError(502, 'PLAN_FAILED'), title: "The planner didn't return a usable plan" },
  { name: 'offline', error: httpError(0), title: "You're offline" },
];

export type ServiceSpy = jasmine.SpyObj<StoryPlanService>;

export function serviceSpy(): ServiceSpy {
  const spy = jasmine.createSpyObj<StoryPlanService>('StoryPlanService', ['getLimits', 'create', 'get', 'save', 'search']);
  spy.getLimits.and.returnValue(of(LIMITS));
  return spy;
}

export function routeFor(id: string) {
  return {
    provide: ActivatedRoute,
    useValue: { paramMap: of(convertToParamMap({ id })), snapshot: { paramMap: convertToParamMap({ id }) } },
  };
}

/** Lets pending promises and signal effects settle, then renders. */
export async function settle(fixture: { detectChanges(): void; whenStable(): Promise<unknown> }): Promise<void> {
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
}
