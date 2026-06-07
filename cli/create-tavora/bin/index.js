#!/usr/bin/env node
// create-tavora — scaffold a Tavora agents project.
//
// Invoked via `npm create tavora` (which npm rewrites to
// `npx create-tavora`). Mirrors the Convex / Vite / Astro pattern:
// prompt for a project name, make the directory, then hand off the
// agent-folder scaffold to `tavora init` so there's a single source
// of truth for the starter files (see
// tavora-cli/internal/codefirst/scaffold/scaffold.go).

'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');
const readline = require('node:readline/promises');

const ARGV = process.argv.slice(2);
const flags = { yes: false, inPlace: false, template: 'starter' };
let positional = null;

for (let i = 0; i < ARGV.length; i++) {
  const a = ARGV[i];
  if (a === '--yes' || a === '-y') flags.yes = true;
  else if (a === '--in-place') flags.inPlace = true;
  else if (a === '--template' || a === '-t') flags.template = ARGV[++i] || 'starter';
  else if (a === '--help' || a === '-h') { printHelp(); process.exit(0); }
  else if (!a.startsWith('-') && positional === null) positional = a;
  else if (a.startsWith('-')) {
    console.error(`create-tavora: unknown option: ${a}`);
    console.error(`Run \`npm create tavora -- --help\` for usage.`);
    process.exit(2);
  }
}

(async () => {
  let name = positional;

  if (flags.inPlace) {
    name = name || path.basename(process.cwd());
  } else if (!name) {
    name = flags.yes ? 'tavora-app' : await prompt('Project name', 'tavora-app');
  }

  if (!isValidName(name)) {
    fail(
      `Invalid project name: ${JSON.stringify(name)}\n` +
      `Use lowercase letters, digits, and single hyphens — e.g. "support-bot" (1-50 chars).`,
    );
  }

  const targetDir = flags.inPlace ? process.cwd() : path.resolve(process.cwd(), name);
  const tavoraDir = path.join(targetDir, 'tavora');

  if (!flags.inPlace) {
    if (fs.existsSync(targetDir)) {
      const entries = fs.readdirSync(targetDir).filter((e) => e !== '.git');
      if (entries.length > 0) {
        if (flags.yes) {
          fail(`Directory ${targetDir} already exists and is not empty.`);
        }
        const ok = await confirm(`Directory "${name}" exists and is not empty. Continue?`, false);
        if (!ok) process.exit(1);
      }
    } else {
      fs.mkdirSync(targetDir, { recursive: true });
    }
  }

  writeOuterReadme(targetDir, name);

  const tavoraCmd = resolveTavoraCmd();
  // --no-footer suppresses tavora init's own "Next steps:" so we
  // don't print two contradictory footers (init's footer assumes the
  // user already has the CLI installed; ours bootstraps the install).
  const initArgs = ['init', '--dir', tavoraDir, '--project', name, '--no-footer'];
  console.log('');
  console.log(`▸ ${formatCmd(tavoraCmd, initArgs)}`);
  console.log('');
  const code = await runTavora(tavoraCmd, initArgs);
  if (code !== 0) {
    fail(
      `tavora init exited with code ${code}.\n` +
      `If the @tavora/cli download failed, see https://github.com/tavora-ai/tavora-sdk-go for manual install.`,
    );
  }

  printNextSteps(name, flags.inPlace);
})().catch((err) => {
  console.error('create-tavora:', err && err.message ? err.message : err);
  process.exit(1);
});

// --- helpers ---

function isValidName(s) {
  if (typeof s !== 'string') return false;
  if (s.length < 1 || s.length > 50) return false;
  if (!/^[a-z][a-z0-9-]*$/.test(s)) return false;
  if (s.includes('--')) return false;
  if (s.endsWith('-')) return false;
  return true;
}

async function prompt(message, def) {
  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
  try {
    const ans = (await rl.question(`${message}${def ? ` (${def})` : ''}: `)).trim();
    return ans || def;
  } finally {
    rl.close();
  }
}

