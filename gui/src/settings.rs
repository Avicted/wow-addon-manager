//! The Settings tab (docs/gui-design/mockups-src/Settings.dc.html).

use std::path::Path;

use eframe::egui::{self, Frame, Margin, ScrollArea, Stroke, Ui};

use crate::{
    addons::pick_folder,
    app::{App, Cmd, Problem, Tone},
    copy,
    theme::{self, Icon, Weight},
    widgets::{self, Button, Kind, mono, text, truncated},
};

pub fn show(app: &mut App, ui: &mut Ui) {
    ScrollArea::vertical()
        .auto_shrink([false, false])
        .show(ui, |ui| {
            Frame::new()
                .inner_margin(Margin {
                    left: 24,
                    right: 24,
                    top: 8,
                    bottom: 16,
                })
                .show(ui, |ui| {
                    ui.set_max_width(680.0);
                    ui.spacing_mut().item_spacing.y = 0.0;
                    section(ui, true, |ui| addons_folder(app, ui));
                    section(ui, false, |ui| rescan(app, ui));
                    section(ui, false, |ui| token(app, ui));
                    section(ui, false, |ui| backups(app, ui));
                });
        });
}

/// A settings group: 16 px above and below, a line above all but the first.
fn section(ui: &mut Ui, first: bool, add: impl FnOnce(&mut Ui)) {
    if !first {
        let w = ui.available_width();
        let (r, _) = ui.allocate_exact_size(egui::vec2(w, 1.0), egui::Sense::hover());
        ui.painter()
            .hline(r.x_range(), r.center().y, Stroke::new(1.0, theme::BORDER));
    }
    Frame::new()
        .inner_margin(Margin::symmetric(0, 16))
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            ui.spacing_mut().item_spacing.y = 8.0;
            add(ui);
        });
}

fn paragraph(ui: &mut Ui, s: &str) {
    ui.scope(|ui| {
        ui.set_max_width(560.0);
        ui.add(egui::Label::new(text(s, 13.0, Weight::Regular, theme::MUTED)).wrap());
    });
}

fn addons_folder(app: &mut App, ui: &mut Ui) {
    ui.label(text("AddOns folder", 14.0, Weight::SemiBold, theme::TEXT));
    let mut dir = app.addons_dir_shown();
    let exists = !dir.is_empty() && Path::new(&dir).is_dir();
    let bad = app.first_run.is_some() || !exists || app.folder_error.is_some();
    ui.horizontal(|ui| {
        let w = ui.available_width() - 100.0;
        let edit = egui::TextEdit::singleline(&mut dir)
            .font(theme::mono())
            .interactive(false);
        widgets::field_with(ui, edit, "", w, bad, 10);
        if ui
            .add(
                Button::new(Kind::Secondary, "Browse\u{2026}")
                    .icon(Icon::Folder)
                    .enabled(!app.busy()),
            )
            .clicked()
        {
            pick_folder(app);
        }
    });
    if bad {
        let msg = app.folder_error.clone().unwrap_or_else(|| {
            "This folder doesn\u{2019}t exist. Choose the AddOns folder inside your game\u{2019}s Interface folder.".into()
        });
        widgets::field_error(ui, &msg);
    } else if let Some(s) = &app.status {
        let untracked: usize = s.untracked.iter().map(|u| u.folders.len()).sum();
        ui.horizontal(|ui| {
            widgets::icon(ui, Icon::Check, theme::OK);
            ui.label(text(
                format!(
                    "Folder found. {}, {untracked} not tracked.",
                    copy::plural(s.addons.len(), "addon")
                ),
                12.0,
                Weight::Regular,
                theme::OK,
            ));
        });
    }
}

fn rescan(app: &mut App, ui: &mut Ui) {
    ui.label(text(
        "Unidentified folders",
        14.0,
        Weight::SemiBold,
        theme::TEXT,
    ));
    paragraph(
        ui,
        "If you added addons by hand, wam can look at those folders again to work out where they came from.",
    );
    let running = matches!(app.running_cmd(), Some(Cmd::Status { rescan: true }));
    let b = Button::new(Kind::Secondary, "Rescan unidentified folders")
        .icon(Icon::Refresh)
        .busy(running)
        .enabled(!app.busy() && app.first_run.is_none());
    if ui.add(b).clicked() {
        app.rescan();
    }
    if let Some((msg, tone)) = &app.settings_note {
        let color = if *tone == Tone::Ok {
            theme::OK
        } else {
            theme::MUTED
        };
        ui.label(text(msg, 12.0, Weight::Regular, color));
    }
}

fn token(app: &mut App, ui: &mut Ui) {
    ui.horizontal(|ui| {
        ui.spacing_mut().item_spacing.x = 4.0;
        ui.label(text("GitHub token", 14.0, Weight::SemiBold, theme::TEXT));
        ui.label(text("(optional)", 14.0, Weight::Regular, theme::MUTED));
    });
    paragraph(
        ui,
        "GitHub allows 60 update checks an hour without a token. If many of your addons come from GitHub, a token raises that limit. It needs no permissions.",
    );
    if let Some(Problem::RateLimit(t)) = &app.problem {
        ui.scope(|ui| {
            ui.set_max_width(560.0);
            widgets::banner(ui, &theme::BANNER_WARNING, Icon::Alert,
                &format!("GitHub\u{2019}s limit was reached. GitHub addons are checked again {t}, or add a token now."));
        });
    }
    ui.horizontal(|ui| {
        let w = ui.available_width().min(560.0) - 40.0;
        let edit = egui::TextEdit::singleline(&mut app.settings.github_token)
            .password(!app.token_visible)
            .font(theme::mono());
        widgets::field_with(ui, edit, "Paste a token", w, false, 10);
        let tip = if app.token_visible {
            "Hide token"
        } else {
            "Show token"
        };
        if ui
            .add(Button::new(Kind::IconOnly, "").icon(Icon::Eye).tooltip(tip))
            .clicked()
        {
            app.token_visible = !app.token_visible;
        }
    });
    widgets::link(
        ui,
        "Create a token on github.com",
        "https://github.com/settings/tokens",
        None,
    );
}

fn backups(app: &mut App, ui: &mut Ui) {
    ui.label(text("Backups", 14.0, Weight::SemiBold, theme::TEXT));
    paragraph(
        ui,
        "Before an addon is removed or replaced, wam saves it here as a zip, so you can always put it back.",
    );
    let dir = app.backups_dir();
    ui.horizontal(|ui| {
        let w = ui.available_width() - 130.0;
        Frame::new()
            .fill(theme::PANEL)
            .stroke(Stroke::new(1.0, theme::BORDER))
            .corner_radius(4)
            .inner_margin(Margin::symmetric(10, 7))
            .show(ui, |ui| {
                ui.set_width(w - 20.0);
                truncated(ui, mono(&dir, theme::TEXT_SECONDARY));
            });
        if ui
            .add(Button::new(Kind::Secondary, "Open folder").icon(Icon::External))
            .clicked()
        {
            let _ = std::fs::create_dir_all(&dir);
            if let Err(e) = open::that_detached(&dir) {
                app.outcome = (format!("Couldn\u{2019}t open {dir}: {e}"), Tone::Error);
            }
        }
    });
}
