DevOrbit Workspace UI Refinement Prompt
This is a design/refinement context prompt, not an implementation request yet.
Analyze the current DevOrbit workspace UI and propose a UI refinement plan based on the desired reference.
Goal
Refine the workspace details screen of DevOrbit so it feels closer to the desired polished product UI, while preserving the current DevOrbit theme system and visual identity.
The current UI already has the base dark theme and layout structure.
The desired reference shows a much more polished visual quality, especially in:
- buttons
- icon style
- spacing
- card polish
- status pills
- hierarchy
- table/service row styling
- overall visual consistency
Important constraints
1. Preserve the current DevOrbit theme approach
   - Do not hardcode a one-off redesign that breaks future theme support.
   - We want to add theme settings later, so the refinement must remain compatible with a theme/token-based system.
   - Keep the current DevOrbit dark styling direction as the base.
2. Do not copy the second reference literally
   - It is a visual inspiration/reference only.
   - We are not recreating the second screen exactly.
   - We are adapting the current DevOrbit screen toward a more polished version.
3. Do not include the tab section from the second image
   - The reference has a tab row like:
     - Services
     - Environment
     - Ports
     - Logs
     - Settings
   - This tab section is not required for this refinement.
   - Focus only on improving the current workspace overview screen and services list experience.
4. Focus especially on correcting these areas
   - top action buttons
   - icon consistency
   - service row actions
   - section spacing and padding
   - card appearance
   - typography hierarchy
   - status badge styling
   - workspace summary presentation
   - “Add service” button treatment
   - workspace action affordances
Current screen characteristics
The current workspace screen already contains:
- workspace breadcrumb / title
- workspace overview section
- summary cards
  - total services
  - running
  - workspace state
- top action buttons
  - Terminals
  - Workspace YAML
  - Edit workspace
- workspace action buttons
  - Start all
  - Stop all
- services section
- services table/list
- row status
- row actions
- delete workspace action
These should remain conceptually present, but the UI should be visually improved.
Desired direction
Use the second reference as inspiration for the following qualities:
1. Better visual hierarchy
Make the workspace screen feel more premium and productized.
Improve:
- heading scale
- subtitle/meta text treatment
- information grouping
- section separation
- scanability
2. Better buttons
The current buttons should be redesigned to feel more intentional and polished.
Improve:
- primary vs secondary distinction
- button height and padding
- icon alignment
- hover/focus/disabled states
- consistency across all actions
Pay attention especially to:
- Start all
- Stop all
- Terminals
- Workspace YAML
- Edit workspace
- Add service
- row action buttons
3. Better icon language
The icon system should feel unified.
Fix inconsistencies in:
- icon size
- icon stroke weight
- icon placement
- relation between icon and label
- service row action icons
- summary card icons
The goal is to make the interface feel like one coherent system.
4. Better summary cards
The three overview cards should feel more polished and useful.
Refine:
- internal spacing
- label styling
- value emphasis
- icon presentation
- badge/status appearance
- card surface treatment
5. Better services section
The services table/list should look more modern and readable.
Improve:
- row height
- spacing
- action alignment
- command text readability
- status pill styling
- service name / secondary text hierarchy
- action button consistency
- search field styling
6. Better status styling
Status badges like:
- Running
- Stopped
- Idle
should be more visually intentional and easier to scan.
Improve:
- color usage
- shape
- spacing
- typography
- icon/dot treatment
7. Better overall polish
Refine:
- border treatment
- panel depth
- spacing rhythm
- consistent corner radius
- subtle emphasis
- visual density
- empty/quiet areas
The result should still feel like DevOrbit, just much more refined.
What should remain unchanged conceptually
- current workspace page purpose
- current feature set
- current screen structure at a high level
- current dark theme direction
- support for future theme settings
- existing functionality
This is primarily a visual and UX refinement, not a feature redesign.
What is explicitly not required
- No tab navigation from the second reference
- No full dashboard redesign
- No new feature invention unless necessary for UI consistency
- No architecture rewrite
- No implementation yet unless explicitly requested later
Agent task
Analyze the current workspace screen and produce:
1. UI refinement analysis
Explain:
- what currently feels weak or inconsistent
- what should be improved
- what should be preserved
2. Design direction
Describe:
- refined component styling direction
- button system improvements
- icon system improvements
- card improvements
- services table/list improvements
- status badge improvements
3. Theme-safe design strategy
Explain how the redesign should remain compatible with future theme settings.
4. Proposed component-level changes
Break down recommended changes for:
- page header
- action buttons
- summary cards
- service list/table
- row actions
- search input
- delete workspace affordance
5. Implementation plan
Provide a phased frontend implementation plan with small steps.
6. Optional
If helpful, propose:
- design tokens to normalize spacing/radius/shadows/colors
- reusable button variants
- reusable badge variants
- reusable card variants
Important final instruction
Do not implement yet.
First, analyze the current screen and prepare a clear UI refinement plan based on the desired visual direction, while keeping DevOrbit’s current theme approach intact and excluding the tab section from the second reference.