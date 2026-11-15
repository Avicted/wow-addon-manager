# wam desktop UI — handover

Mockups: the "wam desktop UI" Design canvas (one artboard per screen and state, plus the UI kit).
This folder holds the text half: `tokens.json`, `components.md`, `copy.md`, `flows.md`,
`open-questions.md`, `icons/`.

## Rationale

A calm utility next to a game launcher. Most people open it, see "2 updates available", press
**Update all** and close it. So the Addons screen leads with one summary line and one primary
button, and anything rare (untracked, broken, failed checks) sits in collapsed sections under the
table with a count pill. Dark first, with a Warcraft look: leather browns, parchment text, gold accents and frames, a crimson
primary button and Cinzel capitals for titles and tabs. Ornament stays light: gold lines, a bevel,
a double-framed dialog, no textures. The HTML mockups in `mockups-src/` still show the earlier
neutral grey and blue palette; `tokens.json` and the code are current.

Everything maps onto cheap egui parts: `TopBottomPanel` (top bar, toolbar, bottom strip),
`CentralPanel`, `egui_extras::TableBuilder` (striped, 32 px rows), `CollapsingHeader`-style
sections, `Window` modals with a scrim, `ProgressBar`, `Spinner`. No gradients, blur or animation
beyond the spinner.

## Navigation: top tabs, not a sidebar

- Three destinations only (Addons, Find addons, Settings). A sidebar costs 160–200 px of width,
  and at the 720 px minimum the table needs every column.
- Tabs live in the 44 px top bar next to the app name: `ui.selectable_label` in a horizontal row,
  2 px gold underline with a small diamond on the active one. Keyboard: Ctrl+1/2/3, Tab order follows reading order.
- The Addons tab carries the addon count.
- While an action runs, the tabs are disabled (opacity 0.45) — one action at a time.

## Activity: a bottom strip, expandable

A 30 px strip always shows the last outcome ("Updated 2 addons, 1 failed"). "Activity (n)"
expands a 132 px log panel above it. Chosen over toasts: toasts need timers and animation, vanish
before a player alt-tabs back from the game, and stack badly. Chosen over a permanent panel: the
log is usually uninteresting. Warnings that matter *now* (game running, network down) are banners
above the table, not log lines.

## Layout (logical px)

Top bar 44 · toolbar ~50 · 10 gap · table (fills) · 10 gap · sections (collapsed 32 each) ·
10 gap · bottom strip 30. Content padding 16. Table columns at 960:
Addon (fill) · Installed 128 · Latest 152 · Source 112 · Status 220 · actions 92.
Below 820 px wide the Source column is dropped (it stays in the row tooltip) and widths become
104 / 128 / 168 / 72.

## Fonts

Cinzel Bold for the app name, tabs and section and dialog titles; Inter (Regular, Medium, SemiBold)
for everything else; JetBrains Mono Regular for versions. All SIL OFL 1.1; bundle the TTFs
with `egui::FontDefinitions`. Mono is only for versions, paths and refs, so long strings like
`v12.9.0-beta3-142-gabc1234` line up and truncate predictably.

## Scaling and keyboard

All sizes are logical px, so 125 % / 150 % come from `pixels_per_point`. Every control is a real
button or field; focus shows a 2 px `#FFD24A` ring. Escape closes dialogs; Enter confirms.
Status never relies on colour alone: icon + word in every state.

## Light theme

Swap `colors.dark` for `colors.light` in `tokens.json`; no layout changes.
