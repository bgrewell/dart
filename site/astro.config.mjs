import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// The site is published as a GitHub project page, so every route is served
// under /dart.
export default defineConfig({
  site: "https://bgrewell.github.io",
  base: "/dart",
  trailingSlash: "always",
  integrations: [
    starlight({
      title: "DART",
      description:
        "Test the things unit tests can't reach — real environments, real reboots, real networks.",
      social: [
        {
          icon: "github",
          label: "GitHub",
          href: "https://github.com/bgrewell/dart",
        },
      ],
      // Each page carries its own editUrl (set by the sync script) pointing
      // at the canonical Markdown rather than the generated copy
      lastUpdated: true,
      customCss: ["./src/styles/custom.css"],
      sidebar: [
        {
          label: "Guides",
          items: [
            { label: "Install", link: "/guides/install/" },
            { label: "Your first suite", link: "/guides/first-suite/" },
            { label: "Common tasks", link: "/guides/common-tasks/" },
            { label: "Beyond unit tests", link: "/guides/beyond-unit-tests/" },
            { label: "Running in CI", link: "/guides/ci/" },
            { label: "How it works", link: "/guides/how-it-works/" },
          ],
        },
        {
          label: "Reference",
          items: [
            { label: "Node types", link: "/reference/node-types/" },
            { label: "Test types", link: "/reference/tests/" },
            { label: "Evaluation checks", link: "/reference/evaluation/" },
            { label: "Steps", link: "/reference/steps/" },
            { label: "Command line", link: "/reference/cli/" },
          ],
        },
      ],
    }),
  ],
});
