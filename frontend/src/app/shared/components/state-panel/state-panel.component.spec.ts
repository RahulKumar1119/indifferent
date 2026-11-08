import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { StatePanelComponent } from './state-panel.component';

@Component({
  standalone: true,
  imports: [StatePanelComponent],
  template: `<app-state-panel [kind]="kind" title="Nothing here" message="Add one" actionLabel="Add" (action)="clicks = clicks + 1" />`,
})
class HostComponent {
  kind: 'loading' | 'empty' | 'error' = 'empty';
  clicks = 0;
}

describe('StatePanelComponent', () => {
  function render(kind: 'loading' | 'empty' | 'error') {
    TestBed.configureTestingModule({ imports: [HostComponent] });
    const f = TestBed.createComponent(HostComponent);
    f.componentInstance.kind = kind;
    f.detectChanges();
    return { f, el: f.nativeElement as HTMLElement };
  }

  it('empty shows what belongs here and one action that fires', () => {
    const { f, el } = render('empty');
    expect(el.textContent).toContain('Nothing here');
    expect(el.textContent).toContain('Add one');
    el.querySelector('button')!.click();
    expect(f.componentInstance.clicks).toBe(1);
    expect(el.querySelector('[role=alert]')).toBeNull();
  });

  it('error is announced as an alert and carries the retry action', () => {
    const { f, el } = render('error');
    expect(el.querySelector('[role=alert]')?.textContent).toContain('Nothing here');
    el.querySelector('button')!.click();
    expect(f.componentInstance.clicks).toBe(1);
  });

  it('loading is a busy status with skeleton bars and no button or text of its own', () => {
    const { el } = render('loading');
    const status = el.querySelector('[role=status]')!;
    expect(status.getAttribute('aria-busy')).toBe('true');
    expect(el.querySelectorAll('.ui-skeleton').length).toBe(3);
    expect(el.querySelector('button')).toBeNull();
  });
});
