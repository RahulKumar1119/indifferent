import { provideRouter } from '@angular/router';
import { TestBed } from '@angular/core/testing';
import { AboutComponent } from './about.component';

describe('AboutComponent bug-condition exploration', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AboutComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  it('renders every affected informational group as semantic text on /about', () => {
    const fixture = TestBed.createComponent(AboutComponent);
    fixture.detectChanges();
    const page = fixture.nativeElement as HTMLElement;

    const section = (heading: string): HTMLElement => {
      const headingElement = Array.from(page.querySelectorAll('h2')).find(
        (element) => element.textContent?.trim() === heading,
      );
      const container = headingElement?.parentElement?.parentElement;
      expect(container).withContext(`Missing ${heading} section`).toBeTruthy();
      return container as HTMLElement;
    };

    const howItWorks = section('How It Works');
    expect(howItWorks.querySelectorAll('h3').length).toBe(4);
    [
      'Upload Your Quiz File',
      'Choose Template & Voice',
      'Automatic Processing',
      'Download & Share',
      'Upload a plain text (.txt) file with your multiple-choice questions. We support numbered, bulleted, and tab-indented formats.',
      'Select a visual template for your slides and pick from 5 professional AI voices powered by Amazon Polly.',
      'Our serverless pipeline parses your questions, generates animated slides, creates narration, and renders the final video.',
      'Preview your video in-browser and download the MP4 file ready for YouTube, social media, or any platform.',
    ].forEach((text) => expect(howItWorks.textContent).toContain(text));

    const supportedFormats = section('Supported Formats');
    expect(supportedFormats.querySelectorAll('span').length).toBe(5);
    [
      'Numbered questions (1. Question text)',
      'Bulleted questions (• or - Question text)',
      'Tab-indented answers',
      'Multiple correct answers supported',
      'Maximum file size: 5MB',
    ].forEach((text) => expect(supportedFormats.textContent).toContain(text));

    const videoTemplates = section('Video Templates');
    expect(videoTemplates.querySelectorAll('div.grid-cols-1 > div.about-card').length).toBe(6);
    [
      'Classic',
      'Modern',
      'Education',
      'Dark',
      'Minimal',
      'Neon',
      'Clean blue theme, professional look',
      'Gradient backgrounds, contemporary design',
      'Warm colors, classroom-friendly',
      'Dark mode, high contrast for readability',
      'Simple white, distraction-free',
      'Vibrant colors, energetic style',
    ].forEach((text) => expect(videoTemplates.textContent).toContain(text));

    const aiVoices = section('AI Voices');
    expect(aiVoices.querySelectorAll('p.font-medium').length).toBe(5);
    [
      'Joanna',
      'Matthew',
      'Amy',
      'Brian',
      'Aditi',
      'US English, female, clear and professional',
      'US English, male, warm and authoritative',
      'British English, female, polished',
      'British English, male, natural',
      'Indian English, female, approachable',
    ].forEach((text) => expect(aiVoices.textContent).toContain(text));

    const videoSpecifications = section('Video Specifications');
    expect(videoSpecifications.querySelectorAll('span').length).toBe(5);
    [
      'Resolution: 1920×1080 (Full HD)',
      'Format: MP4 (H.264)',
      'Audio: AAC',
      'Includes: Question slides, answer reveals, AI narration',
      'Automatic thumbnail generation',
    ].forEach((text) => expect(videoSpecifications.textContent).toContain(text));

    const useCases = section('Use Cases');
    expect(useCases.querySelectorAll('p.font-medium').length).toBe(5);
    [
      'Teachers & Educators',
      'YouTube Creators',
      'E-learning Platforms',
      'Corporate Trainers',
      'Students',
      'Create revision videos for students',
      'Scale quiz/trivia content production',
      'Automate video course creation',
      'Turn compliance quizzes into video assessments',
      'Make study materials more engaging',
    ].forEach((text) => expect(useCases.textContent).toContain(text));

    const builtWith = section('Built With');
    expect(builtWith.querySelectorAll('p.font-medium').length).toBe(9);
    [
      'Angular 20',
      'Go',
      'AWS Lambda',
      'Amazon Polly',
      'FFmpeg',
      'DynamoDB',
      'Amplify Hosting',
      'Step Functions',
      'GSAP',
      'Frontend',
      'Backend',
      'Compute',
      'Narration',
      'Video',
      'Database',
      'CDN',
      'Orchestration',
      'Animations',
    ].forEach((text) => expect(builtWith.textContent).toContain(text));
  });
});


