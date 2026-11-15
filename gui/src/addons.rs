//! The Addons tab (docs/gui-design/mockups-src/Addons.dc.html) and its dialogs.

use eframe::egui::{
    self, Align, CornerRadius, Frame, Id, Key, Layout, Margin, Modal, Rect, ScrollArea, Sense,
    Stroke, Ui, Vec2, pos2,
};

use crate::{
    app::{App, Cmd, Dialog, Origin, Problem, Section, Tab},
    copy,
    theme::{self, Icon, Weight},
    widgets::{self, Button, Kind, in_rect, mono, text, truncated},
};

pub fn show(app: &mut App, ui: &mut Ui) {
    if let Some(path) = app.first_run.clone() {
        first_run(app, ui, &path);
        return;
    }
    let empty = app
        .status
        .as_ref()
        .is_some_and(|s| s.addons.is_empty() && s.untracked.is_empty() && s.broken.is_empty());
    if empty && !app.busy() {
        empty_state(app, ui);
        return;
    }
    egui::Panel::top("toolbar")
        .resizable(false)
        .frame(
            Frame::new()
                .fill(theme::PANEL)
                .inner_margin(Margin::symmetric(16, 10)),
        )
        .show(ui, |ui| toolbar(app, ui));
    let show_sections = !app.loading() && !app.check_failed() && section_count(app) > 0;
    // Room kept under the table: the sections and the 10 px gaps around them.
    let below = if show_sections {
        sections_height(app) + 20.0
    } else {
        10.0
    };
    egui::CentralPanel::no_frame().show(ui, |ui| {
        Frame::new()
            .inner_margin(Margin {
                left: 16,
                right: 16,
                top: 10,
                bottom: 0,
            })
            .show(ui, |ui| {
                banners(app, ui);
                table(app, ui, below);
                if show_sections {
                    ui.add_space(10.0);
                    sections(app, ui);
                }
            });
    });
}

// ---- Toolbar ----

fn toolbar(app: &mut App, ui: &mut Ui) {
    let narrow = ui.available_width() < 790.0;
    ui.horizontal(|ui| {
        let (main, color, rest, progress) = summary(app);
        let right_w = if app.updating() {
            if narrow || app.cancelling() {
                120.0
            } else {
                440.0
            }
        } else {
            260.0
        };
        let left_w = (ui.available_width() - right_w).max(200.0);
        let line = |ui: &mut Ui| {
            ui.spacing_mut().item_spacing.x = 6.0;
            ui.label(text(main, 14.0, Weight::SemiBold, color));
            truncated(ui, text(rest, 13.0, Weight::Regular, theme::MUTED));
        };
        match progress {
            // Alone, the line is centred in a row as tall as the buttons (30 px), level with
            // them. A top-down layout's main_align does not centre it: it stays at the top.
            None => {
                ui.allocate_ui_with_layout(
                    Vec2::new(left_w, 30.0),
                    Layout::left_to_right(Align::Center),
                    line,
                );
            }
            Some(p) => {
                ui.allocate_ui_with_layout(
                    Vec2::new(left_w, 34.0),
                    Layout::top_down(Align::Min).with_main_align(Align::Center),
                    |ui| {
                        ui.spacing_mut().item_spacing.y = 6.0;
                        ui.horizontal(line);
                        widgets::progress(ui, p);
                    },
                );
            }
        }
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            if app.updating() {
                let label = if app.cancelling() {
                    "Cancelling\u{2026}"
                } else {
                    "Cancel"
                };
                if ui
                    .add(
                        Button::new(Kind::Secondary, label)
                            .icon(Icon::X)
                            .enabled(!app.cancelling()),
                    )
                    .clicked()
                {
                    app.cancel();
                }
                if !narrow && !app.cancelling() {
                    ui.label(text(
                        "Cancel lets the current addon finish and skips the rest.",
                        12.0,
                        Weight::Regular,
                        theme::MUTED,
                    ));
                }
                return;
            }
            let n = app.updates();
            if n > 0
                && ui
                    .add(
                        Button::new(Kind::Primary, &format!("Update all ({n})"))
                            .icon(Icon::Update)
                            .enabled(!app.busy()),
                    )
                    .clicked()
            {
                app.run(Cmd::Update { names: vec![] });
            }
            let label = if app.check_failed() || app.problem == Some(Problem::Locked) {
                "Try again"
            } else {
                "Refresh"
            };
            if ui
                .add(
                    Button::new(Kind::Secondary, label)
                        .icon(Icon::Refresh)
                        .enabled(!app.busy()),
                )
                .clicked()
            {
                app.run(Cmd::Status { rescan: false });
            }
        });
    });
}

