//! The Find addons tab (docs/gui-design/mockups-src/Find.dc.html).

use eframe::egui::{
    self, Align, CornerRadius, Frame, Key, Margin, Rect, ScrollArea, Sense, Stroke, Ui, Vec2, pos2,
};

use crate::{
    app::{App, Cmd, FindPhase, Origin, Tone},
    copy,
    theme::{self, Icon, Weight},
    widgets::{self, Button, Kind, in_rect, mono, text, truncated},
};

pub fn show(app: &mut App, ui: &mut Ui) {
    Frame::new().inner_margin(16).show(ui, |ui| {
        ui.spacing_mut().item_spacing.y = 12.0;
        search_form(app, ui);
        let link_h = 118.0
            + if app.find.link_note.is_some() {
                22.0
            } else {
                0.0
            };
        let results_h = (ui.available_height() - link_h - 12.0).max(120.0);
        ui.allocate_ui(Vec2::new(ui.available_width(), results_h), |ui| {
            results(app, ui, results_h)
        });
        link_form(app, ui);
    });
}

fn search_form(app: &mut App, ui: &mut Ui) {
    let searching = matches!(app.find.phase, FindPhase::Searching(_));
    ui.horizontal(|ui| {
        let width = (ui.available_width().min(640.0) - 90.0).max(200.0);
        let edit = egui::TextEdit::singleline(&mut app.find.query)
            .id_salt("search")
            .font(theme::font(13.0, Weight::Regular));
        let r = widgets::field_with(
            ui,
            edit,
            "Search by addon name, for example Questie or bags",
            width,
            false,
            32,
        );
        Icon::Search.paint(
            ui,
            Rect::from_min_size(
                pos2(r.rect.left() + 10.0, r.rect.center().y - 8.0),
                Vec2::splat(16.0),
            ),
            theme::MUTED,
        );
        if std::mem::take(&mut app.focus_search) {
            r.request_focus();
        }
        let enter = r.lost_focus() && ui.input(|i| i.key_pressed(Key::Enter));
        let go = ui
            .add(Button::new(Kind::Primary, "Search").enabled(!app.busy()))
            .clicked();
        let query = app.find.query.trim().to_string();
        if (go || enter) && !query.is_empty() && !app.busy() && !searching {
            app.run(Cmd::Search { query });
        }
    });
}

fn results(app: &mut App, ui: &mut Ui, height: f32) {
    Frame::new().stroke(Stroke::new(1.0, theme::BORDER)).corner_radius(6).show(ui, |ui| {
        ui.set_min_size(Vec2::new(ui.available_width(), height - 2.0));
        let width = ui.available_width();
        match &app.find.phase {
            FindPhase::Results(_) => hits(app, ui, width),
            FindPhase::Idle => message(ui, Some((Icon::Search, theme::MUTED)), "Find an addon",
                "Search CurseForge by name. You can also paste a link below if you already know where the addon lives."),
            FindPhase::Searching(q) => {
                let q = q.clone();
                ui.centered_and_justified(|ui| {
                    ui.horizontal_centered(|ui| {
                        ui.add(egui::Spinner::new().size(20.0).color(theme::ACCENT_TEXT));
                        ui.label(text(format!("Searching CurseForge for \u{201c}{q}\u{201d}\u{2026}"), 13.0, Weight::Regular, theme::TEXT_SECONDARY));
                    });
                });
            }
            FindPhase::Nothing(q) => {
                let title = format!("Nothing found for \u{201c}{q}\u{201d}");
                message(ui, None, &title, "Try a shorter name or check the spelling. If the addon is on GitHub, paste its address below.");
            }
            FindPhase::Down => {
                message(ui, Some((Icon::Alert, theme::ERROR)), "Search isn\u{2019}t available right now",
                    "The search service didn\u{2019}t answer. You can still install an addon from a link below, or try again in a few minutes.");
                ui.vertical_centered(|ui| {
                    let query = app.find.query.trim().to_string();
                    if ui.add(Button::new(Kind::Secondary, "Try again").icon(Icon::Refresh).enabled(!app.busy() && !query.is_empty())).clicked() {
                        app.run(Cmd::Search { query });
                    }
                });
            }
        }
    });
}