describe('AboutComponent preservation baseline', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AboutComponent],
      providers: [provideRouter([])],
    }).compileComponents();
  });

  it('preserves navigation destinations and external GitHub link behavior', () => {
    const fixture = TestBed.createComponent(AboutComponent);
    fixture.detectChanges();
    const page = fixture.nativeElement as HTMLElement;
    const links = Array.from(page.querySelectorAll('a'));

    const logo = links.find((link) => link.querySelector('img[alt="Indifferent"]'));
    expect(logo?.getAttribute('href')).toBe('/');
    expect(logo?.querySelector('img')?.getAttribute('alt')).toBe('Indifferent');

    const backToHome = links.find((link) => link.textContent?.includes('Back to Home'));
    expect(backToHome?.getAttribute('href')).toBe('/');

    const signIn = links.find((link) => link.textContent?.trim() === 'Sign In');
    expect(signIn?.getAttribute('href')).toBe('/login');

    const github = links.find((link) => link.textContent?.includes('Open an Issue on GitHub'));
    expect(github?.getAttribute('href')).toBe('https://github.com/RahulKumar1119/indifferent/issues');
    expect(github?.getAttribute('target')).toBe('_blank');
    expect(github?.getAttribute('rel')?.split(/\s+/)).toContain('noopener');
  });

  it('preserves page copy, section order, and heading structure', () => {
    const fixture = TestBed.createComponent(AboutComponent);
    fixture.detectChanges();
    const page = fixture.nativeElement as HTMLElement;
    const section = (heading: string): HTMLElement => {
      const headingElement = Array.from(page.querySelectorAll('h2')).find(
        (element) => element.textContent?.trim() === heading,
      );
      const container = headingElement?.parentElement?.parentElement;
      expect(container).withContext(`Missing ${heading} section`).toBeTruthy();
      return container as HTMLElement;
    };

    expect(page.querySelectorAll('h1').length).toBe(1);
    expect(page.querySelector('h1')?.textContent?.trim()).toBe('About Indifferent');
    expect(Array.from(page.querySelectorAll('h2')).map((heading) => heading.textContent?.trim())).toEqual([
      'Our Mission',
      'How It Works',
      'Supported Formats',
      'Video Templates',
      'AI Voices',
      'Video Specifications',
      'Use Cases',
      'Built With',
      'Creator',
      'Get In Touch',
    ]);
    expect(page.querySelectorAll('h3').length).toBe(4);

    expect(section('Our Mission').textContent).toContain(
      'Indifferent makes it effortless to convert your multiple-choice quiz files into engaging, YouTube-ready video content.',
    );
    expect(section('Creator').textContent).toContain(
      'Indifferent is built and maintained by a passionate developer focused on making content creation accessible to everyone.',
    );
    expect(section('Get In Touch').textContent).toContain(
      "Have questions, feedback, or feature requests? We'd love to hear from you.",
    );
  });

  it('keeps page links named and keyboard reachable without relying on decorative icons', () => {
    const fixture = TestBed.createComponent(AboutComponent);
    fixture.detectChanges();
    const page = fixture.nativeElement as HTMLElement;
    const links = Array.from(page.querySelectorAll('a'));

    links.forEach((link) => {
      const accessibleName = link.textContent?.trim() || link.querySelector('img')?.getAttribute('alt');
      expect(accessibleName).withContext(`Unnamed link: ${link.outerHTML}`).toBeTruthy();
      expect(link.tabIndex).not.toBe(-1);
    });
    expect(page.querySelector('img[alt="Indifferent"]')).toBeTruthy();
  });
});
