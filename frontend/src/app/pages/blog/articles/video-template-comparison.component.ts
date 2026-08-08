import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-video-template-comparison',
  standalone: true,
  imports: [RouterLink],
  styles: [`
    :host {
      display: block;
      background: #f8fafc;
      min-height: 100vh;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    }
    .blog-nav {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 1rem 2rem;
      background: rgba(255,255,255,0.95);
      border-bottom: 1px solid #e2e8f0;
      backdrop-filter: blur(12px);
      position: sticky;
      top: 0;
      z-index: 100;
    }
    .blog-logo img { height: 2rem; }
    .blog-signin {
      background: #0d9488;
      color: #fff;
      padding: 0.5rem 1.25rem;
      border-radius: 0.5rem;
      font-weight: 600;
      font-size: 0.875rem;
      text-decoration: none;
    }
    .blog-signin:hover { background: #0f766e; }
    .blog-article {
      max-width: 720px;
      margin: 0 auto;
      padding: 3rem 2rem;
    }
    .blog-article-header { margin-bottom: 2rem; }
    .blog-back {
      color: #0d9488;
      font-size: 0.875rem;
      text-decoration: none;
      display: inline-block;
      margin-bottom: 1.5rem;
    }
    .blog-back:hover { text-decoration: underline; }
    .blog-article-header h1 {
      font-size: 2.25rem;
      font-weight: 800;
      color: #0f172a;
      line-height: 1.3;
      margin: 0 0 0.75rem;
    }
    .blog-meta {
      color: #64748b;
      font-size: 0.875rem;
    }
    .blog-content {
      color: #334155;
      font-size: 1.0625rem;
      line-height: 1.8;
    }
    .blog-content h2 {
      font-size: 1.5rem;
      font-weight: 700;
      color: #0f172a;
      margin: 2.5rem 0 1rem;
    }
    .blog-content h3 {
      font-size: 1.25rem;
      font-weight: 600;
      color: #0f172a;
      margin: 2rem 0 0.75rem;
    }
    .blog-content p { margin: 0 0 1.25rem; }
    .blog-content ul, .blog-content ol {
      margin: 0 0 1.25rem;
      padding-left: 1.5rem;
    }
    .blog-content li { margin-bottom: 0.5rem; }
    .blog-content code {
      background: #f1f5f9;
      padding: 0.2rem 0.4rem;
      border-radius: 0.25rem;
      font-size: 0.875rem;
    }
    .blog-content pre {
      background: #1e293b;
      color: #e2e8f0;
      padding: 1.25rem;
      border-radius: 0.5rem;
      overflow-x: auto;
      margin: 0 0 1.25rem;
      font-size: 0.875rem;
      line-height: 1.6;
    }
    .blog-content strong { color: #0f172a; font-weight: 600; }
    .blog-cta {
      margin-top: 3rem;
      padding: 2rem;
      background: #ffffff;
      border: 1px solid #e2e8f0;
      border-radius: 0.75rem;
      text-align: center;
    }
    .blog-cta h3 {
      color: #0f172a;
      font-size: 1.25rem;
      font-weight: 700;
      margin: 0 0 0.5rem;
    }
    .blog-cta p { color: #64748b; margin: 0 0 1.25rem; }
    .blog-cta-btn {
      display: inline-block;
      background: #0d9488;
      color: #fff;
      padding: 0.75rem 1.5rem;
      border-radius: 0.5rem;
      font-weight: 600;
      text-decoration: none;
    }
    .blog-cta-btn:hover { background: #0f766e; }
    .blog-footer {
      text-align: center;
      padding: 2rem;
      border-top: 1px solid #e2e8f0;
    }
    .blog-footer a {
      color: #64748b;
      font-size: 0.8rem;
      text-decoration: none;
      margin: 0 0.5rem;
    }
    .blog-footer a:hover { color: #0d9488; }
    .blog-footer p {
      color: #94a3b8;
      font-size: 0.7rem;
      margin-top: 0.5rem;
    }
  `],
  template: `
    <nav class="blog-nav">
      <a routerLink="/" class="blog-logo"><img src="logo.svg" alt="Indifferent" /></a>
      <a routerLink="/login" class="blog-signin">Sign In</a>
    </nav>
    <article class="blog-article">
      <header class="blog-article-header">
        <a routerLink="/blog" class="blog-back">&larr; Back to Blog</a>
        <h1>Video Template Comparison: Which Style Fits Your Content?</h1>
        <div class="blog-meta">Published December 28, 2024 · 7 min read</div>
      </header>
      <div class="blog-content">
        <p>Choosing the right visual template for your quiz video is more than an aesthetic decision — it directly affects how viewers perceive your content, how long they stay engaged, and whether your video feels appropriate for its intended audience. Indifferent offers six distinct templates, each designed for different content types, audiences, and platforms. This guide will help you understand the strengths of each option and make an informed choice for your next project.</p>

        <h2>Overview of All Six Templates</h2>
        <p>Each template in the Indifferent library has been carefully designed to balance readability, visual appeal, and professional quality. While all templates support the same quiz functionality including countdown timers, answer reveals, and narration synchronization, they differ significantly in visual tone, color palette, and typographic style. Understanding these differences helps you select the template that best communicates your brand and resonates with your target audience.</p>

        <h2>Classic Template</h2>
        <p>The Classic template features a clean white background with navy blue text and subtle accent colors. It uses a traditional serif-inspired heading font paired with a highly readable sans-serif body font. The design is intentionally understated, allowing the content to take center stage without visual distractions.</p>
        <p><strong>Visual characteristics:</strong> White and light gray backgrounds, navy blue primary text, teal accent for correct answers, rounded corners on answer option cards, gentle shadow effects for depth. The countdown timer uses a circular progress indicator in teal that smoothly animates as time decreases.</p>
        <p><strong>Best for:</strong> Educational institutions, corporate training, professional development courses, and any context where credibility and clarity are paramount. The Classic template conveys authority and trustworthiness, making it ideal for subjects like history, science, business, and academic test preparation.</p>

        <h2>Modern Template</h2>
        <p>The Modern template uses a gradient background that shifts between deep purple and electric blue, creating a vibrant and contemporary feel. Text appears in crisp white with generous spacing, and answer options are displayed in semi-transparent glass-effect cards that add visual depth without sacrificing readability.</p>
        <p><strong>Visual characteristics:</strong> Purple-to-blue gradient backgrounds, white text with subtle glow effects, frosted glass answer cards, bold sans-serif typography, animated transitions between question states. The countdown timer features a linear bar that changes color from blue to amber to red as time decreases.</p>
        <p><strong>Best for:</strong> Technology topics, gaming quizzes, content aimed at younger audiences (teenagers and young adults), and creators who want their videos to feel current and energetic. The Modern template works well on social media platforms where bold visuals help content stand out in crowded feeds.</p>

        <h2>Education Template</h2>
        <p>The Education template draws inspiration from classroom environments with a warm, inviting color scheme built around soft greens, warm whites, and chalkboard-inspired accent elements. It prioritizes maximum readability and features larger text sizes compared to other templates, making it accessible to viewers of all ages.</p>
        <p><strong>Visual characteristics:</strong> Warm off-white backgrounds with subtle paper texture, dark green primary text, orange highlights for emphasis, hand-drawn style borders and decorative elements, extra-large question text. The countdown timer resembles an hourglass with sand particles that flow as time passes.</p>
        <p><strong>Best for:</strong> K-12 educational content, language learning, elementary-level subjects, content for senior learners, and any situation where maximum accessibility and warmth are priorities. The Education template feels approachable and non-intimidating, encouraging learners to engage without anxiety.</p>

        <h2>Dark Template</h2>
        <p>The Dark template uses a rich charcoal and near-black color scheme with high-contrast text in white and warm amber accents. This design reduces eye strain during extended viewing sessions and creates a premium, sophisticated atmosphere that appeals to mature audiences.</p>
        <p><strong>Visual characteristics:</strong> Dark charcoal backgrounds with subtle gradient, white primary text, amber and gold accents for correct answers and highlights, sleek sans-serif typography, minimal decorative elements. The countdown timer uses a slim neon line that smoothly depletes against the dark background.</p>
        <p><strong>Best for:</strong> Evening viewing content, advanced-level subjects, professional certification prep, content aimed at adult learners, and topics that benefit from a serious or premium feel such as finance, law, medicine, or advanced technology. The Dark template also performs well for gaming-related quiz content.</p>

        <h2>Minimal Template</h2>
        <p>The Minimal template strips away all unnecessary visual elements, presenting questions in a stark black-on-white format with generous white space and precise typographic spacing. Every element serves a functional purpose, creating an experience that feels calm, focused, and intellectually sophisticated.</p>
        <p><strong>Visual characteristics:</strong> Pure white backgrounds, black text, single accent color (a muted blue-gray) used sparingly for interaction states, monospaced font for answer option letters, generous margins and line spacing. The countdown timer is a thin horizontal line at the top of the screen that subtly decreases in width.</p>
        <p><strong>Best for:</strong> Academic and research contexts, content that emphasizes intellectual rigor, minimalist brand identities, professional audiences who prefer clean interfaces, and subjects where the content itself should be the sole focus. The Minimal template works well for philosophy, literature, mathematics, and logic puzzles.</p>

        <h2>Neon Template</h2>
        <p>The Neon template brings bold energy with a dark background illuminated by vibrant neon-colored elements in electric pink, cyan, and lime green. Text and UI elements feature glow effects that create a retro-futuristic arcade aesthetic, making the quiz feel like an exciting game show.</p>
        <p><strong>Visual characteristics:</strong> Deep navy to black backgrounds, neon pink and cyan text with glow effects, lime green for correct answer highlights, animated pulse effects on interactive elements, bold display typography. The countdown timer features an animated neon ring that changes color intensity as time runs out.</p>
        <p><strong>Best for:</strong> Entertainment and trivia night content, gaming communities, pop culture quizzes, music and movie trivia, content targeting Gen-Z audiences, and any quiz designed purely for fun rather than formal education. The Neon template maximizes excitement and creates a party atmosphere that encourages sharing.</p>

        <h2>Color Psychology in Educational Content</h2>
        <p>The colors in your quiz video influence viewer emotions and behavior in measurable ways. Blue tones promote trust, calm, and focus — ideal for educational content where concentration matters. Green creates feelings of balance and growth, making it appropriate for learning contexts. Purple suggests creativity and wisdom, working well for thought-provoking content.</p>
        <p>Warm colors like orange and amber create energy and enthusiasm but should be used sparingly to avoid overwhelming viewers. Red signals urgency and importance — effective for countdown timers and incorrect answer indicators but counterproductive if overused throughout the design.</p>
        <p>High contrast between text and background is essential for comprehension. Dark text on light backgrounds produces the best readability scores in research, though light text on dark backgrounds can work well when font sizes are larger and line spacing is generous.</p>

        <h2>Typography Choices and Readability</h2>
        <p>Each template uses carefully selected font pairings optimized for on-screen reading at video resolution. The primary concern is legibility at various screen sizes — from large desktop monitors to small mobile phone screens where many YouTube viewers watch content.</p>
        <p>Sans-serif fonts dominate our template designs because they render more clearly at screen resolution than serif alternatives. Heading fonts are selected for personality and impact, while body text fonts prioritize even letter spacing and clear distinction between similar characters like lowercase L and the number one.</p>
        <p>Font size in video content should be larger than you might expect. Viewers often watch on small screens, sometimes while multitasking, so text needs to be immediately readable without squinting or pausing. Our templates use minimum sizes that ensure comfortable reading across all common viewing devices.</p>

        <h2>How Templates Affect Viewer Engagement</h2>
        <p>Template choice measurably impacts viewer retention. Videos using templates that match their content tone typically see fifteen to twenty percent longer average watch times compared to mismatched combinations. A serious academic quiz presented in the Neon template, for example, creates cognitive dissonance that drives viewers away, while the same content in the Education or Classic template feels natural and trustworthy.</p>
        <p>Consistency matters for channel growth. Viewers who enjoy one of your videos expect a similar experience from subsequent uploads. Establishing a signature template creates brand recognition and builds viewer loyalty. Consider using a single template for all content within a series, switching templates only when launching distinctly different content categories.</p>

        <h2>Customization Tips</h2>
        <p>While each template provides a complete visual system ready to use, consider these strategies for making the most of your chosen style. First, match your channel branding to your template choice from the start. If your channel uses purple and white in its banner and logo, the Modern template creates natural visual continuity. If your brand is clean and professional, the Classic or Minimal templates align with that identity.</p>
        <p>Second, consider your viewing context. Content watched primarily on mobile devices benefits from bolder templates with larger text (Modern, Neon, Education). Content consumed on desktops or projected in classrooms can use subtler designs (Minimal, Classic) because viewers have larger, higher-resolution displays.</p>
        <p>Finally, test different templates with your audience. Create the same quiz in two different templates and compare engagement metrics. Your specific audience may respond differently than general trends suggest, and real data from your viewers is always more valuable than assumptions about what they prefer.</p>
      </div>
      <div class="blog-cta">
        <h3>Ready to create your first quiz video?</h3>
        <p>Sign up free and convert your text quizzes into professional videos in minutes.</p>
        <a routerLink="/login" class="blog-cta-btn">Get Started Free</a>
      </div>
    </article>
    <footer class="blog-footer">
      <a routerLink="/">Home</a>
      <a routerLink="/about">About</a>
      <a routerLink="/contact">Contact</a>
      <a routerLink="/privacy">Privacy</a>
      <a routerLink="/terms">Terms</a>
      <p>&copy; 2025 Indifferent. All rights reserved.</p>
    </footer>
  `,
})
export class VideoTemplateComparisonComponent {}
