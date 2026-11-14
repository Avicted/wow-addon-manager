# Flows

## Update all (with a failure and a cancel)
1. Addons shows `2 updates available`; **Update all (2)** is the primary button.
2. Click. Tabs, Refresh, row buttons and section actions disable. If the game is running, the
   warning banner appears above the table (wam sends a `log` warning first).
3. Toolbar becomes `Updating 1 of 2 · Auctionator` + progress bar; **Cancel** replaces the buttons.
   Rows to update show `Waiting`; the current one `Updating…` with a spinner.
4. Each `progress` event updates its row: `Updated 341 → 346` or `Failed` + reason
   ("A file is in use…"). A failure does not stop the run; the old version stays (rollback).
5. Cancel: button turns `Cancelling…` (disabled), summary `Stopping after {addon}…`; the GUI writes
   `cancel` to wam's stdin. Remaining rows switch to `Skipped`.
6. `result` arrives: controls re-enable, bottom strip shows `Updated 1 addon, 1 failed` (or
   `Stopped: 1 updated, 1 skipped`), status reloads. Failed rows keep `Update available` so they
   can be retried.

## Install from search
1. Find addons → type `map` → Enter or **Search**. Results area shows the spinner.
2. Results list; installed ones show `Installed`, web-only ones `Open page ↗`.
3. **Install** on a row: everything disables, the row button shows a spinner `Installing…`.
4. Success: row turns `Installed`, strip `Installed Mapster 1.2.3`, the Addons count goes up.
   No build for this game: strip + row error `No version of this addon works with this game.`
5. By reference: paste into "Install from a link or name" → **Install**, same feedback under the field.

## Track a folder
1. Addons → expand **Not tracked** → **Track…** on `Journalator (+3 folders)`.
2. Dialog lists the folders; focus is in the reference field.
3. Paste link → **Track** (Enter). Button busy while wam checks.
4. Success: dialog closes, the group leaves Not tracked, a row appears in the table with a guessed
   (`~`) version. Bad reference: inline field error, dialog stays open.

## Remove
1. Trash icon on a row → Remove dialog with folder names and backups path; focus on **Cancel**.
2. **Remove** → dialog closes, row disappears, strip `Removed Auctionator. Moved to backups.`

## Fix a broken folder
1. **Broken** → **Fix** on `WeakAuras` (source known): runs install, then removes the broken folder.
2. Unknown source: action is **Track…**; after tracking, **Fix** becomes available.

## First run
1. App starts, wam reports the AddOns folder is missing → the Addons tab shows the first-run card
   instead of the table (Find addons is disabled; Settings stays open).
2. **Choose folder…** opens the native picker (`rfd`).
3. Valid folder → saved, status loads (Loading state) → Addons. Invalid → error line under the
   path in the card, picker can be reopened.
