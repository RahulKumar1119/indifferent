import { failureMessage, FORMAT_GUIDE_PATH, SAMPLE_FILE_PATH } from './failure-message';

describe('failureMessage', () => {
  it('says the file had no questions, and what a question needs, for NO_QUESTION_FORMAT', () => {
    const m = failureMessage('NO_QUESTION_FORMAT');
    expect(m.fileProblem).toBeTrue();
    expect(m.title).toBe('We could not find any questions in your file');
    expect(m.text).toContain('lettered options');
    expect(m.text).toContain('answer line');
  });

  it('says no question was complete for NO_VALID_QUESTIONS', () => {
    const m = failureMessage('NO_VALID_QUESTIONS');
    expect(m.fileProblem).toBeTrue();
    expect(m.title).toBe('No question in your file was complete');
  });

  it('keeps the generic message, and does not blame the file, for anything else', () => {
    for (const code of [undefined, '', 'SOMETHING_ELSE']) {
      const m = failureMessage(code);
      expect(m.fileProblem).withContext(String(code)).toBeFalse();
      expect(m.title).toBe('Processing Failed');
      expect(m.text).not.toMatch(/file|question/i);
    }
  });

  it('never shows pipeline jargon and uses no em dashes', () => {
    for (const code of ['NO_QUESTION_FORMAT', 'NO_VALID_QUESTIONS', undefined]) {
      const m = failureMessage(code);
      expect(m.title + m.text).not.toMatch(/errorMessage|wrapError|detection|parser|S3|Step Functions|\u2014/);
    }
  });

  it('points at the format guide and a sample that the site actually serves', () => {
    expect(FORMAT_GUIDE_PATH).toBe('/blog/quiz-file-format-guide');
    expect(SAMPLE_FILE_PATH).toBe('/samples/quiz-sample.txt');
  });
});
