// Canonical host: www never serves content, avoiding duplicate indexing.
if (window.location.hostname === 'www.indifferent.fun') {
  window.location.replace(`https://indifferent.fun${window.location.pathname}${window.location.search}${window.location.hash}`);
}

// CloudWatch RUM is started by core/consent (RumService) after the visitor allows Analytics (#321).
import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { App } from './app/app';

// Script-only states (the scroll reveal) are scoped under html.js, so with
// scripting off, or in the prerendered copy, everything stays visible (#256).
document.documentElement.classList.add('js');

bootstrapApplication(App, appConfig)
  .catch((err) => console.error(err));
