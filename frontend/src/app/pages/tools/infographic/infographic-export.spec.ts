import { TestBed } from '@angular/core/testing';
import { InfographicExporter, dataUrlToBlob, downloadBlob, exportFilename } from './infographic-export.service';
import { InfographicFonts } from './infographic-fonts.service';
import { newDoc, TEMPLATES } from './templates';

class FakeFonts {
  ensured: string[] = [];
  async ensureLoaded(t: string) {
    this.ensured.push(t);
  }
  async embed() {
    return { css: '', uncovered: [0x9f18] };
  }
}

async function dims(blob: Blob): Promise<[number, number]> {
  const bmp = await createImageBitmap(blob);
  const out: [number, number] = [bmp.width, bmp.height];
  bmp.close();
  return out;
}

describe('infographic export', () => {
  it('turns a data URL into a blob of the same type and bytes', async () => {
    const blob = dataUrlToBlob('data:image/png;base64,AQID');
    expect(blob.type).toBe('image/png');
    expect([...new Uint8Array(await blob.arrayBuffer())]).toEqual([1, 2, 3]);
  });

  it('names files from the title and the real size', () => {
    const d = newDoc(TEMPLATES[0], { title: 'My Story!', format: 'landscape' });
    expect(exportFilename(d, 'png')).toBe('my-story-1920x1080.png');
    expect(exportFilename({ ...d, title: '', format: 'square' }, 'jpg')).toBe('infographic-1080x1080.jpg');
  });

  it('downloads through a temporary link that is removed again', () => {
    const clicks: HTMLAnchorElement[] = [];
    const spy = spyOn(HTMLAnchorElement.prototype, 'click').and.callFake(function (this: HTMLAnchorElement) {
      clicks.push(this);
    });
    downloadBlob(new Blob(['x']), 'a.png', document);
    expect(spy).toHaveBeenCalled();
    expect(clicks[0].download).toBe('a.png');
    expect(document.querySelectorAll('a[download="a.png"]').length).toBe(0);
  });

  describe('exporter', () => {
    let exporter: InfographicExporter;
    let fonts: FakeFonts;
    let stage: HTMLElement;

    beforeEach(() => {
      fonts = new FakeFonts();
      TestBed.configureTestingModule({ providers: [{ provide: InfographicFonts, useValue: fonts }] });
      exporter = TestBed.inject(InfographicExporter);
      stage = document.createElement('div');
      stage.textContent = 'Hello';
      // A scaled preview, as on the page: the export must ignore the scale.
      stage.style.cssText = 'width:1080px;height:1080px;transform:scale(0.3);transform-origin:top left;background:#fff;position:fixed;left:0;top:0';
      document.body.appendChild(stage);
    });

    afterEach(() => stage.remove());

    it('makes a PNG at the real size even when the preview is scaled down', async () => {
      const out = await exporter.export(stage, newDoc(TEMPLATES[0], { format: 'square', title: 'T' }), 'png');
      expect(out.blob.type).toBe('image/png');
      expect(await dims(out.blob)).toEqual([1080, 1080]);
      expect(out.width).toBe(1080);
      expect(out.filename).toBe('t-1080x1080.png');
      expect(out.uncovered).toEqual(['\u9f18']);
      expect(fonts.ensured).toEqual(['Hello']);
    }, 30000);

    it('makes a JPG at the real size', async () => {
      stage.style.width = '1920px';
      const out = await exporter.export(stage, newDoc(TEMPLATES[0], { format: 'landscape' }), 'jpg');
      expect(out.blob.type).toBe('image/jpeg');
      expect(await dims(out.blob)).toEqual([1920, 1080]);
    }, 30000);
  });
});