/// The summary line: main words, their colour, the muted rest, and progress if something runs.
fn summary(app: &App) -> (String, egui::Color32, String, Option<f32>) {
    let total = app.rows.len();
    let fraction = |(done, total): (usize, usize)| {
        if total == 0 {
            0.0
        } else {
            done as f32 / total as f32
        }
    };
    if app.updating() {
        let current = app
            .items
            .iter()
            .find(|i| i.state == "updating")
            .map(|i| i.name.clone());
        let finished = app
            .items
            .iter()
            .filter(|i| matches!(i.state.as_str(), "updated" | "failed" | "skipped"))
            .count();
        let n = app.items.len();
        if app.cancelling() {
            let waiting = app.items.iter().filter(|i| i.state == "waiting").count();
            let main = match current {
                Some(c) => format!("Stopping after {c}\u{2026}"),
                None => "Stopping\u{2026}".into(),
            };
            return (
                main,
                theme::TEXT,
                format!("\u{b7} {} will be skipped", copy::plural(waiting, "addon")),
                Some(fraction((finished, n))),
            );
        }
        if n == 0 {
            let check = app.check.unwrap_or((0, 0));
            return (
                "Checking for updates\u{2026}".into(),
                theme::TEXT,
                format!("{} of {} addons", check.0, check.1),
                Some(fraction(check)),
            );
        }
        let i = (finished + 1).min(n);
        let rest = current.map(|c| format!("\u{b7} {c}")).unwrap_or_default();
        return (
            format!("Updating {i} of {n}"),
            theme::TEXT,
            rest,
            Some(fraction((finished, n))),
        );
    }
    if app.loading() {
        let check = app.check.unwrap_or((0, total));
        return (
            "Checking for updates\u{2026}".into(),
            theme::TEXT,
            format!("{} of {} addons", check.0, check.1),
            Some(fraction(check)),
        );
    }
    if app.check_failed() {
        return (
            "Couldn\u{2019}t check for updates".into(),
            theme::ERROR,
            format!("\u{b7} {} installed", copy::plural(total, "addon")),
            None,
        );
    }
    let n = app.updates();
    let failed = app.rows.iter().filter(|r| r.status == "error").count();
    let mut rest = format!("\u{b7} {}", copy::plural(total, "addon"));
    if failed > 0 {
        rest += &format!(" \u{b7} {failed} couldn\u{2019}t be checked");
    }
    if n > 0 {
        (
            format!("{} available", copy::plural(n, "update")),
            theme::ACCENT_TEXT,
            rest,
            None,
        )
    } else {
        (
            format!("All {} up to date", copy::plural(total, "addon")),
            theme::OK,
            rest,
            None,
        )
    }
}

fn banners(app: &App, ui: &mut Ui) {
    let mut any = false;
    let mut show = |ui: &mut Ui, tint, icon, msg: &str| {
        widgets::banner(ui, tint, icon, msg);
        any = true;
    };
    if app.wow_running && (app.updating() || app.updates() > 0) {
        show(
            ui,
            &theme::BANNER_WARNING,
            Icon::Alert,
            "World of Warcraft is running. Updated addons load after you type /reload in the game, or restart it.",
        );
    }
    match &app.problem {
        Some(Problem::Network) => show(
            ui,
            &theme::BANNER_ERROR,
            Icon::Alert,
            "Can\u{2019}t reach CurseForge or GitHub. Check your internet connection and try again. Your installed addons are fine.",
        ),
        Some(Problem::Locked) => show(
            ui,
            &theme::BANNER_ERROR,
            Icon::Alert,
            "wam is already running in another window. Close it, then try again.",
        ),
        Some(Problem::RateLimit(t)) => show(
            ui,
            &theme::BANNER_WARNING,
            Icon::Alert,
            &format!(
                "GitHub\u{2019}s limit was reached. GitHub addons are checked again {t}, or add a token in Settings."
            ),
        ),
        None => {}
    }
    if any {
        ui.add_space(2.0);
    }
}

