//! What the app knows and how wam's events change it. Drawing is in addons.rs, find.rs and
//! settings.rs; this file has no layout so it can be tested without a window.

use std::{
    collections::VecDeque,
    path::{Path, PathBuf},
};

use chrono::Local;
use eframe::egui;
use serde::{Deserialize, Serialize};

use crate::{
    copy,
    wam::{self, ErrInfo, Event, Item, Msg, Row, SearchData, StatusData},
};

/// What the app remembers between runs.
#[derive(Default, Serialize, Deserialize)]
#[serde(default)]
pub struct Settings {
    /// None until the player picks a folder: wam then uses WAM_ADDONS_DIR or its default.
    pub addons_dir: Option<String>,
    pub github_token: String,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Tab {
    Addons,
    Find,
    Settings,
    About,
}

/// One wam command.
#[derive(Clone, Debug, PartialEq)]
pub enum Cmd {
    Status {
        rescan: bool,
    },
    /// Every addon with an update when `names` is empty.
    Update {
        names: Vec<String>,
    },
    Install {
        reference: String,
        from: Origin,
    },
    Remove {
        name: String,
    },
    Track {
        folder: String,
        reference: String,
    },
    Search {
        query: String,
    },
}

/// Where an install was started.
#[derive(Clone, Debug, PartialEq)]
pub enum Origin {
    Search {
        title: String,
    },
    Link,
    /// Fix in the Broken section.
    Fix,
}

impl Cmd {
    fn args(&self) -> Vec<String> {
        let v = |a: &[&str]| a.iter().map(|s| s.to_string()).collect::<Vec<_>>();
        match self {
            Cmd::Status { rescan: true } => v(&["-rescan", "status"]),
            Cmd::Status { rescan: false } => v(&["status"]),
            Cmd::Update { names } => [v(&["update"]), names.clone()].concat(),
            Cmd::Install { reference, .. } => v(&["install", reference]),
            Cmd::Remove { name } => v(&["remove", name]),
            Cmd::Track { folder, reference } => v(&["track", folder, reference]),
            Cmd::Search { query } => v(&["search", query]),
        }
    }
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Tone {
    Muted,
    Ok,
    Warning,
    Error,
}

pub struct LogLine {
    pub time: String,
    pub tone: Tone,
    pub message: String,
}

/// Problems shown as a banner above the table.
#[derive(Clone, Debug, PartialEq)]
pub enum Problem {
    Network,
    Locked,
    RateLimit(String),
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum Section {
    Untracked,
    Broken,
    Errors,
}

pub enum Dialog {
    Remove {
        name: String,
        folders: Vec<String>,
    },
    Track {
        name: String,
        folders: Vec<String>,
        input: String,
        error: Option<String>,
        focus: bool,
    },
}

pub enum FindPhase {
    Idle,
    Searching(String),
    Results(SearchData),
    Nothing(String),
    Down,
}

pub struct Find {
    pub query: String,
    pub phase: FindPhase,
    pub link: String,
    pub link_note: Option<(String, Tone)>,
    /// The ref being installed from this tab.
    pub installing: Option<String>,
}

struct Running {
    cmd: Cmd,
    job: wam::Job,
    cancelling: bool,
    result: Option<(bool, Option<serde_json::Value>, Option<ErrInfo>)>,
}

pub struct App {
    pub settings: Settings,
    bin: Result<PathBuf, String>,
    /// Woken when wam sends something; tests swap in their harness's context.
    pub ctx: egui::Context,
    pub tab: Tab,
    running: Option<Running>,
    queue: VecDeque<Cmd>,

    pub status: Option<StatusData>,
    /// The table: the last status, updated live while a check runs.
    pub rows: Vec<Row>,
    /// The running update's addons, or the ones the last update changed.
    pub items: Vec<Item>,
    /// Update checks done and total, while they run.
    pub check: Option<(usize, usize)>,
    pub wow_running: bool,
    pub problem: Option<Problem>,
    /// The AddOns folder wam couldn't find; the first-run card replaces the table.
    pub first_run: Option<String>,
    pub open_section: Option<Section>,
    pub dialog: Option<Dialog>,

