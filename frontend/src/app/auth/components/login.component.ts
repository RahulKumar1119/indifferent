import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';
import { LucideDynamicIcon } from '@lucide/angular';
import { environment } from '../../../environments/environment';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [RouterLink, LucideDynamicIcon],
  styles: [`
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s, box-shadow .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
    .btn-primary:active { transform: translateY(1px) scale(.98); }
    .grain::before {
      content: ''; position: fixed; inset: 0; z-index: 60; pointer-events: none; opacity: .05;
      background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.9' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='0.5'/%3E%3C/svg%3E");
    }
    @media (prefers-reduced-motion: reduce) {
      .btn-primary { transition: none; }
    }
  `],
  template: `
    <div class="grain sans flex min-h-[100dvh] bg-[#FAF7F2] text-[#1A1714] antialiased">

      <!-- Left panel · dark studio -->
      <div class="hidden lg:flex lg:w-[52%] relative overflow-hidden bg-[#0F0E0B] text-[#F4EFE6] p-12 flex-col justify-between">
        <img src="https://picsum.photos/seed/indifferent-login-studio/1200/1400" alt="" aria-hidden="true"
          class="absolute inset-0 w-full h-full object-cover opacity-30">
        <div class="absolute inset-0" style="background: radial-gradient(ellipse at 80% 10%, rgba(217,108,61,0.22), transparent 55%), linear-gradient(to bottom, rgba(15,14,11,0.55), rgba(15,14,11,0.94));"></div>

        <div class="relative z-10">
          <a routerLink="/" class="flex items-center gap-2">
            <span class="font-semibold tracking-tight text-[17px]">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </a>
        </div>

        <div class="relative z-10">
          <p class="inline-flex items-center gap-2 rounded-full px-3 py-1 text-[10.5px] uppercase tracking-[0.22em] border border-white/15 text-white/70">
            <span class="w-1.5 h-1.5 rounded-full bg-[#D96C3D]"></span> Studio access
          </p>
          <h1 class="serif mt-5 font-medium leading-[0.95] tracking-[-0.02em] text-[clamp(2.6rem,4.5vw,4.2rem)] text-balance">
            Enter the<br><em class="font-light italic">cutting room.</em>
          </h1>
          <p class="mt-5 text-[16px] leading-relaxed text-white/70 max-w-[44ch]">
            Sign in to upload TXT files and render narrated quiz films.
          </p>

          <!-- Mini render preview -->
          <div class="mt-8 rounded-[20px] p-2 bg-white/[0.06] border border-white/10 backdrop-blur-sm max-w-md">
            <div class="rounded-[calc(20px-0.5rem)] overflow-hidden bg-[#17150F] border border-white/10">
              <div class="relative aspect-video">
                <img src="https://picsum.photos/seed/indifferent-login-frame/640/360" alt="Preview of a generated quiz video" class="absolute inset-0 w-full h-full object-cover">
                <div class="absolute inset-0 flex flex-col items-center justify-center p-5 text-center" style="background: linear-gradient(to bottom, rgba(15,14,11,0.25), rgba(15,14,11,0.72));">
                  <p class="text-[10px] uppercase tracking-[0.25em] text-white/60">Question 12 / 20</p>
                  <p class="serif text-xl mt-1">Which planet is the Red Planet?</p>
                  <div class="grid grid-cols-2 gap-1.5 mt-4 w-full max-w-[280px] text-[11.5px]">
                    <span class="rounded-md bg-white/10 border border-white/20 px-2 py-1.5">Venus</span>
                    <span class="rounded-md bg-[#D96C3D] px-2 py-1.5 font-semibold">Mars</span>
                    <span class="rounded-md bg-white/10 border border-white/20 px-2 py-1.5">Jupiter</span>
                    <span class="rounded-md bg-white/10 border border-white/20 px-2 py-1.5">Saturn</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <figure class="mt-6 max-w-md rounded-[20px] border border-white/10 bg-white/[0.05] backdrop-blur p-5">
            <blockquote class="serif text-[19px] leading-snug">“It stopped looking like slides. It plays like a real quiz show.”</blockquote>
            <figcaption class="mt-3 flex items-center gap-3">
              <span class="w-9 h-9 rounded-full bg-[#BC5227] text-white flex items-center justify-center text-[12px] font-bold">PR</span>
              <span><span class="block text-[13.5px] font-semibold">Priya Rao</span><span class="block text-[12px] text-white/55">Education creator</span></span>
            </figcaption>
          </figure>
        </div>

        <div class="relative z-10">
          <p class="text-white/40 text-[12.5px]">© {{ currentYear }} Indifferent · Architecture of video</p>
        </div>
      </div>

      <!-- Right panel · sign in -->
      <div class="flex-1 flex items-center justify-center px-5 py-14 md:p-12">
        <div class="w-full max-w-[440px]">
          <div class="lg:hidden flex items-center gap-2 justify-center mb-8">
            <span class="text-lg font-bold">Indifferent<span class="text-[#D96C3D]">.</span></span>
          </div>

          <div class="rounded-[20px] p-2 bg-black/[0.04] border border-black/10">
            <div class="rounded-[calc(20px-0.5rem)] bg-white border border-black/[0.06] p-8 md:p-10 shadow-[inset_0_1px_0_rgba(255,255,255,0.6)]">
              <p class="text-[11.5px] uppercase tracking-[0.2em] text-[#6B6560]">Welcome back</p>
              <h2 class="serif mt-2 font-medium text-[clamp(2rem,4vw,2.8rem)] leading-[1.0]">Sign in to<br>your studio.</h2>
              <p class="mt-3 text-[14.5px] text-[#6B6560] leading-relaxed">One click with Google. No passwords, no credit card.</p>

              <button (click)="signInWithGoogle()"
                class="btn-primary mt-7 w-full flex items-center justify-center gap-3 px-6 py-4 rounded-full bg-[#1A1714] text-white font-semibold text-[15px] hover:bg-[#2A2620]">
                <svg class="w-5 h-5 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
                  <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                  <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
                  <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
                </svg>
                Continue with Google
              </button>

              <div class="relative my-6">
                <div class="absolute inset-0 flex items-center"><div class="w-full border-t border-black/10"></div></div>
                <div class="relative flex justify-center text-[11.5px] uppercase tracking-[0.14em]">
                  <span class="bg-white px-4 text-[#6B6560]">Secure OAuth · Google</span>
                </div>
              </div>

              <ul class="grid grid-cols-2 gap-x-4 gap-y-3 text-[13.5px] text-[#6B6560]">
                <li class="flex items-center gap-2"><svg lucideIcon="check" [size]="15" class="text-[#1E3A2A]"></svg>Free to use</li>
                <li class="flex items-center gap-2"><svg lucideIcon="check" [size]="15" class="text-[#1E3A2A]"></svg>No credit card</li>
                <li class="flex items-center gap-2"><svg lucideIcon="check" [size]="15" class="text-[#1E3A2A]"></svg>5 AI voices</li>
                <li class="flex items-center gap-2"><svg lucideIcon="check" [size]="15" class="text-[#1E3A2A]"></svg>HD 1080p MP4</li>
              </ul>
            </div>
          </div>

          <p class="mt-6 text-center text-[12.5px] text-[#6B6560]">
            By continuing, you agree to our
            <a routerLink="/terms" class="underline underline-offset-4 decoration-black/25 hover:decoration-black">Terms of Service</a>
            and
            <a routerLink="/privacy" class="underline underline-offset-4 decoration-black/25 hover:decoration-black">Privacy Policy</a>
          </p>
          <div class="mt-4 text-center">
            <a routerLink="/" class="text-[13.5px] text-[#6B6560] hover:text-[#1A1714] transition-colors">← Back to home</a>
          </div>
        </div>
      </div>
    </div>
  `,
})
export class LoginComponent {
  currentYear = new Date().getFullYear();

  signInWithGoogle(): void {
    const params = new URLSearchParams({
      client_id: environment.googleClientId,
      redirect_uri: environment.googleRedirectUri,
      response_type: 'code',
      scope: 'openid email profile',
    });

    window.location.href = `https://accounts.google.com/o/oauth2/v2/auth?${params.toString()}`;
  }
}