// ---- Table ----

struct Columns {
    widths: Vec<f32>,
    source: bool,
}

fn columns(width: f32) -> Columns {
    let (fixed, source): (&[f32], bool) =
        // Actions are 18 px wider than the design's 92/72, taken from Status: Update and Remove
        // side by side need 112.
        if width < 788.0 { (&[96.0, 120.0, 160.0, 112.0], false) } else { (&[128.0, 152.0, 112.0, 200.0, 112.0], true) };
    let name = (width - fixed.iter().sum::<f32>()).max(120.0);
    Columns {
        widths: std::iter::once(name).chain(fixed.iter().copied()).collect(),
        source,
    }
}

/// Splits a row's rect into its cells' rects.
fn cells(rect: Rect, widths: &[f32]) -> Vec<Rect> {
    let mut x = rect.left();
    widths
        .iter()
        .map(|w| {
            let r = Rect::from_min_max(pos2(x, rect.top()), pos2(x + w, rect.bottom()));
            x += w;
            r
        })
        .collect()
}

fn pad(r: Rect, px: f32) -> Rect {
    r.shrink2(Vec2::new(px, 0.0))
}

/// The table, leaving `below` points free under it.
fn table(app: &mut App, ui: &mut Ui, below: f32) {
    let guessed = !app.updating()
        && !app.check_failed()
        && app.rows.iter().any(|r| r.adopted && r.error.is_none());
    Frame::new().stroke(Stroke::new(1.0, theme::BORDER)).corner_radius(3).show(ui, |ui| {
        let width = ui.available_width();
        let cols = columns(width);
        ui.spacing_mut().item_spacing.y = 0.0;

        let (header, _) = ui.allocate_exact_size(Vec2::new(width, 30.0), Sense::hover());
        let p = ui.painter();
        p.rect_filled(header, CornerRadius { nw: 6, ne: 6, sw: 0, se: 0 }, theme::TABLE_HEADER);
        p.hline(header.x_range(), header.bottom(), Stroke::new(1.0, theme::BORDER));
        let titles: &[&str] =
            if cols.source { &["Addon", "Installed", "Latest", "Source", "Status", ""] } else { &["Addon", "Installed", "Latest", "Status", ""] };
        for (r, t) in cells(header, &cols.widths).into_iter().zip(titles) {
            in_rect(ui, pad(r, 12.0), Align::Min, |ui| ui.label(text(*t, 12.0, Weight::Medium, theme::MUTED)));
        }

        let footer = if guessed { 26.0 } else { 0.0 };
        let rows_h = (ui.available_height() - footer - below).max(64.0);
        let mut action = None;
        ScrollArea::vertical().id_salt("addons").auto_shrink([false, false]).max_height(rows_h).min_scrolled_height(rows_h).show_rows(
            ui,
            32.0,
            app.rows.len(),
            |ui, range| {
                for i in range {
                    if let Some(a) = row(app, ui, i, &cols, width) {
                        action = Some(a);
                    }
                }
            },
        );
        if guessed {
            let (r, _) = ui.allocate_exact_size(Vec2::new(width, footer), Sense::hover());
            ui.painter().rect_filled(r, CornerRadius { nw: 0, ne: 0, sw: 6, se: 6 }, theme::PANEL);
            ui.painter().hline(r.x_range(), r.top(), Stroke::new(1.0, theme::BORDER));
            in_rect(ui, pad(r, 12.0), Align::Min, |ui| {
                ui.label(mono("~", theme::TEXT_SECONDARY));
                truncated(ui, text(
                    "Estimated version: this addon was installed without wam. It becomes exact after its first update.",
                    12.0, Weight::Regular, theme::MUTED));
            });
        }
        match action {
            Some(RowAction::Update(name)) => app.run(Cmd::Update { names: vec![name] }),
            Some(RowAction::Remove(name, folders)) => app.dialog = Some(Dialog::Remove { name, folders }),
            None => {}
        }
    });
}

enum RowAction {
    Update(String),
    Remove(String, Vec<String>),
}