    pub outcome: (String, Tone),
    pub log: Vec<LogLine>,
    pub log_open: bool,

    pub find: Find,
    pub focus_search: bool,
    pub token_visible: bool,
    pub settings_note: Option<(String, Tone)>,
    pub folder_error: Option<String>,
    untracked_before_rescan: Option<usize>,
    /// The running or queued status only refreshes the table after an action, so the strip
    /// keeps saying what the action did.
    quiet_reload: bool,
}

impl App {
    /// `bin` is the unpacked wam, or why it couldn't be unpacked.
    pub fn new(ctx: egui::Context, settings: Settings, bin: Result<PathBuf, String>) -> Self {
        let mut app = App {
            settings,
            bin,
            ctx,
            tab: Tab::Addons,
            running: None,
            queue: VecDeque::new(),
            status: None,
            rows: Vec::new(),
            items: Vec::new(),
            check: None,
            wow_running: false,
            problem: None,
            first_run: None,
            open_section: None,
            dialog: None,
            outcome: (String::new(), Tone::Muted),
            log: Vec::new(),
            log_open: false,
            find: Find {
                query: String::new(),
                phase: FindPhase::Idle,
                link: String::new(),
                link_note: None,
                installing: None,
            },
            focus_search: false,
            token_visible: false,
            settings_note: None,
            folder_error: None,
            untracked_before_rescan: None,
            quiet_reload: false,
        };
        if let Err(e) = &app.bin {
            app.outcome = (e.clone(), Tone::Error);
        } else {
            app.run(Cmd::Status { rescan: false });
        }
        app
    }

    // ---- Queries the views use ----

    pub fn busy(&self) -> bool {
        self.running.is_some()
    }

    pub fn running_cmd(&self) -> Option<&Cmd> {
        self.running.as_ref().map(|r| &r.cmd)
    }

    pub fn updating(&self) -> bool {
        matches!(self.running_cmd(), Some(Cmd::Update { .. }))
    }

    pub fn cancelling(&self) -> bool {
        self.running.as_ref().is_some_and(|r| r.cancelling)
    }

    /// The status check is running for the first time or after Refresh.
    pub fn loading(&self) -> bool {
        matches!(self.running_cmd(), Some(Cmd::Status { .. }))
    }

    /// Every check failed for lack of a network: the "couldn't check" state.
    pub fn check_failed(&self) -> bool {
        self.problem == Some(Problem::Network) && !self.loading()
    }

    pub fn updates(&self) -> usize {
        if self.check_failed() {
            return 0;
        }
        self.rows.iter().filter(|r| r.status == "update").count()
    }

    pub fn item(&self, name: &str) -> Option<&Item> {
        self.items.iter().find(|i| i.name == name)
    }

    pub fn addons_dir_shown(&self) -> String {
        self.settings
            .addons_dir
            .clone()
            .or_else(|| self.status.as_ref().map(|s| s.addons_dir.clone()))
            .or_else(|| self.first_run.clone())
            .unwrap_or_default()
    }

    pub fn backups_dir(&self) -> String {
        self.status
            .as_ref()
            .map(|s| s.backups_dir.clone())
            .or_else(|| {
                let dir = self.settings.addons_dir.as_deref()?;
                let parent = Path::new(dir).parent()?;
                Some(parent.join("AddOns-backups").display().to_string())
            })
            .unwrap_or_default()
    }

    // ---- Starting commands ----

    /// Runs cmd now, or after the running command.
    pub fn run(&mut self, cmd: Cmd) {
        if self.running.is_some() {
            self.queue.push_back(cmd);
            return;
        }
        let bin = match &self.bin {
            Ok(b) => b.clone(),
            Err(e) => {
                self.outcome = (e.clone(), Tone::Error);
                return;
            }
        };
        self.before(&cmd);
        let ctx = self.ctx.clone();
        match wam::spawn(
            &bin,
            self.settings.addons_dir.as_deref(),
            &self.settings.github_token,
            &cmd.args(),
            move || ctx.request_repaint(),
        ) {
            Ok(job) => {
                self.running = Some(Running {
                    cmd,
                    job,
                    cancelling: false,
                    result: None,
                })
            }
            Err(e) => {
                self.outcome = (format!("Couldn\u{2019}t start wam: {e}"), Tone::Error);
                self.after_failed_start(&cmd);
            }
        }
    }

