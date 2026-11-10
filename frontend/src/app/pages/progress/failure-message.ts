import { FailureCode } from '../../shared';

export interface FailureMessage {
  title: string;
  text: string;
  /** True when the fix is a different file, so the screen offers the format help. */
  fileProblem: boolean;
  /** True when the file may be a story rather than questions, so the screen points to Story video. */
  storyHint?: boolean;
}

/** Where a story or script turns into a video. */
export const STORY_PATH = '/story';

/** Where the format is explained, and the file that is known to work. */
export const FORMAT_GUIDE_PATH = '/blog/quiz-file-format-guide';
export const SAMPLE_FILE_PATH = '/samples/quiz-sample.txt';

const GENERIC: FailureMessage = {
  title: 'Processing failed',
  text: 'Something went wrong while processing your video. Please try again.',
  fileProblem: false,
};

/**
 * What to tell the person about a failed project. The server sends only a short
 * code for failures the file causes; anything else keeps the generic message
 * rather than guessing.
 */
export function failureMessage(code: FailureCode | string | undefined): FailureMessage {
  switch (code) {
    case 'NO_QUESTION_FORMAT':
      return {
        title: 'We could not find any questions in your file',
        text: 'Each question needs a number or bullet, lettered options (A, B, C, D) and an answer line. The format guide shows how, and the sample file is a quick way to see one that works.',
        fileProblem: true,
        storyHint: true,
      };
    case 'NO_VALID_QUESTIONS':
      return {
        title: 'No question in your file was complete',
        text: 'We found text, but every question was missing something, such as its options. Each question needs its text and its options, and an answer line to show the correct one.',
        fileProblem: true,
      };
    default:
      return GENERIC;
  }
}