fn message(ui: &mut Ui, icon: Option<(Icon, egui::Color32)>, title: &str, body: &str) {
    let space = (ui.available_height() / 2.0 - 50.0).max(16.0);
    ui.add_space(space);
    ui.vertical_centered(|ui| {
        ui.spacing_mut().item_spacing.y = 8.0;
        ui.set_max_width(400.0);
        if let Some((i, c)) = icon {
            ui.add(i.image(20.0, c));
        }
        ui.label(text(title, 13.0, Weight::SemiBold, theme::TEXT));
        ui.add(egui::Label::new(text(body, 13.0, Weight::Regular, theme::MUTED)).wrap());
    });
}

fn hits(app: &mut App, ui: &mut Ui, width: f32) {
    let FindPhase::Results(data) = &app.find.phase else {
        return;
    };
    ui.spacing_mut().item_spacing.y = 0.0;
    let (bar, _) = ui.allocate_exact_size(Vec2::new(width, 36.0), Sense::hover());
    ui.painter().rect_filled(
        bar,
        CornerRadius {
            nw: 6,
            ne: 6,
            sw: 0,
            se: 0,
        },
        theme::PANEL,
    );
    ui.painter()
        .hline(bar.x_range(), bar.bottom(), Stroke::new(1.0, theme::BORDER));
    in_rect(ui, bar.shrink2(Vec2::new(12.0, 0.0)), Align::Min, |ui| {
        ui.spacing_mut().item_spacing.x = 4.0;
        ui.label(text(
            format!("{} for", copy::plural(data.hits.len(), "result")),
            13.0,
            Weight::Regular,
            theme::MUTED,
        ));
        ui.label(text(
            format!("\u{201c}{}\u{201d}", data.query),
            13.0,
            Weight::Medium,
            theme::TEXT,
        ));
    });
    let cols = [width - 260.0, 120.0, 140.0];
    let split = |r: Rect| {
        let a = r.split_left_right_at_x(r.left() + cols[0]);
        let b = a.1.split_left_right_at_x(a.1.left() + cols[1]);
        [a.0, b.0, b.1]
    };
    let (header, _) = ui.allocate_exact_size(Vec2::new(width, 30.0), Sense::hover());
    ui.painter().rect_filled(header, 0, theme::TABLE_HEADER);
    ui.painter().hline(
        header.x_range(),
        header.bottom(),
        Stroke::new(1.0, theme::BORDER),
    );
    let [h0, h1, _] = split(header);
    in_rect(ui, h0.shrink2(Vec2::new(12.0, 0.0)), Align::Min, |ui| {
        ui.label(text("Addon", 12.0, Weight::Medium, theme::MUTED))
    });
    in_rect(ui, h1.shrink2(Vec2::new(12.0, 0.0)), Align::Max, |ui| {
        ui.label(text("Downloads", 12.0, Weight::Medium, theme::MUTED))
    });

    let note_h = 34.0;
    let mut install = None;
    let installing = app.find.installing.clone();
    let busy = app.busy();
    ScrollArea::vertical().id_salt("hits").auto_shrink([false, false]).max_height(ui.available_height() - note_h).show(ui, |ui| {
        for (i, hit) in data.hits.iter().enumerate() {
            let (rect, _) = ui.allocate_exact_size(Vec2::new(width, 36.0), Sense::hover());
            let base = if i % 2 == 1 { theme::ROW_STRIPE } else { theme::BACKGROUND };
            ui.painter().rect_filled(rect, 0, widgets::row_fill(ui, rect, base));
            let [c0, c1, c2] = split(rect);
            in_rect(ui, c0.shrink2(Vec2::new(12.0, 0.0)), Align::Min, |ui| truncated(ui, text(&hit.title, 13.0, Weight::Medium, theme::TEXT)));
            in_rect(ui, c1.shrink2(Vec2::new(12.0, 0.0)), Align::Max, |ui| match hit.downloads {
                Some(n) => ui.label(mono(copy::downloads(n), theme::TEXT_SECONDARY)).on_hover_text(format!("{n} downloads")),
                None => ui.label(mono("\u{2014}", theme::FAINT)).on_hover_text("Download count not available"),
            });
            in_rect(ui, c2.shrink2(Vec2::new(12.0, 0.0)), Align::Max, |ui| match (&hit.reference, &hit.url) {
                (Some(_), _) if hit.installed => {
                    ui.label(text("Installed", 12.0, Weight::Medium, theme::OK));
                    widgets::icon(ui, Icon::Check, theme::OK);
                }
                (Some(r), _) => {
                    let mine = installing.as_deref() == Some(r.as_str());
                    let label = if mine { "Installing\u{2026}" } else { "Install" };
                    let b = Button::new(Kind::Small, label).icon(Icon::Download).busy(mine).enabled(!busy || mine);
                    if ui.add(b).clicked() {
                        install = Some((r.clone(), hit.title.clone()));
                    }
                }
                (None, Some(url)) => {
                    widgets::link(ui, "Open page", url, Some("Can\u{2019}t be installed automatically. Opens the addon\u{2019}s web page."))
                }
                (None, None) => {}
            });
        }
    });
    let (note, _) = ui.allocate_exact_size(Vec2::new(width, note_h), Sense::hover());
    ui.painter().rect_filled(
        note,
        CornerRadius {
            nw: 0,
            ne: 0,
            sw: 6,
            se: 6,
        },
        theme::PANEL,
    );
    ui.painter()
        .hline(note.x_range(), note.top(), Stroke::new(1.0, theme::BORDER));
    in_rect(ui, note.shrink2(Vec2::new(12.0, 0.0)), Align::Min, |ui| {
        ui.spacing_mut().item_spacing.x = 8.0;
        widgets::icon(ui, Icon::Info, theme::MUTED);
        truncated(
            ui,
            text(
                "Some addons have no version for this game. If so, wam tells you when you install.",
                12.0,
                Weight::Regular,
                theme::MUTED,
            ),
        );
    });
    if let Some((reference, title)) = install {
        app.run(Cmd::Install {
            reference,
            from: Origin::Search { title },
        });
    }
}

