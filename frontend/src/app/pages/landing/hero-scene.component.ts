import { AfterViewInit, Component, ElementRef, OnDestroy, PLATFORM_ID, inject, signal, viewChild } from '@angular/core';
import { isPlatformBrowser } from '@angular/common';
import type * as T from './three-subset';

type SceneState = 'static' | 'loading' | 'ready' | 'failed';

/**
 * Hero visual: the two slides every video is built from (a question slide, then
 * an answer slide), rendered by Indifferent from the sample file.
 *
 * The static <img> is the first paint and the fallback. three.js and GSAP load
 * after the page is idle, only when motion is welcome, and the scene renders
 * only when something changed (pointer, scroll, resize). Nothing loops.
 */
@Component({
  selector: 'app-hero-scene',
  standalone: true,
  styles: [
    `
      :host {
        display: block;
      }
      .hs-stage {
        position: relative;
        width: 100%;
        max-width: 540px;
        aspect-ratio: 16 / 10;
      }
      .hs-img {
        position: absolute;
        inset: 0;
        margin: auto;
        width: 86%;
        height: auto;
        border-radius: 10px;
        border: 1px solid rgba(255, 255, 255, 0.18);
        box-shadow: 0 24px 60px rgba(0, 0, 0, 0.45);
        transition: opacity 0.5s ease;
      }
      .hs-canvas {
        position: absolute;
        inset: 0;
        width: 100%;
        height: 100%;
        opacity: 0;
        transition: opacity 0.5s ease;
      }
      .hs-stage[data-scene='ready'] .hs-canvas {
        opacity: 1;
      }
      .hs-stage[data-scene='ready'] .hs-img {
        opacity: 0;
      }
    `,
  ],
  template: `
    <div #stage class="hs-stage" [attr.data-scene]="state()">
      <img
        class="hs-img"
        src="landing/slide-question.webp"
        width="1280"
        height="720"
        fetchpriority="high"
        decoding="async"
        alt="A question slide made by Indifferent: Question 1 of 3, What is the capital of France? Options A London, B Paris, C Berlin, D Madrid."
      />
      <canvas #canvas class="hs-canvas" aria-hidden="true"></canvas>
    </div>
    <p class="mt-4 text-[13px] leading-relaxed text-white/60">
      Real output from the sample file: each question is a question slide, then an answer slide.
    </p>
  `,
})
export class HeroSceneComponent implements AfterViewInit, OnDestroy {
  protected readonly state = signal<SceneState>('static');

  private readonly isBrowser = isPlatformBrowser(inject(PLATFORM_ID));
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly stage = viewChild.required<ElementRef<HTMLElement>>('stage');
  private readonly canvas = viewChild.required<ElementRef<HTMLCanvasElement>>('canvas');

  private destroyed = false;
  private cleanups: Array<() => void> = [];

  ngAfterViewInit(): void {
    if (!this.isBrowser) return;
    if (!this.motionWelcome()) return;
    const start = () => {
      if (!this.destroyed) void this.init();
    };
    const afterLoad = () => {
      const w = window as Window & { requestIdleCallback?: (cb: () => void, o?: { timeout: number }) => number };
      if (w.requestIdleCallback) w.requestIdleCallback(start, { timeout: 2500 });
      else setTimeout(start, 600);
    };
    if (document.readyState === 'complete') afterLoad();
    else {
      window.addEventListener('load', afterLoad, { once: true });
      this.cleanups.push(() => window.removeEventListener('load', afterLoad));
    }
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    for (const fn of this.cleanups.splice(0).reverse()) {
      try {
        fn();
      } catch {
        /* keep tearing down */
      }
    }
  }

  private motionWelcome(): boolean {
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return false;
    const conn = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection;
    return !conn?.saveData;
  }

  private async init(): Promise<void> {
    this.state.set('loading');
    try {
      const [THREE, g] = await Promise.all([import('./three-subset'), import('./gsap-landing')]);
      if (this.destroyed) return;
      await this.build(THREE, g);
    } catch {
      this.fail();
    }
  }

