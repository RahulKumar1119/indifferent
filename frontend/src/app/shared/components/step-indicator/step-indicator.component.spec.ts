import { TestBed } from '@angular/core/testing';
import { provideLucideIcons, LucideCheck } from '@lucide/angular';
import { StepIndicatorComponent } from './step-indicator.component';

describe('StepIndicatorComponent (#251)', () => {
  function render(current: number) {
    TestBed.configureTestingModule({ imports: [StepIndicatorComponent], providers: [provideLucideIcons(LucideCheck)] });
    const fixture = TestBed.createComponent(StepIndicatorComponent);
    fixture.componentRef.setInput('steps', ['Details', 'Questions']);
    fixture.componentRef.setInput('current', current);
    fixture.componentRef.setInput('label', 'Quiz video steps');
    fixture.detectChanges();
    return fixture.nativeElement as HTMLElement;
  }

  it('marks only the current step with aria-current="step"', () => {
    const el = render(0);
    expect(el.querySelector('ol')!.getAttribute('aria-label')).toBe('Quiz video steps');
    expect(el.querySelector('[data-testid="step-1"]')!.getAttribute('aria-current')).toBe('step');
    expect(el.querySelector('[data-testid="step-2"]')!.hasAttribute('aria-current')).toBeFalse();
    expect(el.querySelectorAll('[aria-current]').length).toBe(1);
  });

  it('reads each step with its position, and says an earlier step is done', () => {
    const el = render(1);
    const first = el.querySelector('[data-testid="step-1"]')!.textContent!.replace(/\s+/g, ' ');
    const second = el.querySelector('[data-testid="step-2"]')!;
    expect(first).toContain('Step 1 of 2: Details');
    expect(first).toContain('(done)');
    expect(second.getAttribute('aria-current')).toBe('step');
    expect(second.textContent).not.toContain('(done)');
  });
});
