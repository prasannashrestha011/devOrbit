# DevOrbit desktop theme

DevOrbit uses charcoal surfaces and orange accents. Compact toolbars, thin dividers, and clear selection markers help users scan services, logs, terminals, and HTTP traffic. The palette and empty-state treatment draw on the supplied Burp Suite screenshot.

## Current color tokens

| Role | CSS token | Value |
| --- | --- | --- |
| App canvas | `--surface-app` | `#323334` |
| Workspace sidebar | `--surface-sidebar` | `#2B2B2B` |
| Panel and toolbar | `--surface-panel` | `#383839` |
| Raised control | `--surface-raised` | `#424244` |
| Inset field and data surface | `--surface-inset` | `#29292A` |
| Terminal canvas | `--surface-terminal` | `#242424` |
| Hover surface | `--surface-hover` | `#4C4C4E` |
| Panel border | `--border-panel` | `#525255` |
| Quiet divider | `--border-soft` | `#454547` |
| Input boundary | `--border-control` | `#78787B` |
| Hover border | `--border-hover` | `#858589` |
| Primary text | `--text-main` | `#F2F2F3` |
| Secondary text | `--text-secondary` | `#CBCBCF` |
| Muted text | `--text-muted` | `#AEAFB3` |
| Text on filled orange | `--text-on-accent` | `#1B100B` |
| Selection marker | `--accent` | `#DB612F` |
| Primary button fill | `--accent-fill` | `#DB612F` |
| Accent hover and highlighted text | `--accent-hover` | `#FF9C72` |
| Selected surface | `--accent-soft` | `#50382F` |
| Keyboard focus | `--focus-ring` | `#FFB28E` |
| Running foreground and fill | `--success` / `--success-soft` | `#46D98C` / `#19392D` |
| Transition or stopped foreground and fill | `--warning` / `--warning-soft` | `#F4BD46` / `#3B3220` |
| Failed foreground and fill | `--danger` / `--danger-soft` | `#ED8D98` / `#45262C` |
| Idle foreground and fill | `--neutral-text` / `--neutral-soft` | `#DDDDE0` / `#464649` |
| Empty-state icon and fill | `--illustration` / `--illustration-soft` | `#87B9E9` / `#293D50` |

The terminal reads its canvas, text, cursor, and selection colors from the shared desktop tokens. The YAML and HTTP viewers use separate syntax colors to distinguish keys, strings, numbers, and punctuation. See the [desktop stylesheet](../../desktop/src/styles.css) and [TerminalPane.tsx](../../desktop/src/TerminalPane.tsx).

## Typography and layout

- Use `"DM Sans", Inter, system-ui, sans-serif` for interface text. Workspace titles scale from 32 to 39 px, section titles are about 22 px, and controls and data labels typically use 12–14 px.
- Use `"JetBrains Mono", monospace` for technical content such as paths, commands, and HTTP message bodies. The interactive terminal uses a 14 px configurable monospace font.
- The workspace sidebar is 252 px wide, narrows to 210 px at 1100 px, and collapses to a 68 px icon rail at 800 px. The desktop stylesheet sets a 700 px minimum body width.
- Separate chrome, panels, and inset data areas with surface tones and 1 px borders. Workspace summary and service panels use 12 px corners. Standard buttons use 6 px corners and search fields use 8 px corners.

## Component patterns

| Component | Treatment |
| --- | --- |
| Workspace navigation | Use an accent-tinted surface and a 3 px leading orange marker for the selected workspace. |
| Detail tabs | Use an orange underline and stronger text for the active tab. Keep the focus outline visible independently of selection. |
| Buttons | Use orange with dark text for the main action and charcoal for secondary actions. Standard buttons are at least 40 px tall, with compact controls in toolbars. Use the danger variant for destructive actions. |
| Forms and search | Place controls on inset surfaces with visible borders. Use the 2 px focus outline for keyboard navigation. |
| Status pills | Pair a colored dot with a readable state label. Running uses green, transitions and stopped states use amber, failures use rose, idle uses gray, and exited uses lavender. |
| Data tables | Keep headers and rows compact. Traffic history uses sticky headers, tabular numbers, and an accent-tinted selected row. |
| Empty states | Use a compact icon, a short heading, and a sentence explaining what will appear. Offer a primary action when the user can resolve the empty state. |

## Screenshot reference

The values below were sampled from the supplied Burp Suite screenshot. DevOrbit uses the reference canvas, chrome, secondary button, divider, and orange accent colors. Its own orbit mark anchors the centered workspace empty state.

| Reference element | Sampled color |
| --- | --- |
| Main canvas, lower tab strip, and status bar | `#323334` |
| Top tab strip | `#2B2B2B` |
| Dividers | `#525255` |
| Secondary button | `#424244` |
| Active underline and primary button | `#DB612F` |
| Empty-state illustration | `#26649D` |