fn row(app: &App, ui: &mut Ui, i: usize, cols: &Columns, width: f32) -> Option<RowAction> {
    let r = &app.rows[i];
    let (rect, _) = ui.allocate_exact_size(Vec2::new(width, 32.0), Sense::hover());
    let base = if i % 2 == 1 {
        theme::ROW_STRIPE
    } else {
        theme::BACKGROUND
    };
    ui.painter()
        .rect_filled(rect, 0, widgets::row_fill(ui, rect, base));
    let hovered = ui.rect_contains_pointer(rect);
    let cell = cells(rect, &cols.widths);
    let mut c = cell.iter();

    in_rect(ui, pad(*c.next()?, 12.0), Align::Min, |ui| {
        truncated(ui, text(&r.name, 13.0, Weight::Medium, theme::TEXT))
    });
    in_rect(ui, pad(*c.next()?, 12.0), Align::Min, |ui| {
        ui.spacing_mut().item_spacing.x = 0.0;
        truncated(ui, mono(&r.installed, theme::TEXT));
        if r.adopted && r.error.is_none() {
            ui.label(mono(" ~", theme::MUTED))
                .on_hover_text("Estimated: installed without wam");
        }
    });
    let (latest, color) = match r.status.as_str() {
        _ if app.check_failed() => ("\u{2014}".to_string(), theme::MUTED),
        "checking" => ("\u{2026}".to_string(), theme::MUTED),
        "error" => ("\u{2014}".to_string(), theme::MUTED),
        "update" => (r.latest.clone(), theme::ACCENT_TEXT),
        _ => (r.latest.clone(), theme::TEXT),
    };
    in_rect(ui, pad(*c.next()?, 12.0), Align::Min, |ui| {
        truncated(ui, mono(latest, color))
    });
    if cols.source {
        in_rect(ui, pad(*c.next()?, 12.0), Align::Min, |ui| {
            ui.label(text(
                copy::site(&r.source),
                13.0,
                Weight::Regular,
                theme::MUTED,
            ))
            .on_hover_text(&r.reference)
        });
    }

    let item = app.item(&r.name);
    let (state, detail) = match item {
        _ if app.check_failed() => ("notchecked".to_string(), String::new()),
        Some(it) => {
            let detail = match it.state.as_str() {
                "updated" => format!("{} \u{2192} {}", it.from, it.to),
                "failed" => it
                    .error
                    .as_ref()
                    .map(|e| copy::error(e, Some(&r.source)))
                    .unwrap_or_default(),
                _ => String::new(),
            };
            (it.state.clone(), detail)
        }
        None => (
            r.status.clone(),
            r.error
                .as_ref()
                .map(|e| copy::error(e, Some(&r.source)))
                .unwrap_or_default(),
        ),
    };
    in_rect(ui, pad(*c.next()?, 12.0), Align::Min, |ui| {
        widgets::status(ui, &widgets::status_look(&state), &detail)
    });

    let mut action = None;
    in_rect(ui, pad(*c.next()?, 8.0), Align::Max, |ui| {
        ui.spacing_mut().item_spacing.x = 4.0;
        // Remove shows only under the pointer; the 26 px stays reserved so columns don't shift.
        if hovered {
            let remove = Button::new(Kind::IconOnly, "")
                .icon(Icon::Trash)
                .enabled(!app.busy());
            let tip = format!("Remove {}", r.name);
            if ui.add(remove.tooltip(&tip)).clicked() {
                action = Some(RowAction::Remove(r.name.clone(), r.folders.clone()));
            }
        } else {
            ui.add_space(26.0);
        }
        let updated = item.is_some_and(|i| i.state == "updated");
        if r.status == "update"
            && !app.busy()
            && !app.check_failed()
            && !updated
            && ui.add(Button::new(Kind::Small, "Update")).clicked()
        {
            action = Some(RowAction::Update(r.name.clone()));
        }
    });
    action
}

// ---- Not tracked, Broken, Errors ----

