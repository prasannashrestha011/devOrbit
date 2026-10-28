export const repo = 'https://github.com/prasannashrestha011/devOrbit';

export const guideGroups = [
  { label: 'Get started', links: [
    { href: '/docs/', label: 'Introduction' },
    { href: '/docs/installation/', label: 'Installation' },
  ] },
  { label: 'Use DevOrbit', links: [
    { href: '/docs/workspaces/', label: 'Workspaces' },
    { href: '/docs/configuration/', label: 'Configuration' },
    { href: '/docs/services/', label: 'Services & terminals' },
    { href: '/docs/docker/', label: 'Docker integration' },
    { href: '/docs/traffic/', label: 'Traffic inspector' },
    { href: '/docs/troubleshooting/', label: 'Troubleshooting' },
  ] },
  { label: 'Go deeper', links: [
    { href: '/architecture/', label: 'Architecture' },
    { href: '/docs/api/', label: 'API reference' },
    { href: '/docs/contributing/', label: 'Contributing' },
  ] },
];

export const guides = guideGroups.flatMap(({ links }) => links);