    pub fn cancel(&mut self) {
        if let Some(r) = &mut self.running {
            r.job.cancel();
            r.cancelling = true;
            let current = self
                .items
                .iter()
                .find(|i| i.state == "updating")
                .map(|i| i.name.clone())
                .unwrap_or_default();
            self.outcome = (
                format!("Cancel requested. {current} will finish first."),
                Tone::Warning,
            );
        }
    }

    /// Uses `picked` (or the AddOns folder inside it) as the AddOns folder and checks it.
    pub fn choose_folder(&mut self, picked: PathBuf) {
        let candidates = [
            picked.join("Interface").join("AddOns"),
            picked.join("AddOns"),
            picked.clone(),
        ];
        let dir = candidates
            .into_iter()
            .find(|p| p.is_dir())
            .unwrap_or(picked);
        if !dir.is_dir() {
            self.folder_error = Some(
                "This folder doesn\u{2019}t exist. Choose the AddOns folder inside your game\u{2019}s Interface folder."
                    .into(),
            );
            return;
        }
        self.folder_error = None;
        self.settings.addons_dir = Some(dir.display().to_string());
        self.status = None;
        self.rows.clear();
        self.items.clear();
        self.run(Cmd::Status { rescan: false });
    }

    pub fn rescan(&mut self) {
        self.untracked_before_rescan = Some(
            self.status
                .as_ref()
                .map_or(0, |s| s.untracked.iter().map(|u| u.folders.len()).sum()),
        );
        self.settings_note = None;
        self.run(Cmd::Status { rescan: true });
    }

    fn before(&mut self, cmd: &Cmd) {
        match cmd {
            Cmd::Status { .. } => {
                self.check = Some((0, self.rows.len()));
                self.problem = None;
                if self.quiet_reload {
                    return;
                }
                self.outcome = (
                    format!(
                        "Checking {}\u{2026}",
                        copy::plural(self.rows.len(), "addon")
                    ),
                    Tone::Muted,
                );
            }
            Cmd::Update { .. } => {
                self.items.clear();
                self.problem = None;
                self.open_section = None;
            }
            Cmd::Search { query } => self.find.phase = FindPhase::Searching(query.clone()),
            Cmd::Install { reference, from } => {
                self.find.installing = Some(reference.clone());
                let what = match from {
                    Origin::Search { title } => format!("Installing {title}\u{2026}"),
                    _ => "Installing\u{2026}".into(),
                };
                if *from == Origin::Link {
                    self.find.link_note = Some((what.clone(), Tone::Muted));
                }
                self.outcome = (what, Tone::Muted);
            }
            Cmd::Remove { .. } | Cmd::Track { .. } => {}
        }
    }

    fn after_failed_start(&mut self, cmd: &Cmd) {
        self.check = None;
        self.find.installing = None;
        if let Cmd::Search { .. } = cmd {
            self.find.phase = FindPhase::Down;
        }
    }

    // ---- Handling what wam sends ----

    /// Reads everything the running wam has sent.
    pub fn poll(&mut self) {
        loop {
            let Some(r) = &self.running else { return };
            let Ok(msg) = r.job.rx.try_recv() else { return };
            match msg {
                Msg::Event(ev) => self.on_event(*ev),
                Msg::Exited { stderr } => {
                    let r = self.running.take().expect("running");
                    let (ok, data, err) = r.result.unwrap_or_else(|| {
                        let last = stderr
                            .lines()
                            .rev()
                            .find(|l| !l.trim().is_empty())
                            .unwrap_or("wam stopped unexpectedly");
                        (
                            false,
                            None,
                            Some(ErrInfo {
                                message: last.trim().into(),
                                code: "other".into(),
                                ..Default::default()
                            }),
                        )
                    });
                    self.finish(r.cmd, ok, data, err);
                    if let Some(next) = self.queue.pop_front() {
                        self.run(next);
                    }
                }
            }
        }
    }

