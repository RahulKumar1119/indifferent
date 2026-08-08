import { Component } from '@angular/core';
import { RouterLink } from '@angular/router';

@Component({
  selector: 'app-blog',
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
    .blog-list {
      max-width: 720px;
      margin: 0 auto;
      padding: 0 2rem 2rem;
    }
    .blog-list-header {
      text-align: center;
      padding: 3rem 0 2rem;
    }
    .blog-list-header h1 {
      font-size: 2.25rem;
      font-weight: 800;
      color: #0f172a;
      margin: 0 0 0.5rem;
    }
    .blog-list-header p {
      color: #64748b;
      font-size: 1.1rem;
      margin: 0;
    }
    .blog-card {
      background: #ffffff;
      border: 1px solid #e2e8f0;
      border-radius: 0.75rem;
      padding: 1.5rem;
      margin-bottom: 1rem;
      text-decoration: none;
      display: block;
      transition: box-shadow 0.2s, transform 0.2s;
    }
    .blog-card:hover {
      box-shadow: 0 4px 12px rgba(0,0,0,0.06);
      transform: translateY(-2px);
    }
    .blog-card h2 {
      font-size: 1.25rem;
      font-weight: 700;
      color: #0f172a;
      margin: 0 0 0.5rem;
    }
    .blog-card p {
      color: #475569;
      font-size: 0.9rem;
      line-height: 1.6;
      margin: 0 0 0.75rem;
    }
    .blog-card-meta {
      color: #94a3b8;
      font-size: 0.8rem;
    }
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

    <div class="blog-list">
      <div class="blog-list-header">
        <h1>Blog</h1>
        <p>Tips, guides, and insights on creating quiz videos and educational content</p>
      </div>

      <a routerLink="/blog/quiz-videos-youtube" class="blog-card">
        <h2>How to Create Quiz Videos for YouTube</h2>
        <p>Learn the complete process of creating engaging quiz videos that drive views, comments, and subscriber growth on YouTube.</p>
        <span class="blog-card-meta">January 15, 2025 · 6 min read</span>
      </a>

      <a routerLink="/blog/educational-video-best-practices" class="blog-card">
        <h2>Best Practices for Educational Video Content</h2>
        <p>Discover proven strategies for creating educational videos that improve knowledge retention and keep learners engaged.</p>
        <span class="blog-card-meta">January 10, 2025 · 7 min read</span>
      </a>

      <a routerLink="/blog/quiz-file-format-guide" class="blog-card">
        <h2>Quiz File Format Guide: TXT Formatting Tips</h2>
        <p>A comprehensive guide to formatting your quiz text files for optimal parsing and video generation with Indifferent.</p>
        <span class="blog-card-meta">January 5, 2025 · 6 min read</span>
      </a>

      <a routerLink="/blog/video-template-comparison" class="blog-card">
        <h2>Video Template Comparison: Which Style Fits Your Content?</h2>
        <p>Compare all six video templates and learn which visual style works best for your audience and subject matter.</p>
        <span class="blog-card-meta">December 28, 2024 · 7 min read</span>
      </a>

      <a routerLink="/blog/ai-narration-guide" class="blog-card">
        <h2>AI Narration for Videos: A Complete Guide</h2>
        <p>Everything you need to know about using AI-powered narration to add professional voiceovers to your quiz videos.</p>
        <span class="blog-card-meta">December 20, 2024 · 6 min read</span>
      </a>
    </div>

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
export class BlogComponent {}
