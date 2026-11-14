# Components

Logical px. Colours are token names from `tokens.json` (dark). States: default / hover / focused /
pressed / disabled / busy. Focus = 2 px `focus_ring` outline, 2 px offset (inset on rows).
Disabled = 0.45 opacity, no hover.

## Buttons

Height 30, padding 0 12, radius 4, 1 px border, Inter 13/500, icon 16 + gap 6.

| Variant | default | hover | pressed | busy |
|---|---|---|---|---|
| Primary (Update all, Track, Choose folder) | fill `accent_fill`, text #FFF | `primary_hover` | `primary_pressed` | spinner replaces icon, label "Updating…" |
| Secondary (Refresh, Browse, Cancel) | fill `control`, border `control_border`, text `text` | `secondary_hover` | `secondary_pressed` | spinner, 0.8 opacity |
| Ghost (Activity toggle) | transparent, text `text_secondary` | `control` | `secondary_pressed` | — |
| Danger (Remove in dialog only) | fill `danger_fill`, text #FFF | `danger_hover` | `danger_pressed` | — |
| Small (row Update, Track…, Fix) | as Secondary, height 26, padding 0 10, 12/500 | | | |
| Icon (row Remove) | 26×26, transparent, icon `muted_text` | fill `control`, icon `text` | | |

One primary button per view.

## Table row

Height 32, cells padding 0 12, alternating `background` / `row_stripe`. Hover `row_hover`.
Selected (keyboard cursor) `selection`. Focused: inset 2 px ring. Name 13/500, versions mono 12,
Source `muted_text`. Long text truncates with "…", full text in tooltip. Guessed version: `341 ~`,
the `~` in `muted_text`, tooltip "Estimated: installed without wam"; one footer line under the
table explains it once.

Header 30, fill `table_header`, 12/500 `muted_text`, 1 px `border` below.

## Status (in-row)

Icon 16 + label 13/500 in the state colour + optional detail 13/400 `muted_text`, truncated.

| State | icon | colour |
|---|---|---|
| Up to date | check | ok |
| Update available | update (arrow up) | accent_text |
| Couldn't check | alert | error |
| Checking… / Updating… | spinner | muted / accent_text |
| Waiting | clock | muted |
| Updated (detail "341 → 346") | check | ok |
| Failed (detail = reason) | alert | error |
| Skipped / Not checked | skip | muted |

## Text field

Height 32, padding 0 10, radius 4, fill `field_background`, border `control_border`.
Hover border #4A515A. Focused border `progress_fill` + 2 px ring rgba(76,141,240,.35).
Error border `field_error_border` + error line under it (12, `error`, alert icon). Label above,
12/500 `text_secondary`; help below, 12 `muted_text`. Token field is masked with an eye toggle.

## Dialog

Width 440 (Remove) / 480 (Track), padding 20, gap 14, radius 8, fill `raised_dialog`, 1 px #343A41,
shadow 0 12 32 rgba(0,0,0,.5), scrim `scrim`. Title 15/600. Folder list: mono 12 in a `background`
box. Buttons right-aligned, Cancel first. Escape = Cancel, Enter = primary. Focus starts in the
field (Track) or on Cancel (Remove).

## Section header (Not tracked / Broken / Errors)

Height 32, fill `panel`, chevron 16, title 13/600, count pill (18 high, radius 9, 11/600), hint
13/400 `muted_text` truncated. Pills: Not tracked #2A2F35/#C9CED4, Broken #3A2A14/#F0C979, Errors
#3A1F1D/#F4A79F. Hidden entirely when its count is 0. Expanded list rows 36, indent 36, at most
3 visible (108 px) then scrolls, so a long list never pushes the table off screen. One section open
at a time.

## Banner

Full content width, padding 8 12, radius 6, icon + one sentence. Warning: `banner_warning_*`.
Error: `banner_error_*`.

## Log entry

Row padding 3 16, 12 px: time (mono, `faint_text`), level (60 wide, 500, Info muted / Warning
warning / Error error), message `text_secondary`.

## Progress

Bar 4 high, radius 2, track `progress_track`, fill `progress_fill`, max width 420, under the
summary line. Spinner 16 (20 in empty states), `accent_text`.

## Empty state

Centered, max width 380: 44×44 icon tile (`control`, radius 8), title 16/600, one sentence muted,
up to two buttons.
