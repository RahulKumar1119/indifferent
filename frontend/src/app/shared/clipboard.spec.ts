import { copyText } from './clipboard';

describe('copyText', () => {
  afterEach(() => document.querySelectorAll('textarea[readonly]').forEach((n) => n.remove()));

  it('uses the Clipboard API when the browser allows it', async () => {
    const write = spyOn(navigator.clipboard, 'writeText').and.returnValue(Promise.resolve());
    const exec = spyOn(document, 'execCommand');
    expect(await copyText('hello')).toBeTrue();
    expect(write).toHaveBeenCalledOnceWith('hello');
    expect(exec).not.toHaveBeenCalled();
  });

  it('falls back to a hidden text box when the Clipboard API refuses, and cleans it up', async () => {
    spyOn(navigator.clipboard, 'writeText').and.returnValue(Promise.reject(new DOMException('no', 'NotAllowedError')));
    let copied = '';
    spyOn(document, 'execCommand').and.callFake(() => {
      copied = (document.querySelector('textarea[readonly]') as HTMLTextAreaElement).value;
      return true;
    });
    expect(await copyText('two\nlines')).toBeTrue();
    expect(copied).toBe('two\nlines');
    expect(document.querySelector('textarea[readonly]')).toBeNull();
  });

  it('says false, and does not throw, when nothing works', async () => {
    spyOn(navigator.clipboard, 'writeText').and.returnValue(Promise.reject(new Error('no')));
    spyOn(document, 'execCommand').and.returnValue(false);
    expect(await copyText('x')).toBeFalse();
    spyOn(document, 'createElement').and.throwError('boom');
    expect(await copyText('x')).toBeFalse();
  });
});