fn link_form(app: &mut App, ui: &mut Ui) {
    Frame::new()
        .fill(theme::PANEL)
        .stroke(Stroke::new(1.0, theme::BORDER))
        .corner_radius(6)
        .inner_margin(Margin::symmetric(16, 14))
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            ui.spacing_mut().item_spacing.y = 8.0;
            ui.label(text(
                "Install from a link or name",
                13.0,
                Weight::SemiBold,
                theme::TEXT,
            ));
            let installing = matches!(
                app.running_cmd(),
                Some(Cmd::Install {
                    from: Origin::Link,
                    ..
                })
            );
            ui.horizontal(|ui| {
                let width = (ui.available_width().min(640.0) - 100.0).max(200.0);
                let edit = egui::TextEdit::singleline(&mut app.find.link)
                    .id_salt("link")
                    .font(theme::font(13.0, Weight::Regular));
                let error = matches!(app.find.link_note, Some((_, Tone::Error)));
                let r = widgets::field_with(
                    ui,
                    edit,
                    "Paste a CurseForge or GitHub link, or type a name like details",
                    width,
                    error,
                    10,
                );
                if r.changed() && error {
                    app.find.link_note = None;
                }
                let enter = r.lost_focus() && ui.input(|i| i.key_pressed(Key::Enter));
                let reference = app.find.link.trim().to_string();
                let b = Button::new(Kind::Secondary, "Install")
                    .icon(Icon::Download)
                    .busy(installing)
                    .enabled(!app.busy() || installing);
                if (ui.add(b).clicked() || enter) && !reference.is_empty() && !app.busy() {
                    app.run(Cmd::Install {
                        reference,
                        from: Origin::Link,
                    });
                }
            });
            ui.horizontal(|ui| {
                ui.spacing_mut().item_spacing.x = 4.0;
                ui.label(text("Also works:", 12.0, Weight::Regular, theme::MUTED));
                ui.label(mono("curse:61284", theme::TEXT_SECONDARY));
                ui.label(text("or", 12.0, Weight::Regular, theme::MUTED));
                ui.label(mono("github:Questie/Questie", theme::TEXT_SECONDARY));
            });
            match &app.find.link_note {
                Some((msg, Tone::Error)) => widgets::field_error(ui, msg),
                Some((msg, tone)) => {
                    let color = if *tone == Tone::Ok {
                        theme::OK
                    } else {
                        theme::MUTED
                    };
                    ui.horizontal(|ui| {
                        if *tone == Tone::Ok {
                            widgets::icon(ui, Icon::Check, color);
                        }
                        ui.label(text(msg, 12.0, Weight::Regular, color));
                    });
                }
                None => {}
            }
        });
}