  private fail(): void {
    if (!this.destroyed) this.state.set('failed');
  }

  private async build(THREE: typeof T, g: typeof import('./gsap-landing')): Promise<void> {
    const { gsap, ScrollTrigger } = g;
    const stageEl = this.stage().nativeElement;
    const canvas = this.canvas().nativeElement;
    const hero = (this.host.nativeElement.closest('section') as HTMLElement | null) ?? stageEl;
    const coarse = window.matchMedia('(pointer: coarse)').matches;

    // Probe on a throwaway canvas first, so a device without WebGL falls back quietly
    // instead of three.js logging an error while it fails to create a context.
    const probe = document.createElement('canvas');
    if (!(probe.getContext('webgl2') ?? probe.getContext('webgl'))) {
      this.fail();
      return;
    }

    const renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: true, powerPreference: 'low-power' });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, coarse ? 1.5 : 2));
    renderer.setClearColor(0x000000, 0);

    const scene = new THREE.Scene();
    const camera = new THREE.PerspectiveCamera(30, 16 / 10, 0.1, 60);
    const deck = new THREE.Group(); // pointer tilt
    const fan = new THREE.Group(); // scroll motion
    deck.add(fan);
    scene.add(deck);

    const loader = new THREE.TextureLoader();
    const load = (url: string) =>
      new Promise<T.Texture>((resolve, reject) =>
        loader.load(
          url,
          (t) => {
            t.colorSpace = THREE.SRGBColorSpace;
            t.anisotropy = Math.min(4, renderer.capabilities.getMaxAnisotropy());
            resolve(t);
          },
          undefined,
          reject,
        ),
      );
    const [qTex, aTex] = await Promise.all([load('landing/slide-question.webp'), load('landing/slide-answer.webp')]);
    if (this.destroyed) {
      qTex.dispose();
      aTex.dispose();
      renderer.dispose();
      return;
    }

    const W = 3.2;
    const H = 1.8;
    const geoms: T.PlaneGeometry[] = [new THREE.PlaneGeometry(W, H), new THREE.PlaneGeometry(W + 0.07, H + 0.07)];
    const mats: T.MeshBasicMaterial[] = [];
    const makeCard = (tex: T.Texture, x: number, y: number, rotY: number) => {
      const card = new THREE.Group();
      const frameMat = new THREE.MeshBasicMaterial({ color: 0xd8d2c4, transparent: true, opacity: 0 });
      const faceMat = new THREE.MeshBasicMaterial({ map: tex, transparent: true, opacity: 0, toneMapped: false });
      mats.push(frameMat, faceMat);
      const frame = new THREE.Mesh(geoms[1], frameMat);
      frame.position.z = -0.01;
      const face = new THREE.Mesh(geoms[0], faceMat);
      card.add(frame, face);
      card.position.set(x, y, -3);
      card.rotation.y = rotY;
      fan.add(card);
      return { card, mats: [frameMat, faceMat] };
    };
    const q = makeCard(qTex, -0.55, 0.3, 0.1);
    const a = makeCard(aTex, 0.55, -0.3, -0.08);

    // Frame both cards inside the stage whatever its shape.
    const fit = () => {
      const w = stageEl.clientWidth || 1;
      const h = stageEl.clientHeight || 1;
      renderer.setSize(w, h, false);
      camera.aspect = w / h;
      const need = 4.9; // scene units to keep in view horizontally
      const halfFovV = (camera.fov * Math.PI) / 360;
      const distH = need / 2 / (Math.tan(halfFovV) * camera.aspect);
      const distV = 2.9 / 2 / Math.tan(halfFovV);
      camera.position.set(0, 0, Math.max(distH, distV));
      camera.updateProjectionMatrix();
    };
    fit();

    let lastSig = NaN;
    let visible = true;
    const sigOf = () =>
      deck.rotation.x * 3 + deck.rotation.y * 5 + fan.rotation.x * 7 + fan.rotation.y * 11 +
      q.card.position.x * 13 + q.card.position.z * 17 + a.card.position.x * 19 + a.card.position.z * 23 +
      q.mats[1].opacity * 29 + a.mats[1].opacity * 31 + camera.position.z * 37 + camera.aspect * 41;
    const tick = () => {
      if (!visible) return;
      const sig = sigOf();
      if (sig === lastSig) return; // nothing moved, nothing to draw
      lastSig = sig;
      renderer.render(scene, camera);
    };
    gsap.ticker.add(tick);

    const ctx = gsap.context(() => {
      // Entrance: the two slides arrive once, back card first.
      gsap.to(q.card.position, { z: -0.5, duration: 1, ease: 'power3.out', delay: 0.05 });
      gsap.to(a.card.position, { z: 0.45, duration: 1, ease: 'power3.out', delay: 0.2 });
      gsap.to(q.mats, { opacity: 1, duration: 0.8, ease: 'power1.out', delay: 0.05 });
      gsap.to(a.mats, { opacity: 1, duration: 0.8, ease: 'power1.out', delay: 0.2 });

      // Scroll: the slides fan apart as the hero scrolls away.
      gsap
        .timeline({ scrollTrigger: { trigger: hero, start: 'top top', end: 'bottom top', scrub: 0.6 } })
        .to(fan.rotation, { y: -0.42, x: 0.1, ease: 'none' }, 0)
        .to(q.card.position, { x: -0.85, ease: 'none' }, 0)
        .to(a.card.position, { x: 0.85, ease: 'none' }, 0);

      // Pointer: a gentle tilt on devices that have a mouse.
      if (!coarse) {
        const toY = gsap.quickTo(deck.rotation, 'y', { duration: 0.9, ease: 'power3.out' });
        const toX = gsap.quickTo(deck.rotation, 'x', { duration: 0.9, ease: 'power3.out' });
        const move = (e: PointerEvent) => {
          const r = hero.getBoundingClientRect();
          const nx = ((e.clientX - r.left) / r.width) * 2 - 1;
          const ny = ((e.clientY - r.top) / r.height) * 2 - 1;
          toY(nx * 0.16);
          toX(ny * 0.1);
        };
        const leave = () => {
          toY(0);
          toX(0);
        };
        hero.addEventListener('pointermove', move, { passive: true });
        hero.addEventListener('pointerleave', leave);
        this.cleanups.push(() => {
          hero.removeEventListener('pointermove', move);
          hero.removeEventListener('pointerleave', leave);
        });
      }
    }, stageEl);

    // Draw only while on screen and while the tab is visible.
    const io = new IntersectionObserver((es) => {
      visible = es.some((e) => e.isIntersecting) && !document.hidden;
      if (visible) lastSig = NaN;
    });
    io.observe(stageEl);
    const onVis = () => {
      visible = !document.hidden;
      if (visible) lastSig = NaN;
    };
    document.addEventListener('visibilitychange', onVis);
    const ro = new ResizeObserver(() => {
      fit();
      lastSig = NaN;
    });
    ro.observe(stageEl);
    const onLost = (e: Event) => {
      e.preventDefault();
      this.state.set('failed');
    };
    canvas.addEventListener('webglcontextlost', onLost);

    this.cleanups.push(() => {
      gsap.ticker.remove(tick);
      ctx.revert();
      ScrollTrigger.getAll().forEach((st) => {
        if (st.trigger === hero) st.kill();
      });
      io.disconnect();
      ro.disconnect();
      document.removeEventListener('visibilitychange', onVis);
      canvas.removeEventListener('webglcontextlost', onLost);
      geoms.forEach((x) => x.dispose());
      mats.forEach((x) => x.dispose());
      qTex.dispose();
      aTex.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
    });

    renderer.render(scene, camera); // first frame before the cross-fade
    this.state.set('ready');
  }
}
