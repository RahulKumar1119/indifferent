import { Component, OnInit, inject } from '@angular/core';
import { CommonModule, DOCUMENT } from '@angular/common';
import { Meta, Title } from '@angular/platform-browser';
import { RouterLink } from '@angular/router';
import { setCanonical } from '../../../shared/seo';

@Component({
  selector: 'app-quiz-file-format-guide',
  standalone: true,
  imports: [CommonModule, RouterLink],
  styles: [`
    .sans { font-family: 'Outfit', 'Inter', system-ui, sans-serif; }
    .serif { font-family: 'Cormorant Garamond', 'Playfair Display', Georgia, serif; }
    .article-body { color: #3E3A33; font-size: 1.0625rem; line-height: 1.8; }
    .article-body h2 { font-size: 1.5rem; font-weight: 700; color: #1A1714; margin: 2.5rem 0 1rem; }
    .article-body h3 { font-size: 1.25rem; font-weight: 600; color: #1A1714; margin: 2rem 0 0.75rem; }
    .article-body p { margin: 0 0 1.25rem; }
    .article-body ul, .article-body ol { margin: 0 0 1.25rem; padding-left: 1.5rem; }
    .article-body li { margin-bottom: 0.5rem; }
    .article-body code { background: rgba(26,23,20,0.06); padding: 0.2rem 0.4rem; border-radius: 0.25rem; font-size: 0.875rem; }
    .article-body pre { background: #0F0E0B; color: #EDE8DB; padding: 1.25rem; border-radius: 12px; overflow-x: auto; margin: 0 0 1.25rem; font-size: 0.875rem; line-height: 1.6; }
    .article-body strong { color: #1A1714; font-weight: 600; }
    .btn-primary { transition: transform .2s cubic-bezier(0.23,1,0.32,1), background-color .2s; }
    .btn-primary:hover { transform: translateY(-1px); }
  `],
  template: `
    <div class="sans bg-[#FAF7F2] text-[#1A1714] antialiased min-h-[100dvh]">
      <div class="max-w-3xl mx-auto px-4 pt-20 pb-6">
        <a routerLink="/blog" class="text-[13px] text-[#6B6560] hover:text-[#1A1714]">&larr; Back to Blog</a>
        <p class="mt-6 text-[11.5px] uppercase tracking-[0.22em] text-[#6B6560]">Guide · Quiz format</p>
        <h1 class="serif mt-3 font-medium tracking-[-0.02em] leading-[1.05] text-[clamp(2rem,5vw,3rem)]">Quiz File Format Guide: TXT Formatting Tips</h1>
        <p class="mt-3 text-[13px] text-[#6B6560]">Published January 5, 2025 · 6 min read</p>
      </div>
      <main class="max-w-3xl mx-auto px-4 pb-10">
        <div class="article-body">
        <p>Getting your quiz file formatted correctly is the first step toward creating professional quiz videos with Indifferent. The platform supports multiple text formats and automatically detects the structure of your questions, but understanding the supported patterns will help you create files that parse perfectly every time. This guide covers everything you need to know about formatting quiz text files for optimal results.</p>

        <h2>Supported Formats</h2>
        <p>Indifferent's parser recognizes three primary formatting styles for quiz questions. You can use any of these formats, and the system will automatically detect which one you are using. For best results, stick with one format consistently throughout your entire file rather than mixing styles.</p>

        <h3>Numbered Format</h3>
        <p>The numbered format is the most common and straightforward approach. Each question begins with a number followed by a period or parenthesis, and answer options are listed below with letter prefixes. Here is how it looks:</p>
        <pre>1. What is the capital of France?
a) London
b) Paris
c) Berlin
d) Madrid
Answer: b</pre>
        <p>The parser recognizes numbers followed by periods, closing parentheses, or colons as question indicators. Answer options can use lowercase or uppercase letters followed by periods, parentheses, or colons. The correct answer line should begin with "Answer:" followed by the letter of the correct option.</p>

        <h3>Bulleted Format</h3>
        <p>The bulleted format uses dash or asterisk characters to denote answer options. This style works well when you are converting content from note-taking applications or markdown documents:</p>
        <pre>What is the largest planet in our solar system?
- Mercury
- Jupiter
- Saturn
- Neptune
Answer: Jupiter</pre>
        <p>In this format, questions are identified as lines that end with a question mark and are followed by bulleted options. The answer line references the full text of the correct option rather than a letter prefix.</p>

        <h3>Tab-Indented Format</h3>
        <p>The tab-indented format uses indentation to distinguish between questions and their answer options. This format is particularly useful when exporting from spreadsheet applications or when you prefer a clean visual hierarchy:</p>
        <pre>Which element has the chemical symbol Au?
	Silver
	Gold
	Copper
	Iron
Answer: Gold</pre>
        <p>Each answer option is indented with a tab character, clearly separating it from the question text above. The parser treats any indented lines following a non-indented line as answer options for that question.</p>

        <h2>Common Formatting Mistakes</h2>
        <p>Understanding frequent errors will save you time and frustration when preparing your quiz files. Here are the most common issues and their solutions:</p>
        <p><strong>Inconsistent numbering:</strong> If you start with numbered questions, maintain that format throughout. Switching between numbered and bulleted formats mid-file can confuse the parser and produce unexpected results.</p>
        <p><strong>Missing answer indicators:</strong> Every question must have a clearly marked correct answer. Without the "Answer:" line, the parser cannot determine which option is correct, and the generated video will not include proper answer reveals.</p>
        <p><strong>Extra blank lines:</strong> While one blank line between questions is fine and can improve readability, multiple consecutive blank lines may cause the parser to treat separate parts of the same question as different entries. Keep spacing consistent with single blank lines between question blocks.</p>
        <p><strong>Special characters in questions:</strong> Avoid using characters that might be interpreted as formatting markers. Specifically, do not start questions with dashes, asterisks, or tab characters unless you intend them to be parsed as answer options.</p>
        <p><strong>Inconsistent answer format:</strong> If you use letter-based answers (a, b, c, d) for some questions but full-text answers for others, the parser may fail to match answers correctly. Choose one answer style and use it throughout your file.</p>

        <h2>Writing Effective Multiple-Choice Questions</h2>
        <p>Beyond formatting, the quality of your questions determines how engaging your video will be. Well-crafted questions keep viewers interested and encourage them to think critically about each answer option.</p>
        <p>Write clear, unambiguous question stems that test a single concept. Avoid double negatives or overly complex sentence structures that confuse rather than challenge. The question should be understandable on first reading without requiring viewers to re-read multiple times.</p>
        <p>Create plausible distractors: wrong answers that are reasonable enough to require genuine thought. Obvious filler options reduce the challenge and make your quiz feel less professional. Each wrong answer should represent a common misconception or a related-but-incorrect fact about the topic.</p>
        <p>Keep all answer options roughly the same length. When the correct answer is consistently longer or shorter than other options, viewers learn to identify patterns rather than actually knowing the content. This undermines the educational value of your quiz.</p>

        <h2>Handling Special Characters</h2>
        <p>Most standard text characters work perfectly in quiz files. However, certain characters require attention:</p>
        <ul>
          <li>Quotation marks (single and double) are fully supported and can appear in both questions and answers</li>
          <li>Mathematical symbols like plus, minus, equals, and percentage signs work without issues</li>
          <li>Ampersands and angle brackets are safe to use in question text</li>
          <li>Currency symbols (dollar, euro, pound) are supported</li>
          <li>Accented characters and non-Latin scripts are supported as the parser uses UTF-8 encoding</li>
        </ul>
        <p>Characters to avoid at the start of answer lines include hash symbols, which may be interpreted as markdown headers, and greater-than signs, which some systems interpret as quotation markers.</p>

        <h2>File Size and Question Limits</h2>
        <p>Indifferent accepts text files up to five megabytes in size, which is sufficient for hundreds of questions. For optimal video length and viewer engagement, we recommend the following guidelines:</p>
        <ul>
          <li>Minimum: 5 questions (produces a video of approximately two minutes)</li>
          <li>Recommended: 15 to 25 questions (produces videos of eight to fifteen minutes, ideal for YouTube)</li>
          <li>Maximum: 100 questions per file (longer quizzes should be split into multiple videos)</li>
        </ul>
        <p>Each question should have between three and five answer options. Four options is the sweet spot: enough to provide genuine challenge without overwhelming viewers with too many choices to read within the countdown timer.</p>

        <h2>Best Practices for Answer Options</h2>
        <p>The standard and most effective configuration is four answer options with exactly one correct answer. This format provides enough variety to be challenging while remaining manageable for viewers to read and evaluate within the time allowed by the countdown timer.</p>
        <p>Three options work well for simpler topics or content aimed at younger audiences. They reduce cognitive load and allow for faster pacing. Five options are appropriate for advanced or professional-level content where more nuanced distinctions between answers are meaningful.</p>
        <p>Always place the correct answer in a random position across your questions. If the correct answer is consistently option B or always the longest choice, observant viewers will notice the pattern and game the system rather than actually testing their knowledge.</p>

        <h2>How the Parser Handles Edge Cases</h2>
        <p>The Indifferent parser is designed to be forgiving and handle common variations gracefully. If the parser encounters a line it cannot categorize, it attempts to associate it with the nearest question context. Trailing whitespace is automatically trimmed, and various line ending formats (Windows, Unix, Mac) are all supported.</p>
        <p>When the parser detects a question with fewer than two answer options, it flags it as potentially incomplete and will notify you during the upload process. Questions with no marked correct answer are similarly flagged, giving you the opportunity to fix issues before video generation begins.</p>
        <p>The parser also handles UTF-8 byte order marks that some text editors add automatically, ensuring compatibility across different operating systems and text editing tools. Whether you create your file in Notepad, TextEdit, VS Code, or any other editor, the output will parse correctly as long as you follow the formatting guidelines described above.</p>
      </div>
        <div class="mt-10 rounded-[20px] bg-[#1A1714] text-white px-6 py-8 text-center">
          <p class="font-medium text-[19px]">Ready to create your first quiz video?</p>
          <p class="mt-2 text-[14px] text-white/60">Sign up free and convert your text quizzes into professional videos in minutes.</p>
          <a routerLink="/login" class="btn-primary mt-4 inline-block px-7 py-3 rounded-full bg-[#D96C3D] hover:bg-[#BC5227] transition-colors text-white font-semibold text-[14.5px]">Get Started Free</a>
          <p class="mt-3 text-[13px]"><a routerLink="/features/txt-to-video-quiz" class="underline underline-offset-4 text-white/70 hover:text-white">How the quiz generator works →</a></p>
        </div>
      </main>
      <footer class="border-t border-black/10 px-4 py-8">
        <p class="text-center text-[12.5px] text-[#6B6560]">© {{ currentYear }} Indifferent · <a routerLink="/blog" class="underline underline-offset-4">Blog</a></p>
      </footer>
    </div>
  `,
})
export class QuizFileFormatGuideComponent implements OnInit {
  private readonly title = inject(Title);
  private readonly meta = inject(Meta);
  private readonly document = inject(DOCUMENT);

  currentYear = new Date().getFullYear();

  ngOnInit(): void {
    const url = 'https://indifferent.fun/blog/quiz-file-format-guide';
    const description = 'Format quiz TXT files correctly: numbered, bulleted and tab-indented patterns, answer markers, and common mistakes to avoid.';
    this.title.setTitle('Quiz File Format Guide: TXT Formatting Tips | Indifferent');
    this.meta.updateTag({ name: 'description', content: description });
    this.meta.updateTag({ property: 'og:title', content: 'Quiz File Format Guide: TXT Formatting Tips | Indifferent' });
    this.meta.updateTag({ property: 'og:description', content: description });
    this.meta.updateTag({ property: 'og:url', content: url });
    this.meta.updateTag({ property: 'og:image', content: 'https://indifferent.fun/og-cover.jpg' });
    setCanonical(this.document, url);
  }
}
