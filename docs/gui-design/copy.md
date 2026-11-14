# Copy

Plain English, sentence case, no terminal commands. `{x}` = value. Curly quotes and "…" as shown.

## Top bar
- App name: `wam`
- Tabs: `Addons {count}` · `Find addons` · `Settings`

## Addons — summary line
- `{n} updates available` / `1 update available` · muted: `· {total} addons` (+ ` · {k} couldn't be checked`)
- `All {total} addons up to date`
- Loading: `Checking for updates…` · `{done} of {total} addons`
- Updating: `Updating {i} of {n}` · `· {addon}`
- Cancelling: `Stopping after {addon}…` · `· {k} addons will be skipped`
- Done: `Updated {a} addons` / `Updated {a} addons, {f} failed` / `Stopped: {a} updated, {s} skipped`
- Check failed: `Couldn't check for updates` · `· {total} addons installed`

## Addons — buttons
`Refresh` · `Try again` (after a failed check) · `Update all ({n})` · `Cancel` · `Cancelling…` · row `Update` · row icon `Remove` (tooltip and screen-reader label `Remove {addon}`)
- Cancel hint: `Cancel lets the current addon finish and skips the rest.`

## Addons — table
- Headers: `Addon` · `Installed` · `Latest` · `Source` · `Status`
- Sources: `CurseForge` · `GitHub` (tooltip shows the reference)
- Statuses: `Up to date` · `Update available` · `Couldn't check` · `Checking…` · `Waiting` · `Updating…` · `Updated` · `Failed` · `Skipped` · `Not checked`
- Missing latest version: `—`
- Guessed footnote: `~ Estimated version: this addon was installed without wam. It becomes exact after its first update.`
- Guessed tooltip: `Estimated: installed without wam`

## Addons — sections
- `Not tracked` — `Folders wam can't identify, so it doesn't check them for updates.` · item `{name} (+{k} folders)` · action `Track…`
- `Broken` — `Folders the game can't load.` · action `Fix` (source known) or `Track…`
  - `Unpacked one folder too deep, so the game can't load it.`
  - `Has the version in its folder name, so the game can't load it.`
  - `Source unknown. Tell wam where it comes from, then fix it.`
- `Errors` — `Update checks that failed.`

## Banners
- Game running: `World of Warcraft is running. Updated addons load after you type /reload in the game, or restart it.`
- Network: `Can't reach CurseForge or GitHub. Check your internet connection and try again. Your installed addons are fine.`
- Already running: `wam is already running in another window. Close it, then try again.`
- GitHub limit: `GitHub's limit was reached. GitHub addons are checked again at {time}, or add a token in Settings.`

## Error messages (row detail / log)
- `CurseForge isn't responding (HTTP {code})`
- `GitHub limit reached. Checks resume at {time}, or add a token in Settings.`
- `A file is in use. Close the game and try again.` (update failed, rolled back)
- `No version of this addon works with this game.` (install, no matching build)
- `Couldn't find an addon at that address.` (bad reference)
- `Not enough disk space.` · `wam isn't allowed to write to the AddOns folder.`
- Generic fallback: `Something went wrong: {message}`

## Empty state
- `No addons yet` — `Your AddOns folder is empty. Find an addon to install, or check that wam is looking in the right folder.` · `Find addons` · `Check folder`

## First run / folder not found
- `Where is your AddOns folder?`
- `wam looks after the addons in your game's AddOns folder, but it couldn't find that folder here:` {path}
- `Choose the AddOns folder inside your game's Interface folder. If there is no AddOns folder yet, start the game once, then come back.`
- `Choose folder…`

## Find addons
- Search placeholder: `Search by addon name, for example Questie or bags` · button `Search`
- Idle: `Find an addon` — `Search CurseForge by name. You can also paste a link below if you already know where the addon lives.`
- Searching: `Searching CurseForge for "{q}"…`
- Results: `{n} results for "{q}"` · headers `Addon` · `Downloads` · actions `Install` / `Installed` / `Open page` (tooltip `Can't be installed automatically. Opens the addon's web page.`)
- Missing downloads: `—` (tooltip `Download count not available`)
- Note: `Some addons have no version for this game. If so, wam tells you when you install.`
- None: `Nothing found for "{q}"` — `Try a shorter name or check the spelling. If the addon is on GitHub, paste its address below.`
- Down: `Search isn't available right now` — `The search service didn't answer. You can still install an addon from a link below, or try again in a few minutes.` · `Try again`
- By reference: `Install from a link or name` · placeholder `Paste a CurseForge or GitHub link, or type a name like details` · `Install` · `Also works: curse:61284 or github:Questie/Questie`
- Installing: `Installing {name}…` · done `Installed {name} {version}`

## Settings
- `AddOns folder` · `Browse…` · ok `Folder found. {n} addons, {k} not tracked.` · bad `This folder doesn't exist. Choose the AddOns folder inside your game's Interface folder.`
- `Unidentified folders` — `If you added addons by hand, wam can look at those folders again to work out where they came from.` · `Rescan unidentified folders` · done `Rescan done. {k} folders identified.`
- `GitHub token (optional)` — `GitHub allows 60 update checks an hour without a token. If many of your addons come from GitHub, a token raises that limit. It needs no permissions.` · placeholder `Paste a token` · `Show token` · `Create a token on github.com`
- `Backups` — `Before an addon is removed or replaced, wam saves it here as a zip, so you can always put it back.` · `Open folder`

## About
- Own tab (Ctrl+4): logo · `wam` · `v{version}` · `Keeps your World of Warcraft Forever 1.60 addons up to date.` · `Made by Victor Anderssén` · links `Source code on GitHub`, `github.com/Avicted` · `MIT licence`

## Dialogs
- Track: `Where does {name} come from?` — `wam found these folders but doesn't know which addon they belong to, so it can't check them for updates.` · label `Addon page or name` · help `Paste a CurseForge link, type a CurseForge name such as details, or a GitHub project such as github:Questie/Questie.` · `Cancel` · `Track`
- Remove: `Remove {name}?` — `These folders are saved as a zip in the backups folder first, so you can put them back later.` · `Backups folder` {path} · `Your in-game settings for this addon are never touched.` · `Cancel` · `Remove`

## Bottom strip
- `Last check {time} · {summary}` · `Activity ({n})`
- Levels: `Info` · `Warning` · `Error`