    pub fn on_event(&mut self, ev: Event) {
        match ev {
            Event::Log {
                level,
                message,
                code,
            } => {
                if code.as_deref() == Some("wow_running") {
                    self.wow_running = true;
                    return;
                }
                let tone = if level == "warning" {
                    Tone::Warning
                } else {
                    Tone::Muted
                };
                self.add_log(tone, message);
            }
            Event::Checking { addons } => {
                self.check = Some((0, addons.len()));
                if self.loading() {
                    self.rows = addons;
                }
            }
            Event::Checked { addon, done, total } => {
                self.check = Some((done, total));
                if let Some(row) = self.rows.iter_mut().find(|r| r.name == addon.name) {
                    *row = addon;
                }
            }
            Event::Item { item } => {
                match item.state.as_str() {
                    "updated" => self.add_log(
                        Tone::Muted,
                        format!(
                            "Updated {} ({} \u{2192} {}).",
                            item.name, item.from, item.to
                        ),
                    ),
                    "failed" => {
                        let why = item
                            .error
                            .as_ref()
                            .map(|e| copy::error(e, None))
                            .unwrap_or_default();
                        self.add_log(Tone::Error, format!("{} failed: {why}", item.name));
                    }
                    "updating" => self.check = None,
                    _ => {}
                }
                match self.items.iter_mut().find(|i| i.name == item.name) {
                    Some(i) => *i = item,
                    None => self.items.push(item),
                }
            }
            Event::Result { ok, data, error } => {
                if let Some(r) = &mut self.running {
                    r.result = Some((ok, data, error));
                }
            }
        }
    }

    fn finish(
        &mut self,
        cmd: Cmd,
        ok: bool,
        data: Option<serde_json::Value>,
        err: Option<ErrInfo>,
    ) {
        self.check = None;
        if let Some(e) = &err {
            match e.code.as_str() {
                "addons_dir" => {
                    self.first_run =
                        Some(e.path.clone().unwrap_or_else(|| self.addons_dir_shown()));
                    self.status = None;
                    self.rows.clear();
                    self.find.installing = None;
                    self.outcome = ("AddOns folder not found".into(), Tone::Warning);
                    return;
                }
                "locked" => {
                    self.problem = Some(Problem::Locked);
                    self.find.installing = None;
                    self.outcome = (copy::error(e, None), Tone::Error);
                    if let Cmd::Search { .. } = cmd {
                        self.find.phase = FindPhase::Down;
                    }
                    return;
                }
                _ => {}
            }
        }
        let message =
            |e: &Option<ErrInfo>| e.as_ref().map(|e| copy::error(e, None)).unwrap_or_default();
        match cmd {
            Cmd::Status { rescan } => match parse::<StatusData>(&data) {
                Some(s) if ok => self.got_status(s, rescan),
                _ => self.outcome = (message(&err), Tone::Error),
            },
            Cmd::Update { .. } => {
                let Some(d) = parse::<wam::UpdateData>(&data) else {
                    self.outcome = (message(&err), Tone::Error);
                    return;
                };
                let (text, tone) = if d.skipped > 0 {
                    (
                        format!("Stopped: {} updated, {} skipped", d.updated, d.skipped),
                        Tone::Warning,
                    )
                } else if d.failed > 0 {
                    (
                        format!(
                            "Updated {}, {} failed",
                            copy::plural(d.updated, "addon"),
                            d.failed
                        ),
                        Tone::Error,
                    )
                } else {
                    (
                        format!("Updated {}", copy::plural(d.updated, "addon")),
                        Tone::Ok,
                    )
                };
                self.add_log(
                    if tone == Tone::Ok { Tone::Muted } else { tone },
                    format!("{text}."),
                );
                self.outcome = (text, tone);
                // Updated rows keep saying so; failed and skipped ones show their update again.
                self.items = d
                    .items
                    .into_iter()
                    .filter(|i| i.state == "updated")
                    .collect();
                self.reload();
            }
            Cmd::Install { reference, from } => {
                self.find.installing = None;
                match parse::<wam::InstallData>(&data) {
                    Some(d) if ok => {
                        if let FindPhase::Results(found) = &mut self.find.phase {
                            for h in found
                                .hits
                                .iter_mut()
                                .filter(|h| h.reference.as_deref() == Some(reference.as_str()))
                            {
                                h.installed = true;
                            }
                        }
                        let text = format!("Installed {} {}", d.name, d.version);
                        self.add_log(Tone::Muted, format!("{text}."));
                        if from == Origin::Link {
                            self.find.link_note = Some((text.clone(), Tone::Ok));
                            self.find.link.clear();
                        }
                        self.outcome = (text, Tone::Ok);
                        self.reload();
                    }
                    _ => {
                        let text = message(&err);
                        self.add_log(Tone::Error, text.clone());
                        if from == Origin::Link {
                            self.find.link_note = Some((text.clone(), Tone::Error));
                        }
                        self.outcome = (text, Tone::Error);
                    }
                }
            }
            Cmd::Remove { .. } => match parse::<wam::RemoveData>(&data) {
                Some(d) if ok => {
                    let text = format!("Removed {}. Saved a backup zip.", d.name);
                    self.add_log(Tone::Muted, text.clone());
                    self.outcome = (text, Tone::Muted);
                    self.reload();
                }
                _ => self.outcome = (message(&err), Tone::Error),
            },
            Cmd::Track { folder, .. } => {
                if ok {
                    self.dialog = None;
                    let text = format!("Now tracking {folder}.");
                    self.add_log(Tone::Muted, text.clone());
                    self.outcome = (text, Tone::Muted);
                    self.reload();
                } else if let Some(Dialog::Track { error, .. }) = &mut self.dialog {
                    *error = Some(message(&err));
                } else {
                    self.outcome = (message(&err), Tone::Error);
                }
            }
            Cmd::Search { query } => {
                self.find.phase = match parse::<SearchData>(&data) {
                    Some(d) if ok && !d.hits.is_empty() => FindPhase::Results(d),
                    Some(d) if ok && !d.wago_unavailable => FindPhase::Nothing(query),
                    _ => FindPhase::Down,
                }
            }
        }
    }