struct SectionItem {
    name: String,
    extra: String,
    note: String,
    action: Option<(&'static str, ItemAction)>,
}

#[derive(Clone)]
enum ItemAction {
    Track(String, Vec<String>),
    Fix(String),
}

fn section_items(app: &App, s: Section) -> Vec<SectionItem> {
    let Some(status) = &app.status else {
        return vec![];
    };
    match s {
        Section::Untracked => status
            .untracked
            .iter()
            .map(|u| {
                let (name, extra) = match u.folders.len() {
                    1 => (u.folders[0].clone(), String::new()),
                    n => (u.root.clone(), format!("(+{} folders)", n - 1)),
                };
                SectionItem {
                    action: Some((
                        "Track\u{2026}",
                        ItemAction::Track(name.clone(), u.folders.clone()),
                    )),
                    name,
                    extra,
                    note: String::new(),
                }
            })
            .collect(),
        Section::Broken => status
            .broken
            .iter()
            .map(|b| match &b.reference {
                Some(r) => SectionItem {
                    name: b.folder.clone(),
                    extra: String::new(),
                    note:
                        "Unpacked wrongly, so the game can\u{2019}t load it. Fix installs it again."
                            .into(),
                    action: Some(("Fix", ItemAction::Fix(r.clone()))),
                },
                None => SectionItem {
                    name: b.folder.clone(),
                    extra: String::new(),
                    note: "Source unknown. Tell wam where it comes from, then fix it.".into(),
                    action: Some((
                        "Track\u{2026}",
                        ItemAction::Track(b.folder.clone(), vec![b.folder.clone()]),
                    )),
                },
            })
            .collect(),
        Section::Errors => app
            .rows
            .iter()
            .filter_map(|r| {
                let e = r.error.as_ref()?;
                Some(SectionItem {
                    name: r.name.clone(),
                    extra: String::new(),
                    note: copy::error(e, Some(&r.source)),
                    action: None,
                })
            })
            .collect(),
    }
}

const SECTIONS: [(Section, &str, &str, theme::Tint); 3] = [
    (
        Section::Untracked,
        "Not tracked",
        "Folders wam can\u{2019}t identify, so it doesn\u{2019}t check them for updates.",
        theme::PILL_NEUTRAL,
    ),
    (
        Section::Broken,
        "Broken",
        "Folders the game can\u{2019}t load.",
        theme::PILL_WARNING,
    ),
    (
        Section::Errors,
        "Errors",
        "Update checks that failed.",
        theme::PILL_ERROR,
    ),
];

fn section_count(app: &App) -> usize {
    SECTIONS
        .iter()
        .filter(|(s, ..)| !section_items(app, *s).is_empty())
        .count()
}

/// The sections' height: a 32 px header each, up to 3 rows of the open one, the border.
fn sections_height(app: &App) -> f32 {
    SECTIONS
        .iter()
        .map(|(s, ..)| match section_items(app, *s).len() {
            0 => 0.0,
            n if app.open_section == Some(*s) => 32.0 + 36.0 * n.min(3) as f32,
            _ => 32.0,
        })
        .sum::<f32>()
        + 2.0
}

fn sections(app: &mut App, ui: &mut Ui) {
    let mut action = None;
    Frame::new()
        .stroke(Stroke::new(1.0, theme::BORDER))
        .corner_radius(3)
        .show(ui, |ui| {
            ui.spacing_mut().item_spacing.y = 0.0;
            let width = ui.available_width();
            let mut first = true;
            for (section, title, hint, tint) in &SECTIONS {
                let items = section_items(app, *section);
                if items.is_empty() {
                    continue;
                }
                let open = app.open_section == Some(*section);
                let (rect, resp) = ui.allocate_exact_size(Vec2::new(width, 32.0), Sense::click());
                resp.widget_info(|| {
                    egui::WidgetInfo::selected(egui::accesskit::Role::Button, true, open, *title)
                });
                ui.painter()
                    .rect_filled(rect, 0, widgets::row_fill(ui, rect, theme::PANEL));
                if !first {
                    ui.painter()
                        .hline(rect.x_range(), rect.top(), Stroke::new(1.0, theme::BORDER));
                }
                first = false;
                in_rect(ui, pad(rect, 12.0), Align::Min, |ui| {
                    ui.spacing_mut().item_spacing.x = 8.0;
                    widgets::icon(
                        ui,
                        if open {
                            Icon::ChevronDown
                        } else {
                            Icon::ChevronRight
                        },
                        theme::MUTED,
                    );
                    ui.label(text(*title, 13.0, Weight::Display, theme::ACCENT_TEXT));
                    widgets::pill(ui, items.len(), tint);
                    truncated(ui, text(*hint, 13.0, Weight::Regular, theme::MUTED));
                });
                if resp.has_focus() {
                    ui.painter().rect_stroke(
                        rect.shrink(2.0),
                        4,
                        Stroke::new(2.0, theme::FOCUS),
                        egui::StrokeKind::Inside,
                    );
                }
                if resp.clicked() {
                    app.open_section = if open { None } else { Some(*section) };
                }
                if !open {
                    continue;
                }
                ScrollArea::vertical()
                    .id_salt(*title)
                    .max_height(108.0)
                    .auto_shrink([false, true])
                    .show(ui, |ui| {
                        for it in &items {
                            let (r, _) =
                                ui.allocate_exact_size(Vec2::new(width, 36.0), Sense::hover());
                            ui.painter().rect_filled(r, 0, theme::BACKGROUND);
                            ui.painter().hline(
                                r.x_range(),
                                r.top(),
                                Stroke::new(1.0, theme::LIST_DIVIDER),
                            );
                            let inner = Rect::from_min_max(
                                pos2(r.left() + 36.0, r.top()),
                                pos2(r.right() - 12.0, r.bottom()),
                            );
                            let action_w = if it.action.is_some() { 90.0 } else { 0.0 };
                            let (text_r, btn_r) =
                                inner.split_left_right_at_x(inner.right() - action_w);
                            in_rect(ui, text_r, Align::Min, |ui| {
                                ui.spacing_mut().item_spacing.x = 10.0;
                                ui.label(text(&it.name, 13.0, Weight::Medium, theme::TEXT));
                                if !it.extra.is_empty() {
                                    ui.label(text(&it.extra, 13.0, Weight::Regular, theme::MUTED));
                                }
                                truncated(ui, text(&it.note, 13.0, Weight::Regular, theme::MUTED));
                            });
                            if let Some((label, act)) = &it.action {
                                in_rect(ui, btn_r, Align::Max, |ui| {
                                    if ui
                                        .add(Button::new(Kind::Small, label).enabled(!app.busy()))
                                        .clicked()
                                    {
                                        action = Some(act.clone());
                                    }
                                });
                            }
                        }
                    });
            }
        });
    match action {
        Some(ItemAction::Track(name, folders)) => {
            app.dialog = Some(Dialog::Track {
                name,
                folders,
                input: String::new(),
                error: None,
                focus: true,
            })
        }
        Some(ItemAction::Fix(reference)) => app.run(Cmd::Install {
            reference,
            from: Origin::Fix,
        }),
        None => {}
    }
}

// ---- Dialogs ----

enum DialogAction {
    Close,
    Remove(String),
    Track(String, String),
}

pub fn dialogs(app: &mut App, ctx: &egui::Context) {
    let tracking = matches!(app.running_cmd(), Some(Cmd::Track { .. }));
    let backups = app.backups_dir();
    let busy = app.busy();
    let Some(dialog) = &mut app.dialog else {
        return;
    };
    let width = if matches!(dialog, Dialog::Remove { .. }) {
        440.0
    } else {
        480.0
    };
    let frame = Frame::new()
        .fill(theme::RAISED)
        .stroke(Stroke::new(1.0, theme::DIALOG_BORDER))
        .corner_radius(4)
        .inner_margin(20)
        .shadow(egui::Shadow {
            offset: [0, 12],
            blur: 32,
            spread: 0,
            color: egui::Color32::from_black_alpha(128),
        });
    let mut action = None;
    let resp = Modal::new(Id::unique("wam-dialog")).backdrop_color(theme::SCRIM).frame(frame).show(ctx, |ui| {
        ui.set_width(width);
        ui.spacing_mut().item_spacing.y = 14.0;
        match dialog {
            Dialog::Remove { name, folders } => {
                ui.label(text(format!("Remove {name}?"), 15.0, Weight::Display, theme::TEXT));
                ui.add(egui::Label::new(text(
                    "These folders are saved as a zip in the backups folder first, so you can put them back later.",
                    13.0, Weight::Regular, theme::TEXT_SECONDARY)).wrap());
                folder_list(ui, folders);
                ui.vertical(|ui| {
                    ui.spacing_mut().item_spacing.y = 2.0;
                    ui.label(text("Backups folder", 12.0, Weight::Regular, theme::MUTED));
                    ui.add(egui::Label::new(mono(&backups, theme::TEXT_SECONDARY)).wrap());
                });
                ui.label(text("Your in-game settings for this addon are never touched.", 12.0, Weight::Regular, theme::MUTED));
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    if ui.add(Button::new(Kind::Danger, "Remove").icon(Icon::Trash).enabled(!busy)).clicked() {
                        action = Some(DialogAction::Remove(name.clone()));
                    }
                    let cancel = ui.add(Button::new(Kind::Secondary, "Cancel"));
                    if ui.memory(|m| m.focused().is_none()) {
                        cancel.request_focus();
                    }
                    if cancel.clicked() {
                        action = Some(DialogAction::Close);
                    }
                });
            }
            Dialog::Track { name, folders, input, error, focus } => {
                ui.label(text(format!("Where does {name} come from?"), 15.0, Weight::Display, theme::TEXT));
                ui.add(egui::Label::new(text(
                    "wam found these folders but doesn\u{2019}t know which addon they belong to, so it can\u{2019}t check them for updates.",
                    13.0, Weight::Regular, theme::TEXT_SECONDARY)).wrap());
                folder_list(ui, folders);
                let mut submit = false;
                ui.vertical(|ui| {
                    ui.spacing_mut().item_spacing.y = 6.0;
                    ui.label(text("Addon page or name", 12.0, Weight::Medium, theme::TEXT_SECONDARY));
                    let hint = format!("https://www.curseforge.com/wow/addons/{}", name.to_lowercase());
                    let r = widgets::field(ui, input, &hint, ui.available_width(), error.is_some());
                    if std::mem::take(focus) {
                        r.request_focus();
                    }
                    if r.changed() {
                        *error = None;
                    }
                    submit = r.lost_focus() && ui.input(|i| i.key_pressed(Key::Enter));
                    match error {
                        Some(e) => widgets::field_error(ui, e),
                        None => {
                            ui.add(egui::Label::new(text(
                                "Paste a CurseForge link, type a CurseForge name such as details, or a GitHub project such as github:Questie/Questie.",
                                12.0, Weight::Regular, theme::MUTED)).wrap());
                        }
                    }
                });
                ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
                    let ready = !input.trim().is_empty() && !busy;
                    if ui.add(Button::new(Kind::Primary, "Track").enabled(ready || tracking).busy(tracking)).clicked() || (submit && ready) {
                        action = Some(DialogAction::Track(folders[0].clone(), input.trim().to_string()));
                    }
                    if ui.add(Button::new(Kind::Secondary, "Cancel").enabled(!tracking)).clicked() {
                        action = Some(DialogAction::Close);
                    }
                });
            }
        }
    });
    // A thin inner line inside the gold border makes the dialog a double frame.
    ctx.layer_painter(resp.response.layer_id).rect_stroke(
        resp.response.rect.shrink(4.0),
        egui::CornerRadius::same(2),
        Stroke::new(1.0, theme::BORDER),
        egui::StrokeKind::Inside,
    );
    if resp.should_close() && !tracking && action.is_none() {
        action = Some(DialogAction::Close);
    }
    match action {
        Some(DialogAction::Close) => app.dialog = None,
        Some(DialogAction::Remove(name)) => {
            app.dialog = None;
            app.run(Cmd::Remove { name });
        }
        Some(DialogAction::Track(folder, reference)) => {
            if let Some(Dialog::Track { error, .. }) = &mut app.dialog {
                *error = None;
            }
            app.run(Cmd::Track { folder, reference });
        }
        None => {}
    }
}

