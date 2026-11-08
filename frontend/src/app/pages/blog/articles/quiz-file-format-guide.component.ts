import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { ArticleShellComponent } from './article-shell.component';
import { setArticleSeo } from './article-seo';
import { findArticle } from './registry';

@Component({
  selector: 'app-quiz-file-format-guide',
  standalone: true,
  imports: [ArticleShellComponent],
  template: `
    <app-article-shell [meta]="article">
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

        <h3>Marking the correct answer</h3>
        <p>There are several ways to mark the answer, and you can use any one of them. The parser accepts all of these:</p>
        <ul>
          <li><strong>An answer line:</strong> <code>Answer: B</code>, or <code>Answer: B, D</code> when more than one option is correct. In the numbered format use the option letter. In the bulleted and tabbed formats use the full text of the option, for example <code>Answer: Jupiter</code>.</li>
          <li><strong>An asterisk after the option:</strong> <code>b) Paris *</code></li>
          <li><strong>An asterisk before the option:</strong> <code>*b) Paris</code>, in the numbered format</li>
          <li><strong>A (correct) note:</strong> <code>b) Paris (correct)</code></li>
        </ul>
        <p>If a question has no marked answer, the video still renders, but that question gets no answer reveal, and the project page tells you which question it was. Nothing is silently dropped.</p>

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
        <p><strong>Missing answer indicators:</strong> Every question should have a clearly marked correct answer. Without one, the parser cannot determine which option is correct, so that question renders with no answer reveal. The project page lists these as warnings so you can fix the file.</p>
        <p><strong>Extra blank lines:</strong> While one blank line between questions is fine and can improve readability, multiple consecutive blank lines may cause the parser to treat separate parts of the same question as different entries. Keep spacing consistent with single blank lines between question blocks.</p>
        <p><strong>Special characters in questions:</strong> Avoid using characters that might be interpreted as formatting markers. Specifically, do not start questions with dashes, asterisks, or tab characters unless you intend them to be parsed as answer options.</p>
        <p><strong>Inconsistent answer format:</strong> The parser reads a letter or the option text on the answer line, so both styles work. Keeping one style throughout your file is still simpler to check by eye.</p>

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
        <p>Indifferent accepts text files up to five megabytes in size, which is far more than a quiz needs. The 5 MB file size is the only limit it enforces. For video length and viewer engagement, we recommend the following guidelines:</p>
        <ul>
          <li>Minimum: 5 questions (produces a video of approximately two minutes)</li>
          <li>Recommended: 15 to 25 questions (produces videos of eight to fifteen minutes, ideal for YouTube)</li>
          <li>Upper end: around 100 questions per file. There is no hard cap on the number of questions, so this is a viewing guideline, not a limit. Split longer quizzes into several videos</li>
        </ul>
        <p>Each question should have between three and five answer options. Four options is the sweet spot: enough to provide genuine challenge without overwhelming viewers with too many choices to read while the question is on screen.</p>

        <h2>Best Practices for Answer Options</h2>
        <p>The standard and most effective configuration is four answer options with exactly one correct answer. This format provides enough variety to be challenging while remaining manageable for viewers to read and evaluate within the time the question is on screen.</p>
        <p>Three options work well for simpler topics or content aimed at younger audiences. They reduce cognitive load and allow for faster pacing. Five options are appropriate for advanced or professional-level content where more nuanced distinctions between answers are meaningful.</p>
        <p>Always place the correct answer in a random position across your questions. If the correct answer is consistently option B or always the longest choice, observant viewers will notice the pattern and game the system rather than actually testing their knowledge.</p>

        <h2>How the Parser Handles Edge Cases</h2>
        <p>The Indifferent parser is designed to be forgiving and handle common variations gracefully. If the parser encounters a line it cannot categorize, it attempts to associate it with the nearest question context. Trailing whitespace is automatically trimmed, and various line ending formats (Windows, Unix, Mac) are all supported.</p>
        <p>When the parser detects a question with fewer than two answer options, it flags it as potentially incomplete and will notify you during the upload process. Questions with no marked correct answer are similarly flagged, giving you the opportunity to fix issues before video generation begins.</p>
        <p>The parser also handles UTF-8 byte order marks that some text editors add automatically, ensuring compatibility across different operating systems and text editing tools. Whether you create your file in Notepad, TextEdit, VS Code, or any other editor, the output will parse correctly as long as you follow the formatting guidelines described above.</p>
      
    </app-article-shell>
  `,
})
export class QuizFileFormatGuideComponent implements OnInit {
  private readonly document = inject(DOCUMENT);
  protected readonly article = findArticle('quiz-file-format-guide')!;

  ngOnInit(): void {
    setArticleSeo(this.document, this.article);
  }
}
