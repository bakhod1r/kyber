import { defineConfig } from 'vitepress'

const repo = 'https://github.com/bakhod1r/kyber'

export default defineConfig({
  title: 'Kyber',
  description: 'Open-source, self-hosted issue & project tracker — the core of Jira, in one Go binary.',
  base: '/kyber/',
  lang: 'en-US',
  cleanUrls: true,
  lastUpdated: false,
  themeConfig: {
    nav: [
      { text: 'Getting started', link: '/getting-started/' },
      { text: 'Guides', link: '/guide/projects-and-boards' },
      { text: 'Operations', link: '/operations/helm' },
      { text: 'Architecture', link: '/architecture/' },
    ],
    sidebar: [
      {
        text: 'Getting started',
        items: [
          { text: 'Install & run', link: '/getting-started/' },
          { text: 'Configuration reference', link: '/getting-started/configuration' },
        ],
      },
      {
        text: 'Guides',
        items: [
          { text: 'Projects & boards', link: '/guide/projects-and-boards' },
          { text: 'JQL search', link: '/guide/jql' },
          { text: 'Workflows', link: '/guide/workflows' },
          { text: 'Reports', link: '/guide/charts' },
          { text: 'Webhooks & GitHub', link: '/guide/webhooks' },
          { text: 'AI assistants (MCP)', link: '/guide/ai-assistants' },
          { text: 'Single sign-on & SCIM', link: '/guide/sso' },
          { text: 'Live updates (MQTT)', link: '/guide/live-updates' },
        ],
      },
      {
        text: 'Operations',
        items: [
          { text: 'Helm chart', link: '/operations/helm' },
          { text: 'Backup & restore', link: '/operations/backup' },
          { text: 'Upgrading', link: '/operations/upgrade' },
          { text: 'Rate limiting', link: '/operations/rate-limiting' },
          { text: 'Security notes', link: '/operations/security' },
        ],
      },
      {
        text: 'Architecture',
        items: [
          { text: 'DDD contexts', link: '/architecture/' },
          { text: 'Decision records (ADRs)', link: '/architecture/adr' },
        ],
      },
    ],
    socialLinks: [{ icon: 'github', link: repo }],
    editLink: { pattern: `${repo}/edit/main/site/:path`, text: 'Edit this page on GitHub' },
    search: { provider: 'local' },
    footer: { message: 'Released under the MIT License.' },
  },
})
