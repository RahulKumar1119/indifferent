import { Component, OnInit, inject } from '@angular/core';
import { DOCUMENT } from '@angular/common';
import { ArticleShellComponent } from './article-shell.component';
import { setArticleSeo } from './article-seo';
import { findArticle } from './registry';

@Component({
  selector: 'app-ai-narration-guide',
  standalone: true,
  imports: [ArticleShellComponent],
  template: `
    <app-article-shell [meta]="article">
        <p>AI narration makes professional voiceovers accessible to everyone, regardless of recording equipment or vocal training. For quiz video creators, AI narration provides consistent, clear, and natural-sounding audio that enhances the viewing experience and keeps audiences engaged. This guide covers everything you need to know about using AI narration effectively in your video projects.</p>

        <h2>What AI Narration Is and How It Works</h2>
        <p>AI narration uses text-to-speech technology to convert written text into natural-sounding human speech. Amazon Polly, which powers Indifferent's narration, offers several voice engines. Its neural engines use machine-learning models trained on recorded speech. Its standard engine, which Indifferent uses for all five of its voices, joins together short pieces of recorded speech, a method AWS calls concatenative synthesis. Either way, the engine decides intonation, rhythm, and emphasis from the text, which is why punctuation and sentence length change how a line sounds.</p>
        <p>The process works by analyzing your text at multiple levels. First, the system processes the linguistic content, understanding word boundaries, sentence structure, and parts of speech. Then it applies prosody rules that determine pitch, speed, and emphasis based on context. Finally, it synthesizes the audio waveform using learned voice characteristics, producing output that closely mimics human speech patterns.</p>
        <p>For quiz videos specifically, the narration system processes each question and its answer options as separate speech segments. This keeps each question and each answer in its own spoken segment, so the narration lines up with the question slide and the answer slide. The result is a synchronized audiovisual experience.</p>

        <h2>Benefits Over Manual Recording</h2>
        <p><strong>Consistency:</strong> AI voices maintain identical quality, volume, and tone across every question in your video and across multiple videos. Human narrators naturally vary in energy, pronunciation, and pacing between recording sessions, which can create an uneven viewing experience, especially in longer quiz videos.</p>
        <p><strong>Speed:</strong> Generating narration for a twenty-question quiz takes seconds rather than the hours required for manual recording, editing, and retakes. This dramatically accelerates your production workflow and lets you publish content more frequently.</p>
        <p><strong>Cost:</strong> Professional voice actors charge per word or per minute of finished audio. AI narration removes that per-word fee, and Indifferent is free during the beta, so producing many videos does not add up the way studio sessions do.</p>
        <p><strong>Iteration:</strong> If you need to change a question or fix a typo, you edit the text file and render the video again, which takes a few minutes. With human recording, any change requires scheduling another session, re-recording, and re-editing the audio.</p>
        <p><strong>Scalability:</strong> Whether you need narration for one video or one hundred, AI narration scales instantly without additional coordination, scheduling, or cost considerations.</p>

        <h2>Overview of Available Voices</h2>
        <p>Indifferent offers five carefully selected AI voices, each suited to different content styles and audiences:</p>

        <h3>Joanna</h3>
        <p>Joanna is an American English female voice with a warm, clear, and professional tone. She speaks at a moderate pace with excellent clarity on complex vocabulary. Joanna is the most popular choice for educational content because her voice conveys authority while remaining approachable and engaging. She handles technical terminology particularly well.</p>

        <h3>Matthew</h3>
        <p>Matthew is an American English male voice with a deep, confident tone that projects knowledge and reliability. His natural speaking rhythm is slightly slower than Joanna's, which works well for content that requires careful attention to detail. Matthew is an excellent choice for science, history, and professional development content.</p>

        <h3>Amy</h3>
        <p>Amy is a British English female voice with a crisp, articulate delivery that conveys sophistication and precision. Her accent adds variety for audiences accustomed to American English voices and works particularly well for literature, arts, and humanities content. Amy's clear enunciation makes her ideal for content with complex vocabulary or proper nouns.</p>

        <h3>Brian</h3>
        <p>Brian is a British English male voice with a measured, authoritative tone. His voice carries a natural gravitas that suits serious subjects and formal educational contexts. Brian excels at content related to business, law, politics, and advanced academic subjects. His pacing gives listeners ample time to process complex information.</p>

        <h3>Aditi</h3>
        <p>Aditi is an Indian English female voice that brings diversity to your content and connects with the large global audience of Indian English speakers. Her clear pronunciation and moderate pace make her accessible to international audiences while providing representation that resonates with viewers from South Asia.</p>

        <h2>Choosing the Right Voice for Your Content</h2>
        <p>Selecting the appropriate voice involves considering your audience demographics, subject matter, and the emotional tone you want to establish. Younger audiences generally respond well to voices with more energy and warmth, while professional audiences prefer measured, authoritative delivery.</p>
        <p>Consider the subject matter carefully. Science and technology content benefits from precise, clear voices that handle technical terms confidently. Creative subjects like art, music, and literature can work with more expressive voices. Quiz content about entertainment and pop culture suits voices with natural warmth and enthusiasm.</p>
        <p>Platform matters too. Content destined for YouTube often benefits from more energetic voices that compete with the platform's fast-paced environment. Content embedded in learning management systems or corporate training platforms can use calmer, more measured voices since viewers are typically in a focused learning mindset.</p>

        <h2>Tips for Writing Narration-Friendly Scripts</h2>
        <p>The quality of your narration depends heavily on how you write your questions and answer options. AI voices perform best with clear, straightforward sentences that follow natural speech patterns. Here are key principles for narration-optimized writing:</p>
        <ul>
          <li>Write complete sentences rather than fragments or bullet-point shorthand</li>
          <li>Avoid excessive abbreviations: spell out terms that should be spoken in full</li>
          <li>Use punctuation to control pacing: commas create brief pauses, periods create longer ones</li>
          <li>Keep question sentences under twenty-five words for comfortable listening</li>
          <li>Spell out numbers under ten and use numerals for larger numbers that have obvious pronunciation</li>
          <li>Avoid homographs (words spelled the same but pronounced differently) where context might be ambiguous</li>
        </ul>
        <p>For answer options, keep each choice concise and parallel in structure. When all four options follow the same grammatical pattern, the narration flows naturally and viewers can focus on content differences rather than processing varied sentence structures.</p>

        <h2>Pacing and Timing Considerations</h2>
        <p>Effective narration pacing aligns with the visual rhythm of your quiz video. The question narration should complete before the countdown timer begins, giving viewers a moment to process what they heard and read simultaneously. A brief pause between the question and the start of the timer creates comfortable breathing room that reduces anxiety.</p>
        <p>Answer option narration speed matters for comprehension. Speaking too quickly forces viewers to choose between listening and reading, which creates cognitive conflict. The narration speed in Indifferent is calibrated to match comfortable reading speed, allowing viewers to process both channels simultaneously.</p>
        <p>The answer reveal moment should include a brief narration confirming the correct answer. This auditory confirmation reinforces the visual cue and helps viewers who may have looked away from the screen momentarily. A simple "The correct answer is..." followed by the option text provides clear closure for each question.</p>

        <h2>When Human Narration Might Be Better</h2>
        <p>Despite the advantages of AI narration, certain situations still benefit from human voices. Highly emotional content that requires conveying genuine enthusiasm, humor, or empathy may feel more authentic with a human narrator who can bring personal expression to the delivery.</p>
        <p>Content that relies heavily on vocal performance (dramatic readings, character voices, or comedic timing) generally requires human talent. AI voices excel at clear, informative delivery but cannot match the creative range of a skilled human performer.</p>
        <p>Brand-building content where a recognizable personal voice is part of the channel identity may also warrant human narration. Some creators' voices become inseparable from their brand, and switching to AI narration could feel disconnected to existing audiences.</p>

        <h2>The Future of AI Voices in Education</h2>
        <p>AI voice technology keeps improving. AWS now offers neural, long-form, and generative engines next to the standard one. Indifferent currently uses the standard engine for its five voices, so it does not offer the newer engines yet.</p>
        <p>Across the industry, voice cloning, narration that switches languages inside one video, and pacing that adapts to the content are active areas of work. Indifferent does not offer any of these today.</p>
        <p>For educational content creators, these advances mean that the gap between AI and human narration will continue to narrow. Investing in AI narration workflows today positions you to benefit from each improvement without changing your production process. As the technology improves, your existing content structure and text formatting skills will produce increasingly natural results.</p>
      
    </app-article-shell>
  `,
})
export class AiNarrationGuideComponent implements OnInit {
  private readonly document = inject(DOCUMENT);
  protected readonly article = findArticle('ai-narration-guide')!;

  ngOnInit(): void {
    setArticleSeo(this.document, this.article);
  }
}
