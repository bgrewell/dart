// Generates the documentation site's pages from the repository's canonical
// Markdown, so the site can never drift from what a reader sees on GitHub:
// the README becomes the guide pages, docs/*.md become the reference.
//
// Run automatically before dev/build via the package scripts.
import { mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const siteDir = dirname(dirname(fileURLToPath(import.meta.url)));
const repoRoot = dirname(siteDir);
const outDir = join(siteDir, "src", "content", "docs");

// README sections that become guide pages, in sidebar order. Sections not
// listed here stay README-only (they exist for GitHub readers, not the site).
const GUIDES = [
  { heading: "Install", slug: "install", description: "Install DART and validate your first suite." },
  { heading: "Your first suite", slug: "first-suite", description: "Nodes, tests, and evaluate — the whole loop in ten lines." },
  { heading: "Common things developers test", slug: "common-tasks", description: "Service smoke tests, config deployment, and clean-machine installs." },
  { heading: "What unit tests can't do", slug: "beyond-unit-tests", description: "Real reboots, convergence, firewall policy, drift, and performance gating." },
  { heading: "Running in CI", slug: "ci", description: "JUnit reports, validation, and parameterized suites in pipelines." },
  { heading: "How it works", slug: "how-it-works", description: "The phase order, and what happens when a run fails." },
];

// docs/*.md files that become reference pages, in sidebar order.
const REFERENCE = [
  { file: "node-types.md", slug: "node-types", description: "Every node type and option: local, Docker, Compose, LXD/Incus, and SSH." },
  { file: "tests.md", slug: "tests", description: "Every test type, plus retries, timeouts, skips, captures, variables, and tags." },
  { file: "evaluation.md", slug: "evaluation", description: "Every check available in an evaluate block." },
  { file: "steps.md", slug: "steps", description: "Every setup and teardown step type." },
  { file: "cli.md", slug: "cli", description: "Flags, exit codes, and report formats." },
];

const REPO_URL = "https://github.com/bgrewell/dart";

// rewriteLinks converts repository-relative links into site routes, and
// points anything that only exists in the repository at GitHub.
function rewriteLinks(markdown) {
  return markdown
    .replace(/\]\(docs\/([a-z-]+)\.md\)/g, "](/dart/reference/$1/)")
    .replace(/\]\(\.\.\/README\.md\)/g, "](/dart/)")
    .replace(/\]\(([a-z-]+)\.md\)/g, "](/dart/reference/$1/)")
    .replace(/\]\(docs\/reviews\/?\)/g, `](${REPO_URL}/tree/main/docs/reviews)`)
    .replace(/\]\(docs\/\)/g, "](/dart/reference/node-types/)")
    .replace(/\]\(examples\/?\)/g, `](${REPO_URL}/tree/main/examples)`)
    .replace(/\]\(LICENSE\)/g, `](${REPO_URL}/blob/main/LICENSE)`)
    .replace(/\]\(reviews\/?\)/g, `](${REPO_URL}/tree/main/docs/reviews)`);
}

// frontmatter escapes a title for YAML (several contain apostrophes).
function frontmatter({ title, description, editUrl }) {
  const escape = (text) => `"${text.replace(/"/g, '\\"')}"`;
  return [
    "---",
    `title: ${escape(title)}`,
    description ? `description: ${escape(description)}` : null,
    // Point "Edit page" at the canonical source, not at the generated file
    // in this directory, which is not committed
    editUrl ? `editUrl: ${escape(editUrl)}` : null,
    "---",
    "",
  ]
    .filter(Boolean)
    .join("\n");
}

// splitSections cuts Markdown into { heading, body } at level-2 headings,
// dropping the horizontal rules used as separators between them.
function splitSections(markdown) {
  const sections = [];
  let current = null;

  for (const line of markdown.split("\n")) {
    const match = /^##\s+(.+)$/.exec(line);
    if (match) {
      if (current) sections.push(current);
      current = { heading: match[1].trim(), lines: [] };
      continue;
    }
    if (current) current.lines.push(line);
  }
  if (current) sections.push(current);

  return sections.map((section) => ({
    heading: section.heading,
    body: section.lines.join("\n").replace(/^\s*---\s*$/gm, "").trim(),
  }));
}

