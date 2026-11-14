# Open questions / assumptions

1. **Errors section vs. table.** A failed check already shows in its row (`Couldn't check` + reason).
   The Errors section repeats them so they're findable in a 60-row list. Drop it if that feels redundant.
2. **"Another wam is running"** has only copy, no dedicated mockup: same banner slot as the
   network error, error style, with **Try again**.
3. **Row Update while another update runs** is hidden, not disabled, to keep rows calm.
4. **Source column** shows CurseForge/GitHub; the raw ref (`curse:6124`) is a tooltip. Brief listed
   the ref itself; this reads better for players.
5. **Fix** has no confirm dialog (it reinstalls, then moves the broken folder to backups).
6. **Download counts** are abbreviated (2.1M, 310K); assumes wam sends raw numbers.
7. **Backups path** on Linux is the XDG data dir; mockups show the Windows path.
8. **Light theme** has tokens only, no mockups (per brief).
9. **Window title bar** is native and not drawn in the mockups.
10. **Sample data** beyond the brief (other addon names, versions, download counts) is placeholder.
11. **Update all while some rows failed a check** counts only addons with a known newer version.
12. **Keyboard**: Ctrl+1/2/3 for tabs, Ctrl+R refresh, Ctrl+F jumps to Find search. Confirm these
    don't clash with anything egui reserves.
13. **Spinner** is egui's built-in; the static arc in mockups stands for it.