fn folder_list(ui: &mut Ui, folders: &[String]) {
    Frame::new()
        .fill(theme::BACKGROUND)
        .stroke(Stroke::new(1.0, theme::BORDER))
        .corner_radius(2)
        .inner_margin(Margin::symmetric(12, 8))
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            ui.spacing_mut().item_spacing.y = 2.0;
            ScrollArea::vertical().max_height(120.0).show(ui, |ui| {
                for f in folders {
                    ui.label(mono(f, theme::TEXT_SECONDARY));
                }
            });
        });
}

// ---- First run and empty ----

/// Opens the folder picker and uses what the player picks.
pub fn pick_folder(app: &mut App) {
    if let Some(dir) = rfd::FileDialog::new()
        .set_title("Choose your AddOns folder")
        .pick_folder()
    {
        app.choose_folder(dir);
    }
}

fn centered(ui: &mut Ui, height: f32, add: impl FnOnce(&mut Ui)) {
    let space = ((ui.available_height() - height) / 2.0).max(24.0);
    ui.add_space(space);
    ui.vertical_centered(add);
}

fn first_run(app: &mut App, ui: &mut Ui, path: &str) {
    centered(ui, 300.0, |ui| {
        let width = 500.0_f32.min(ui.available_width() - 48.0);
        Frame::new()
            .fill(theme::PANEL)
            .stroke(Stroke::new(1.0, theme::BORDER))
            .corner_radius(4)
            .inner_margin(28)
            .show(ui, |ui| {
                ui.set_width(width - 56.0);
                ui.spacing_mut().item_spacing.y = 14.0;
                ui.with_layout(Layout::top_down(Align::Min), |ui| {
                    ui.horizontal(|ui| {
                        ui.spacing_mut().item_spacing.x = 10.0;
                        ui.add(Icon::Folder.image(20.0, theme::WARNING));
                        ui.label(text("Where is your AddOns folder?", 18.0, Weight::Display, theme::TEXT));
                    });
                    ui.add(egui::Label::new(text(
                        "wam looks after the addons in your game\u{2019}s AddOns folder, but it couldn\u{2019}t find that folder here:",
                        13.0, Weight::Regular, theme::TEXT_SECONDARY)).wrap());
                    Frame::new()
                        .fill(theme::LOG_BG)
                        .stroke(Stroke::new(1.0, theme::BORDER))
                        .corner_radius(2)
                        .inner_margin(Margin::symmetric(10, 8))
                        .show(ui, |ui| {
                            ui.set_width(ui.available_width());
                            ui.add(egui::Label::new(mono(path, theme::MUTED)).wrap());
                        });
                    if let Some(e) = &app.folder_error {
                        widgets::field_error(ui, e);
                    }
                    ui.add(egui::Label::new(text(
                        "Choose the AddOns folder inside your game\u{2019}s Interface folder. If there is no AddOns folder yet, start the game once, then come back.",
                        13.0, Weight::Regular, theme::MUTED)).wrap());
                    let row = Vec2::new(ui.available_width(), 30.0);
                    ui.allocate_ui_with_layout(row, Layout::right_to_left(Align::Center), |ui| {
                        if ui.add(Button::new(Kind::Primary, "Choose folder\u{2026}").icon(Icon::Folder).enabled(!app.busy())).clicked() {
                            pick_folder(app);
                        }
                    });
                });
            });
    });
}

