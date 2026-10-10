import { Component, ElementRef, HostListener, inject, signal, viewChild } from '@angular/core';

let nextId = 0;

/**
 * Labeled "Menu" button for screens below md, where the desktop links are
 * hidden. The panel opens under the header and holds the projected links.
 */
@Component({
  selector: 'app-nav-mobile-menu',
  standalone: true,
  template: `
    <div class="md:hidden" (keydown.escape)="closeAndFocus()" (focusout)="onFocusOut($event)">
      <button
        #trigger
        type="button"
        [attr.aria-expanded]="open()"
        [attr.aria-controls]="panelId"
        (click)="open.set(!open())"
        class="inline-flex items-center gap-2 h-11 px-3 rounded-full text-[13.5px] text-white/80 hover:text-white transition-colors cursor-pointer bg-transparent border-none"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="18"
          height="18"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
        >
          @if (open()) {
            <path d="M18 6 6 18M6 6l12 12" />
          } @else {
            <path d="M4 7h16M4 12h16M4 17h16" />
          }
        </svg>
        {{ open() ? 'Close' : 'Menu' }}
      </button>
      @if (open()) {
        <div
          [id]="panelId"
          class="absolute left-0 right-0 top-full mt-2 max-h-[calc(100dvh-6.5rem)] overflow-y-auto overscroll-contain rounded-2xl border border-white/10 bg-[#17150F] p-2 shadow-[0_20px_60px_rgba(0,0,0,0.5)]"
          (click)="onPanelClick($event)"
        >
          <ng-content></ng-content>
        </div>
      }
    </div>
  `,
})
export class NavMobileMenuComponent {
  protected readonly open = signal(false);
  protected readonly panelId = `nav-mobile-panel-${nextId++}`;

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly trigger = viewChild.required<ElementRef<HTMLButtonElement>>('trigger');

  protected closeAndFocus(): void {
    if (!this.open()) return;
    this.open.set(false);
    this.trigger().nativeElement.focus();
  }

  protected onFocusOut(e: FocusEvent): void {
    if (!this.host.nativeElement.contains(e.relatedTarget as Node | null)) this.open.set(false);
  }

  protected onPanelClick(e: Event): void {
    if ((e.target as HTMLElement).closest('a')) this.open.set(false);
  }

  @HostListener('document:click', ['$event'])
  protected onDocumentClick(e: Event): void {
    if (!this.host.nativeElement.contains(e.target as Node)) this.open.set(false);
  }
}
