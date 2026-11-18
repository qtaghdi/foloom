import type { Run } from './contracts';

const eastAsianScripts = '[\\p{Script_Extensions=Han}\\p{Script_Extensions=Hangul}\\p{Script_Extensions=Hiragana}\\p{Script_Extensions=Katakana}\\p{Script_Extensions=Bopomofo}]';
const eastAsianLetter = new RegExp(`^${eastAsianScripts}+$`, 'u');
const eastAsianSegments = new RegExp(`((?:${eastAsianScripts}\\p{M}*)+)`, 'u');

export function runFont(run: Run): string | undefined {
  const letters = run.eastAsiaFont ? run.text.replace(/[\p{P}\p{Z}\p{N}\s]/gu, '') : '';
  return letters && eastAsianLetter.test(letters) ? run.eastAsiaFont : run.font;
}

export function splitRunFonts(run: Run, text = run.text) {
  const base = runFont(run);
  const eastAsia = run.eastAsiaFont;
  if (!eastAsia || eastAsia === base || !text) return [{ text, font: base }];
  // The full run determines the default font, even when copying only a slice.
  return text.split(eastAsianSegments).flatMap((segment, index) =>
    segment ? [{ text: segment, font: index % 2 === 1 ? eastAsia : base }] : []);
}