fn empty_state(app: &mut App, ui: &mut Ui) {
    centered(ui, 180.0, |ui| {
        ui.set_max_width(380.0);
        ui.spacing_mut().item_spacing.y = 12.0;
        let (tile, _) = ui.allocate_exact_size(Vec2::splat(44.0), Sense::hover());
        ui.painter().rect_filled(tile, 8, theme::CONTROL);
        Icon::Download.paint(
            ui,
            Rect::from_center_size(tile.center(), Vec2::splat(20.0)),
            theme::MUTED,
        );
        ui.label(text("No addons yet", 16.0, Weight::Display, theme::TEXT));
        ui.add(egui::Label::new(text(
            "Your AddOns folder is empty. Find an addon to install, or check that wam is looking in the right folder.",
            13.0, Weight::Regular, theme::MUTED)).wrap());
        ui.horizontal(|ui| {
            let find = Button::new(Kind::Primary, "Find addons").icon(Icon::Search);
            let check = Button::new(Kind::Secondary, "Check folder");
            let w = find.width(ui) + ui.spacing().item_spacing.x + check.width(ui);
            ui.add_space((ui.available_width() - w).max(0.0) / 2.0);
            if ui.add(find).clicked() {
                app.tab = Tab::Find;
            }
            if ui.add(check).clicked() {
                app.tab = Tab::Settings;
            }
        });
    });
}
