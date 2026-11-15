import { TestBed } from '@angular/core/testing';
import { RUM_FACTORY, RumHandle, RumService } from './rum.service';

function setup() {
  const calls: string[] = [];
  const handle: RumHandle = {
    enable: () => calls.push('enable'),
    disable: () => calls.push('disable'),
    allowCookies: (a) => calls.push(`cookies:${a}`),
  };
  const factory = jasmine.createSpy('factory').and.callFake(() => Promise.resolve(handle));
  TestBed.configureTestingModule({ providers: [{ provide: RUM_FACTORY, useValue: factory }] });
  return { svc: TestBed.inject(RumService), factory, calls };
}
const settle = () => new Promise<void>((r) => setTimeout(r));

describe('RumService', () => {
  it('does not create the client without Analytics', async () => {
    const { svc, factory } = setup();
    svc.apply(false);
    await settle();
    expect(factory).not.toHaveBeenCalled();
  });

  it('creates it once after Analytics', async () => {
    const { svc, factory } = setup();
    svc.apply(true);
    svc.apply(true);
    await settle();
    expect(factory).toHaveBeenCalledTimes(1);
  });

  it('disables it and stops cookies on withdrawal, and re-enables on a new allow', async () => {
    const { svc, calls, factory } = setup();
    svc.apply(true);
    await settle();
    svc.apply(false);
    expect(calls).toEqual(['disable', 'cookies:false']);
    svc.apply(true);
    expect(calls.slice(2)).toEqual(['enable', 'cookies:true']);
    expect(factory).toHaveBeenCalledTimes(1);
  });

  it('switches it off at once when the choice was withdrawn while the library loaded', async () => {
    const { svc, calls } = setup();
    svc.apply(true);
    svc.apply(false);
    await settle();
    expect(calls).toEqual(['disable', 'cookies:false']);
  });

  it('survives a failing load and can try again', async () => {
    const factory = jasmine.createSpy('factory').and.returnValue(Promise.reject(new Error('offline')));
    TestBed.configureTestingModule({ providers: [{ provide: RUM_FACTORY, useValue: factory }] });
    const svc = TestBed.inject(RumService);
    svc.apply(true);
    await settle();
    svc.apply(true);
    await settle();
    expect(factory).toHaveBeenCalledTimes(2);
  });
});