// stripTitle removes a document's leading H1 (Starlight renders the title
// from frontmatter) and any immediate "back to README" breadcrumb.
function stripTitle(markdown) {
  return markdown
    .replace(/^#\s+.+$/m, "")
    .replace(/^\[← Back to the README\]\([^)]*\)$/m, "")
    .trim();
}

async function main() {
  await rm(outDir, { recursive: true, force: true });
  await mkdir(join(outDir, "guides"), { recursive: true });
  await mkdir(join(outDir, "reference"), { recursive: true });

  // Guides come from the README's sections
  const readme = await readFile(join(repoRoot, "README.md"), "utf8");
  const sections = splitSections(readme);
  for (const guide of GUIDES) {
    const section = sections.find((candidate) => candidate.heading === guide.heading);
    if (!section) {
      throw new Error(
        `README section "${guide.heading}" not found — update site/scripts/sync-docs.mjs when the README's structure changes`,
      );
    }
    await writeFile(
      join(outDir, "guides", `${guide.slug}.md`),
      frontmatter({
        title: guide.heading,
        description: guide.description,
        editUrl: `${REPO_URL}/edit/main/README.md`,
      }) +
        rewriteLinks(section.body) +
        "\n",
    );
  }

  // Reference pages come from docs/
  const docsDir = join(repoRoot, "docs");
  const present = new Set(await readdir(docsDir));
  for (const page of REFERENCE) {
    if (!present.has(page.file)) {
      throw new Error(`docs/${page.file} is missing — update site/scripts/sync-docs.mjs`);
    }
    const source = await readFile(join(docsDir, page.file), "utf8");
    const title = /^#\s+(.+)$/m.exec(source)?.[1]?.trim() ?? page.slug;
    await writeFile(
      join(outDir, "reference", `${page.slug}.md`),
      frontmatter({
        title,
        description: page.description,
        editUrl: `${REPO_URL}/edit/main/docs/${page.file}`,
      }) +
        rewriteLinks(stripTitle(source)) +
        "\n",
    );
  }

  // The landing page is authored for the site rather than synced: it is a
  // hero, not a document.
  await writeFile(join(outDir, "index.mdx"), await landingPage(readme));

  console.log(
    `synced ${GUIDES.length} guides and ${REFERENCE.length} reference pages`,
  );
}

// landingPage builds the splash hero, reusing the README's opening pitch and
// its first example so the two cannot disagree.
async function landingPage(readme) {
  const pitch = readme
    .split("\n## ")[0]
    .split("```")[0]
    .replace(/^#\s+DART\s*$/m, "")
    .replace(/^\*\*Test the things unit tests can't reach\.\*\*\s*$/m, "")
    .replace(/^>.*$/gm, "")
    .trim();

  const example = readme.split("```yaml")[1]?.split("```")[0]?.trim() ?? "";

  return `---
title: DART
description: Test the things unit tests can't reach — real environments, real reboots, real networks.
template: splash
head:
  - tag: title
    content: "DART — test the things unit tests can't reach"
hero:
  tagline: Test the things unit tests can't reach.
  actions:
    - text: Get started
      link: /dart/guides/install/
      icon: right-arrow
      variant: primary
    - text: View on GitHub
      link: ${REPO_URL}
      icon: external
      variant: minimal
---

import { Card, CardGrid } from '@astrojs/starlight/components';

${pitch}

\`\`\`yaml
${example}
\`\`\`

<CardGrid stagger>
  <Card title="Real environments" icon="puzzle">
    Containers, VMs, and remote hosts — built, tested, and torn down by the
    suite itself. Cleanup runs even when a test fails.
  </Card>
  <Card title="Assertions unit tests can't make" icon="approve-check">
    Survive a real reboot, wait for a cluster to converge, prove a firewall
    rule blocks a port, catch config drift across a fleet.
  </Card>
  <Card title="Built for CI" icon="rocket">
    JUnit and JSON reports, meaningful exit codes, tag filtering, and
    \`--check\` to validate a suite without touching infrastructure.
  </Card>
  <Card title="Declarative YAML" icon="document">
    Nodes describe where things run, tests describe what must be true, and
    \`evaluate\` defines what true means.
  </Card>
</CardGrid>
`;
}

await main();
