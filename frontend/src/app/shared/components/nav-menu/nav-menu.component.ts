import { Component, ElementRef, HostListener, inject, input, signal, viewChild } from '@angular/core';

let nextId = 0;

/**
 * Header dropdown that opens by hover (only on devices that can hover),
 * tap, or keyboard (Enter/Space on the button, Escape to close).
 * Links are projected, so each page keeps its own copy.
 */
@Component({
  selector: 'app-nav-menu',
  standalone: true,
  template: `
    <div
      class="relative"
      (keydown.escape)="closeAndFocus()"
      (focusout)="onFocusOut($event)"
      (mouseenter)="hoverOpen()"
      (mouseleave)="hoverClose()"
    >
      <button
        #trigger
        type="button"
        aria-haspopup="true"
        [attr.aria-expanded]="open()"
        [attr.aria-controls]="panelId"
        (click)="toggle()"
        class="flex items-center gap-1.5 bg-transparent border-none text-[13.5px] transition-colors cursor-pointer p-0 min-h-[44px]"
        [class]="open() ? 'text-white' : 'text-white/70 hover:text-white'"
      >
        {{ label() }}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="12"
          height="12"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          aria-hidden="true"
          class="transition-transform duration-200 motion-reduce:transition-none"
          [class.rotate-180]="open()"
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      <div
        [id]="panelId"
        class="absolute top-full left-1/2 -translate-x-1/2 pt-3 z-10 transition-all duration-200 motion-reduce:transition-none"
        [class]="open() ? 'opacity-100 visible translate-y-0' : 'opacity-0 invisible translate-y-1'"
        (click)="onPanelClick($event)"
      >
        <div
          class="rounded-2xl border border-white/10 bg-[#17150F] shadow-[0_20px_60px_rgba(0,0,0,0.5)] p-2"
          [style.width.px]="width()"
        >
          <ng-content></ng-content>
        </div>
      </div>
    </div>
  `,
})
export class NavMenuComponent {
  readonly label = input.required<string>();
  readonly width = input(270);

  protected readonly open = signal(false);
  protected readonly panelId = `nav-menu-panel-${nextId++}`;

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly trigger = viewChild.required<ElementRef<HTMLButtonElement>>('trigger');

  // True once opened by click or keyboard, so moving the pointer away keeps it open.
  private pinned = false;

  // On touch devices a tap fires mouseenter and then click, which would open and
  // immediately close the menu, so hover is only honoured where it really exists.
  private canHover(): boolean {
    return typeof window !== 'undefined' && !!window.matchMedia?.('(hover: hover)').matches;
  }

  private close(): void {
    this.open.set(false);
    this.pinned = false;
  }

  protected toggle(): void {
    // A click on a menu that hover just opened pins it instead of closing it.
    if (this.open() && !this.pinned) {
      this.pinned = true;
      return;
    }
    const next = !this.open();
    this.open.set(next);
    this.pinned = next;
  }

  protected hoverOpen(): void {
    if (this.canHover() && !this.open()) {
      this.open.set(true);
      this.pinned = false;
    }
  }

  protected hoverClose(): void {
    if (this.canHover() && !this.pinned) this.open.set(false);
  }

  protected closeAndFocus(): void {
    if (!this.open()) return;
    this.close();
    this.trigger().nativeElement.focus();
  }

  protected onFocusOut(e: FocusEvent): void {
    if (!this.host.nativeElement.contains(e.relatedTarget as Node | null)) this.close();
  }

  protected onPanelClick(e: Event): void {
    if ((e.target as HTMLElement).closest('a')) this.close();
  }

  @HostListener('document:click', ['$event'])
  protected onDocumentClick(e: Event): void {
    if (!this.host.nativeElement.contains(e.target as Node)) this.close();
  }
}
