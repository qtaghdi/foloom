import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';

const workflowDirectory = resolve(process.env.WORKFLOW_DIR ?? '.github/workflows');
const releaseWorkflow = resolve(workflowDirectory, 'release.yml');

const fail = (messages) => {
  for (const message of messages) {
    console.error(`Private alpha policy failed: ${message}`);
  }
  process.exit(1);
};

if (!existsSync(releaseWorkflow)) {
  fail([`private alpha workflow is missing at ${releaseWorkflow}`]);
}

const releaseSource = readFileSync(releaseWorkflow, 'utf8');
const errors = [];
if (!/^\s*workflow_dispatch:\s*$/m.test(releaseSource)) {
  errors.push('release.yml must require an authorized manual dispatch');
}
if (/^\s*push:\s*$/m.test(releaseSource)) {
  errors.push('release.yml must not run from a branch or tag push');
}
if (/contents:\s*write/i.test(releaseSource)) {
  errors.push('release.yml must not request write access to repository contents');
}

const publicationPatterns = [
  /gh\s+release\s+create/i,
  /softprops\/action-gh-release/i,
  /actions\/create-release/i,
  /ncipollo\/release-action/i,
];
for (const name of readdirSync(workflowDirectory).filter((entry) => /\.ya?ml$/.test(entry))) {
  const source = readFileSync(resolve(workflowDirectory, name), 'utf8');
  for (const pattern of publicationPatterns) {
    if (pattern.test(source)) {
      errors.push(`${name} contains a public release publisher`);
      break;
    }
  }
}

if (errors.length > 0) {
  fail(errors);
}

console.log('Private alpha policy passed: manual artifacts only, read-only contents, no release publisher');
