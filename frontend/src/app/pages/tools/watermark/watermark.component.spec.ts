import { TestBed } from '@angular/core/testing';
import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter } from '@angular/router';
import {
  provideLucideIcons,
  LucideCheck,
  LucideChevronRight,
  LucideDownload,
  LucideImage,
  LucideLock,
  LucidePause,
  LucidePlay,
  LucideRotateCcw,
  LucideShieldCheck,
  LucideUpload,
  LucideUserX,
} from '@lucide/angular';
import { WatermarkComponent } from './watermark.component';
import { FF_CORE_JS_SHA384, FF_CORE_JS_URL, FF_CORE_WASM_SHA384, FF_CORE_WASM_URL } from './ffmpeg-core';

/** A tiny real PNG (2 by 2), so the image path runs end to end in the browser. */
async function pngFile(): Promise<File> {
  const c = document.createElement('canvas');
  c.width = 2;
  c.height = 2;
  c.getContext('2d')!.fillRect(0, 0, 2, 2);
  const blob: Blob = await new Promise((r) => c.toBlob((b) => r(b!), 'image/png'));
  return new File([blob], 'photo.png', { type: 'image/png' });
}

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

describe('WatermarkComponent', () => {
  let fetchSpy: jasmine.Spy;
  let xhrOpen: jasmine.Spy;
  let beacon: jasmine.Spy;

  function setup() {
    TestBed.configureTestingModule({
      imports: [WatermarkComponent],
      providers: [
        provideRouter([]),
        provideHttpClient(),
        provideHttpClientTesting(),
        provideLucideIcons(LucideCheck, LucideChevronRight, LucideDownload, LucideImage, LucideLock, LucidePause, LucidePlay, LucideRotateCcw, LucideShieldCheck, LucideUpload, LucideUserX),
      ],
    });
    const fixture = TestBed.createComponent(WatermarkComponent);
    fixture.detectChanges();
    return { fixture, cmp: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
  }

  beforeEach(() => {
    // Watch every way a page can send bytes out. Blob and data URLs are local reads.
    const realFetch = window.fetch.bind(window);
    fetchSpy = spyOn(window, 'fetch').and.callFake((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input instanceof Request ? input.url : input);
      if (url.startsWith('blob:') || url.startsWith('data:')) return realFetch(input, init);
      return Promise.reject(new Error('blocked in test: ' + url));
    });
    xhrOpen = spyOn(XMLHttpRequest.prototype, 'open').and.callThrough();
    beacon = spyOn(navigator, 'sendBeacon').and.returnValue(true);
  });

  function outbound(): string[] {
    const fromFetch = fetchSpy.calls.allArgs().map((a) => String(a[0] instanceof Request ? a[0].url : a[0]));
    const fromXhr = xhrOpen.calls.allArgs().map((a) => String(a[1]));
    return [...fromFetch, ...fromXhr].filter((u) => !u.startsWith('blob:') && !u.startsWith('data:'));
  }

  it('shows the editor entry point first, with the browser-only note, and no sign-in gate', () => {
    const { el } = setup();
    const main = el.querySelector('main')!;
    expect(main.querySelector('[data-testid="upload"]')).toBeTruthy();
    expect(main.textContent).toContain('Drop an image or video here');
    expect(el.querySelector('[data-testid="local-note"]')!.textContent).toContain('never uploaded');
    expect(el.querySelector('a[href="/login"]')?.closest('main')).toBeFalsy();
    // The editor comes before the how-it-works steps and the FAQ in the page.
    const order = Array.from(el.querySelectorAll('main, app-watermark-about'));
    expect(order[0].tagName).toBe('MAIN');
    expect(el.querySelectorAll('h1').length).toBe(1);
  });

  it('accepts a dropped image and opens the editor with before/after and download', async () => {
    const { fixture, cmp, el } = setup();
    const file = await pngFile();
    const dt = new DataTransfer();
    dt.items.add(file);
    cmp.onDrop(new DragEvent('drop', { dataTransfer: dt }));
    fixture.detectChanges();
    await wait(200);
    fixture.detectChanges();
    expect(cmp.file?.name).toBe('photo.png');
    expect(cmp.fileType).toBe('image');
    expect(el.textContent).toContain('Before / After');
    expect(el.textContent).toContain('Download PNG');
    expect(el.querySelector('[data-testid="local-note"]')!.textContent).toContain('Nothing is uploaded');
    // An image is never handed to the video element, so no codec error appears.
    expect(el.querySelector('video')).toBeNull();
    expect(cmp.fileError).toBe('');
  });

  it('watermarks and downloads an image without any network request or backend call', async () => {
    const { fixture, cmp } = setup();
    const http = TestBed.inject(HttpTestingController);
    const input = document.createElement('input');
    const dt = new DataTransfer();
    dt.items.add(await pngFile());
    input.type = 'file';
    input.files = dt.files;
    cmp.onFileSelected({ target: input } as unknown as Event);
    fixture.detectChanges();
    await wait(300);
    fixture.detectChanges();
    cmp.setImgFormat('jpeg');
    await wait(200);
    expect(cmp.downloadUrl.startsWith('blob:')).toBeTrue();
    const click = spyOn(HTMLAnchorElement.prototype, 'click');
    cmp.download();
    expect(click).toHaveBeenCalled();
    expect(outbound()).toEqual([]);
    expect(beacon).not.toHaveBeenCalled();
    http.verify();
  });

  it('keeps the pinned ffmpeg core: the only remote files the video export may fetch', () => {
    expect(FF_CORE_JS_URL).toBe('https://cdn.jsdelivr.net/npm/@ffmpeg/core-st@0.11.1/dist/ffmpeg-core.js');
    expect(FF_CORE_WASM_URL).toBe('https://cdn.jsdelivr.net/npm/@ffmpeg/core-st@0.11.1/dist/ffmpeg-core.wasm');
    expect(FF_CORE_JS_SHA384).toBe('BeUhGK5N9KTyO9NFKUSUBTxwvxLI4EZFbynF9mLtj73/2DqmPqGLk6EJ9ZU+R+6J');
    expect(FF_CORE_WASM_SHA384).toBe('vAbQ21XJMfp9RyAaryzB4ciuXISd636UpiZYr7vDBAuX+NmAYvnQNdvA3RubvdHH');
  });

  it('warns before a file over 500 MB and lets the user go on or pick another', () => {
    const { fixture, cmp, el } = setup();
    const big = new File(['x'], 'big.mp4', { type: 'video/mp4' });
    Object.defineProperty(big, 'size', { value: 501 * 1024 * 1024 });
    (cmp as unknown as { handleFile(f: File): void }).handleFile(big);
    fixture.detectChanges();
    expect(cmp.sizeGate).toBeTrue();
    expect(el.textContent).toContain('Large file (over 500');
    expect(el.textContent).toContain('Process anyway');
  });

  it('keeps the share links to the public tool URL', () => {
    const { cmp } = setup();
    expect(cmp.xShareHref).toContain(encodeURIComponent('https://indifferent.fun/tools/add-watermark'));
    expect(cmp.facebookShareHref).toContain(encodeURIComponent('https://indifferent.fun/tools/add-watermark'));
    expect(cmp.whatsappShareHref).toContain('wa.me');
  });

  it('keeps hex colours out of the page markup', () => {
    const { el } = setup();
    expect(el.innerHTML).not.toMatch(/#[0-9A-Fa-f]{6}\b/);
  });
});