    fn got_status(&mut self, s: StatusData, rescan: bool) {
        let quiet = std::mem::take(&mut self.quiet_reload);
        let now = copy::clock(Local::now());
        let checked: Vec<&ErrInfo> = s.addons.iter().filter_map(|r| r.error.as_ref()).collect();
        self.problem = if !s.addons.is_empty()
            && checked.len() == s.addons.len()
            && checked.iter().all(|e| e.code == "network")
        {
            Some(Problem::Network)
        } else {
            checked
                .iter()
                .find(|e| e.code == "rate_limit")
                .map(|e| Problem::RateLimit(copy::retry_time(e)))
        };
        self.wow_running = s.wow_running;
        self.first_run = None;
        self.rows = s.addons.clone();
        // Updates and failed checks first, so they aren't buried in a long list.
        self.rows.sort_by_key(|r| match r.status.as_str() {
            "update" => 0,
            "error" => 1,
            _ => 2,
        });
        let summary = if self.problem == Some(Problem::Network) {
            self.outcome = (
                "Couldn\u{2019}t reach CurseForge or GitHub".into(),
                Tone::Error,
            );
            self.add_log(
                Tone::Error,
                "Can\u{2019}t reach CurseForge or GitHub.".into(),
            );
            None
        } else if s.updates > 0 {
            Some(format!("{} available", copy::plural(s.updates, "update")))
        } else if s.addons.is_empty() {
            Some("no addons found".into())
        } else {
            Some(format!(
                "All {} up to date",
                copy::plural(s.addons.len(), "addon")
            ))
        };
        if let Some(summary) = summary.filter(|_| !quiet) {
            self.add_log(
                Tone::Muted,
                format!(
                    "Checked {}. {summary}.",
                    copy::plural(s.addons.len(), "addon")
                ),
            );
            for r in &s.addons {
                if let Some(e) = &r.error {
                    self.add_log(
                        Tone::Warning,
                        format!("{}: {}", r.name, copy::error(e, Some(&r.source))),
                    );
                }
            }
            self.outcome = (format!("Last check {now} \u{b7} {summary}"), Tone::Muted);
        }
        if rescan && let Some(before) = self.untracked_before_rescan.take() {
            let after: usize = s.untracked.iter().map(|u| u.folders.len()).sum();
            let found = before.saturating_sub(after);
            self.settings_note = Some((
                format!("Rescan done. {} identified.", copy::plural(found, "folder")),
                Tone::Ok,
            ));
        }
        self.status = Some(s);
    }