async function confirm(message, def) {
  const hint = def ? '[Y/n]' : '[y/N]';
  const ans = (await prompt(`${message} ${hint}`, '')).toLowerCase();
  if (!ans) return def;
  return ans === 'y' || ans === 'yes';
}

// resolveTavoraCmd picks how to invoke `tavora init`. Priority:
//   1. $TAVORA_BIN — explicit override (useful for local dev against
//      an unreleased binary, or air-gapped installs).
//   2. `tavora` on PATH — already-installed CLI.
//   3. `npx --yes @tavora/cli` — fetches the published wrapper, which
//      in turn pulls the Go binary from a GitHub Release.
function resolveTavoraCmd() {
  if (process.env.TAVORA_BIN) {
    return { cmd: process.env.TAVORA_BIN, args: [] };
  }
  const probe = spawnSync(
    process.platform === 'win32' ? 'where' : 'which',
    ['tavora'],
    { encoding: 'utf8' },
  );
  if (probe.status === 0 && probe.stdout.trim()) {
    return { cmd: 'tavora', args: [] };
  }
  return { cmd: 'npx', args: ['--yes', '@tavora/cli'] };
}

function formatCmd({ cmd, args }, extra) {
  return [cmd, ...args, ...extra].join(' ');
}

function runTavora({ cmd, args }, extra) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, [...args, ...extra], { stdio: 'inherit' });
    child.on('error', reject);
    child.on('exit', (code) => resolve(code ?? 1));
  });
}

function writeOuterReadme(dir, name) {
  const p = path.join(dir, 'README.md');
  if (fs.existsSync(p)) return;
  const body = `# ${name}

A Tavora agents project. Agents are defined as code under \`tavora/\`
and synced to the Tavora cloud as a mutable dev draft on save, then
promoted to an immutable published version with \`tavora deploy\`.

## Layout

\`\`\`
${name}/
  README.md           ← this file
  tavora/             ← agent source-of-truth (manifest, agents, skills, evals)
    tavora.jsonc
    AGENTS.md
    agents/
      starter/
        agent.jsonc
        persona.md
        skills/
        evals/
\`\`\`

When you wire this into a real backend, copy or move the \`tavora/\`
folder into that repo — it's the entire portable unit.

## Quickstart

\`\`\`sh
cd ${name}
npm install -g @tavora/cli        # one-time; ships a small Go binary
tavora login                      # authenticate against the Tavora cloud
tavora dev                        # validate, watch, sync a dev draft
tavora run starter "Hello"        # invoke the just-synced draft
tavora deploy                     # cut an immutable published version
\`\`\`

Docs: https://docs.tavora.ai/code-first
`;
  fs.writeFileSync(p, body);
}

function printNextSteps(name, inPlace) {
  console.log('');
  console.log('✓ Done.');
  console.log('');
  console.log('Next steps:');
  if (!inPlace) console.log(`  cd ${name}`);
  console.log('  npm install -g @tavora/cli   # or use `npx @tavora/cli ...`');
  console.log('  tavora login');
  console.log('  tavora dev');
  console.log('');
  console.log('Docs: https://docs.tavora.ai/code-first');
}

function fail(msg) {
  console.error('create-tavora:', msg);
  process.exit(1);
}

function printHelp() {
  console.log(`create-tavora — scaffold a Tavora agents project.

Usage:
  npm create tavora [name]
  npm create tavora [name] -- [options]

Arguments:
  [name]            Project directory to create. Prompted if omitted.

Options:
  --in-place        Scaffold into the current directory instead of a new one.
  --yes, -y         Skip prompts; use defaults.
  --template, -t    Template to use (default: starter).
  --help, -h        Show this help.

Environment:
  TAVORA_BIN        Path to a tavora binary. If unset, uses 'tavora' on PATH,
                    else falls back to 'npx --yes @tavora/cli'.

Docs: https://docs.tavora.ai/code-first
`);
}