    /// Checks again after something changed AddOns.
    fn reload(&mut self) {
        self.quiet_reload = true;
        self.queue.push_back(Cmd::Status { rescan: false });
    }

    /// An error in the activity log, for problems that happen outside a command.
    pub fn log_error(&mut self, message: String) {
        self.add_log(Tone::Error, message);
    }

    fn add_log(&mut self, tone: Tone, message: String) {
        self.log.push(LogLine {
            time: copy::clock(Local::now()),
            tone,
            message,
        });
        if self.log.len() > 500 {
            self.log.remove(0);
        }
    }
}

#[cfg(test)]
impl App {
    /// Pretends `cmd` is running, for drawing busy states in tests.
    pub fn pretend_running(&mut self, cmd: Cmd, cancelling: bool) {
        self.running = Some(Running {
            cmd,
            job: wam::Job::idle(),
            cancelling,
            result: None,
        });
    }

    /// Applies a result as if wam had sent it.
    pub fn pretend_finished(&mut self, cmd: Cmd, data: serde_json::Value) {
        self.finish(cmd, true, Some(data), None);
    }
}

/// A result's data as the command's type.
fn parse<T: serde::de::DeserializeOwned>(data: &Option<serde_json::Value>) -> Option<T> {
    data.clone().and_then(|d| serde_json::from_value(d).ok())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn app() -> App {
        let mut a = App::new(
            egui::Context::default(),
            Settings::default(),
            Err("no wam in tests".into()),
        );
        a.outcome = (String::new(), Tone::Muted);
        a
    }

    fn row(name: &str, status: &str) -> Row {
        Row {
            name: name.into(),
            status: status.into(),
            installed: "1".into(),
            source: "curse".into(),
            ..Default::default()
        }
    }

    fn status(addons: Vec<Row>) -> serde_json::Value {
        serde_json::json!({"addons_dir":"/a","backups_dir":"/b","untracked":[],"broken":[],
            "updates": addons.iter().filter(|r| r.status == "update").count(),"wow_running":false,
            "addons": addons.iter().map(|r| serde_json::json!({"name":r.name,"installed":r.installed,"latest":"2",
                "source":"curse","ref":"curse:1","status":r.status,"adopted":false,"folders":[r.name],
                "error": r.error.as_ref().map(|e| serde_json::json!({"message":e.message,"code":e.code}))})).collect::<Vec<_>>()})
    }

    #[test]
    fn status_result_fills_the_table_and_summary() {
        let mut a = app();
        a.finish(
            Cmd::Status { rescan: false },
            true,
            Some(status(vec![row("Questie", "update"), row("Bagnon", "ok")])),
            None,
        );
        assert_eq!(a.rows.len(), 2);
        assert_eq!(a.updates(), 1);
        assert!(
            a.outcome.0.ends_with("1 update available"),
            "{}",
            a.outcome.0
        );
        assert_eq!(a.queue.len(), 0);
    }

    #[test]
    fn all_checks_failing_on_the_network_is_the_couldnt_check_state() {
        let mut a = app();
        let mut r = row("Questie", "error");
        r.error = Some(ErrInfo {
            code: "network".into(),
            message: "dial".into(),
            ..Default::default()
        });
        a.finish(
            Cmd::Status { rescan: false },
            true,
            Some(status(vec![r])),
            None,
        );
        assert!(a.check_failed());
        assert_eq!(a.updates(), 0);
        assert_eq!(a.outcome.1, Tone::Error);
    }

    #[test]
    fn missing_addons_folder_shows_first_run() {
        let mut a = app();
        let err = ErrInfo {
            code: "addons_dir".into(),
            path: Some("/nope".into()),
            ..Default::default()
        };
        a.finish(Cmd::Status { rescan: false }, false, None, Some(err));
        assert_eq!(a.first_run.as_deref(), Some("/nope"));
    }

    #[test]
    fn update_progress_and_outcome() {
        let mut a = app();
        let item = |state: &str| Event::Item {
            item: Item {
                name: "Questie".into(),
                from: "1".into(),
                to: "2".into(),
                state: state.into(),
                error: None,
            },
        };
        a.on_event(item("waiting"));
        a.on_event(item("updating"));
        assert_eq!(a.item("Questie").unwrap().state, "updating");
        a.on_event(item("updated"));
        let data = serde_json::json!({"items":[{"name":"Questie","from":"1","to":"2","state":"updated"},
            {"name":"Bagnon","from":"1","to":"2","state":"skipped"}],"dry_run":false,"updated":1,"failed":0,"skipped":1});
        a.finish(
            Cmd::Update { names: vec![] },
            false,
            Some(data),
            Some(ErrInfo {
                code: "cancelled".into(),
                ..Default::default()
            }),
        );
        assert_eq!(
            a.outcome,
            ("Stopped: 1 updated, 1 skipped".into(), Tone::Warning)
        );
        assert_eq!(a.items.len(), 1, "only updated items are kept");
        assert_eq!(
            a.queue.front(),
            Some(&Cmd::Status { rescan: false }),
            "status reloads"
        );
        let reload = a.queue.pop_front().unwrap();
        a.finish(reload, true, Some(status(vec![row("Questie", "ok")])), None);
        assert_eq!(
            a.outcome.0, "Stopped: 1 updated, 1 skipped",
            "the reload keeps it"
        );
    }

    #[test]
    fn bad_reference_keeps_the_track_dialog_open_with_an_error() {
        let mut a = app();
        a.dialog = Some(Dialog::Track {
            name: "J".into(),
            folders: vec![],
            input: "zz".into(),
            error: None,
            focus: false,
        });
        let err = ErrInfo {
            code: "bad_ref".into(),
            ..Default::default()
        };
        a.finish(
            Cmd::Track {
                folder: "J".into(),
                reference: "zz".into(),
            },
            false,
            None,
            Some(err),
        );
        assert!(
            matches!(&a.dialog, Some(Dialog::Track { error: Some(e), .. }) if e.starts_with("Couldn\u{2019}t find"))
        );
    }

    #[test]
    fn search_states() {
        let mut a = app();
        let res = |hits: serde_json::Value, down: bool| serde_json::json!({"query":"x","hits":hits,"wago_unavailable":down});
        a.finish(
            Cmd::Search { query: "x".into() },
            true,
            Some(res(serde_json::json!([]), false)),
            None,
        );
        assert!(matches!(a.find.phase, FindPhase::Nothing(_)));
        a.finish(
            Cmd::Search { query: "x".into() },
            true,
            Some(res(serde_json::json!([]), true)),
            None,
        );
        assert!(matches!(a.find.phase, FindPhase::Down));
        a.finish(
            Cmd::Search { query: "x".into() },
            true,
            Some(res(
                serde_json::json!([{"title":"X","installed":false}]),
                false,
            )),
            None,
        );
        assert!(matches!(a.find.phase, FindPhase::Results(_)));
    }

    #[test]
    fn command_arguments() {
        assert_eq!(Cmd::Status { rescan: true }.args(), ["-rescan", "status"]);
        assert_eq!(
            Cmd::Update {
                names: vec!["Questie".into()]
            }
            .args(),
            ["update", "Questie"]
        );
        assert_eq!(
            Cmd::Track {
                folder: "J".into(),
                reference: "curse:1".into()
            }
            .args(),
            ["track", "J", "curse:1"]
        );
    }
}
