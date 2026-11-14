//! wam-gui: a desktop window over the wam command-line tool, which it carries inside itself.
//! The design is in docs/gui-design/; how the two talk is in internal/cli/json.go.

#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")] // no console window on Windows

mod about;
mod addons;
mod app;
mod copy;
mod find;
mod settings;
mod theme;
mod wam;
mod widgets;

use eframe::egui::{
    self, Align, Frame, Key, KeyboardShortcut, Layout, Margin, Modifiers, ScrollArea, Sense,
    Stroke, Ui, Vec2,
};

use app::{App, Cmd, Tab, Tone};
use theme::{Icon, Weight};
use widgets::{Button, Kind, mono, text, truncated};

/// OpenGL on Linux; wgpu on Windows, where OpenGL support varies too much between drivers.
fn renderer() -> eframe::Renderer {
    #[cfg(windows)]
    return eframe::Renderer::Wgpu;
    #[cfg(not(windows))]
    return eframe::Renderer::Glow;
}

fn main() -> eframe::Result {
    let options = eframe::NativeOptions {
        viewport: egui::ViewportBuilder::default()
            .with_title(format!("wam | v{}", wam::VERSION))
            .with_app_id("wam")
            .with_inner_size([960.0, 640.0])
            .with_min_inner_size([720.0, 480.0]),
        renderer: renderer(),
        // Beside wam's own state, so WAM_DATA_DIR moves both.
        persistence_path: Some(wam::data_dir().join("gui")),
        ..Default::default()
    };
    eframe::run_native(
        "wam",
        options,
        Box::new(|cc| {
            theme::apply(&cc.egui_ctx);
            egui_extras::install_image_loaders(&cc.egui_ctx);
            let settings = cc
                .storage
                .and_then(|s| eframe::get_value(s, eframe::APP_KEY))
                .unwrap_or_default();
            let bin = wam::unpack(&wam::data_dir())
                .map_err(|e| format!("Couldn\u{2019}t prepare wam: {e}"));
            Ok(Box::new(Shell(App::new(
                cc.egui_ctx.clone(),
                settings,
                bin,
            ))))
        }),
    )
}

/// The window around the tabs: top bar, bottom strip and activity log.
struct Shell(App);

impl eframe::App for Shell {
    fn save(&mut self, storage: &mut dyn eframe::Storage) {
        eframe::set_value(storage, eframe::APP_KEY, &self.0.settings);
    }

    fn ui(&mut self, ui: &mut Ui, _frame: &mut eframe::Frame) {
        draw(&mut self.0, ui);
    }
}

/// One frame of the whole window.
fn draw(app: &mut App, ui: &mut Ui) {
    {
        app.poll();
        shortcuts(app, ui.ctx());

        egui::Panel::top("top")
            .resizable(false)
            .exact_size(44.0)
            .frame(Frame::new().fill(theme::PANEL).inner_margin(Margin {
                left: 16,
                right: 12,
                top: 0,
                bottom: 0,
            }))
            .show(ui, |ui| top_bar(app, ui));
        egui::Panel::bottom("strip")
            .resizable(false)
            .exact_size(30.0)
            .frame(Frame::new().fill(theme::PANEL).inner_margin(Margin {
                left: 16,
                right: 8,
                top: 0,
                bottom: 0,
            }))
            .show(ui, |ui| strip(app, ui));
        if app.log_open {
            egui::Panel::bottom("log")
                .resizable(false)
                .exact_size(132.0)
                .frame(
                    Frame::new()
                        .fill(theme::LOG_BG)
                        .inner_margin(Margin::symmetric(0, 6)),
                )
                .show(ui, |ui| log(app, ui));
        }
        egui::CentralPanel::no_frame()
            .frame(Frame::new().fill(theme::BACKGROUND))
            .show(ui, |ui| match app.tab {
                Tab::Addons => addons::show(app, ui),
                Tab::Find => find::show(app, ui),
                Tab::Settings => settings::show(app, ui),
                Tab::About => about::show(ui),
            });
        addons::dialogs(app, ui.ctx());
    }
}

#[cfg(test)]
mod ui_tests;

fn shortcuts(app: &mut App, ctx: &egui::Context) {
    if app.dialog.is_some() {
        return;
    }
    let key = |k| KeyboardShortcut::new(Modifiers::COMMAND, k);
    let pressed = |k| ctx.input_mut(|i| i.consume_shortcut(&key(k)));
    let can_switch = !app.busy() || matches!(app.running_cmd(), Some(Cmd::Search { .. }));
    for (k, tab) in [
        (Key::Num1, Tab::Addons),
        (Key::Num2, Tab::Find),
        (Key::Num3, Tab::Settings),
        (Key::Num4, Tab::About),
    ] {
        if pressed(k) && can_switch && tab_enabled(app, tab) {
            app.tab = tab;
        }
    }
    if pressed(Key::R) && !app.busy() {
        app.run(Cmd::Status { rescan: false });
    }
    if pressed(Key::F) && can_switch && tab_enabled(app, Tab::Find) {
        app.tab = Tab::Find;
        app.focus_search = true;
    }
}

fn tab_enabled(app: &App, tab: Tab) -> bool {
    !(tab == Tab::Find && app.first_run.is_some())
}

fn top_bar(app: &mut App, ui: &mut Ui) {
    ui.horizontal_centered(|ui| {
        ui.spacing_mut().item_spacing.x = 8.0;
        ui.add(Icon::Logo.image(18.0, theme::ACCENT_TEXT));
        ui.label(text("wam", 14.0, Weight::SemiBold, theme::TEXT));
        ui.add_space(8.0);
        ui.spacing_mut().item_spacing.x = 0.0;
        // One action at a time: the tabs rest while an update or install runs.
        let locked = app.busy()
            && !matches!(
                app.running_cmd(),
                Some(Cmd::Search { .. }) | Some(Cmd::Status { .. })
            );
        let count = app.rows.len();
        for (tab, label) in [
            (Tab::Addons, "Addons"),
            (Tab::Find, "Find addons"),
            (Tab::Settings, "Settings"),
            (Tab::About, "About"),
        ] {
            let enabled = !locked && tab_enabled(app, tab);
            if tab_button(
                ui,
                label,
                (tab == Tab::Addons).then_some(count),
                app.tab == tab,
                enabled,
            ) {
                app.tab = tab;
            }
        }
    });
}

/// A tab: 13 px label, a count pill on Addons, and a 2 px accent underline when active.
fn tab_button(ui: &mut Ui, label: &str, count: Option<usize>, active: bool, enabled: bool) -> bool {
    let weight = if active {
        Weight::SemiBold
    } else {
        Weight::Medium
    };
    let mut color = if active { theme::TEXT } else { theme::MUTED };
    let galley = ui
        .painter()
        .layout_no_wrap(label.to_string(), theme::font(13.0, weight), color);
    let pill_w = count.map_or(0.0, |n| {
        6.0 + (n.to_string().len() as f32 * 7.0 + 12.0).max(20.0)
    });
    let size = Vec2::new(24.0 + galley.size().x + pill_w, 44.0);
    let sense = if enabled {
        Sense::click()
    } else {
        Sense::hover()
    };
    let (rect, resp) = ui.allocate_exact_size(size, sense);
    resp.widget_info(|| {
        egui::WidgetInfo::selected(egui::accesskit::Role::Tab, enabled, active, label)
    });
    if resp.hovered() && enabled && !active {
        color = theme::TEXT_SECONDARY;
    }
    let opacity = if enabled {
        1.0
    } else {
        theme::DISABLED_OPACITY
    };
    let p = ui.painter().clone();
    let x = rect.left() + 12.0;
    p.galley(
        egui::pos2(x, rect.center().y - galley.size().y / 2.0),
        galley.clone(),
        color.gamma_multiply(opacity),
    );
    if let Some(n) = count {
        let mut pill = ui.new_child(
            egui::UiBuilder::new().max_rect(egui::Rect::from_center_size(
                egui::pos2(
                    x + galley.size().x + 6.0 + (pill_w - 6.0) / 2.0,
                    rect.center().y,
                ),
                Vec2::new(pill_w - 6.0, 18.0),
            )),
        );
        pill.set_opacity(opacity);
        widgets::pill(&mut pill, n, &theme::PILL_NEUTRAL);
    }
    if active {
        let line = egui::Rect::from_min_max(
            egui::pos2(rect.left(), rect.bottom() - 2.0),
            rect.right_bottom(),
        );
        p.rect_filled(line, 0, theme::PROGRESS_FILL.gamma_multiply(opacity));
    }
    if resp.has_focus() {
        p.rect_stroke(
            rect.shrink(4.0),
            4,
            Stroke::new(2.0, theme::FOCUS),
            egui::StrokeKind::Inside,
        );
    }
    resp.clicked()
}

fn strip(app: &mut App, ui: &mut Ui) {
    ui.horizontal_centered(|ui| {
        let (msg, tone) = &app.outcome;
        let color = match tone {
            Tone::Muted => theme::MUTED,
            Tone::Ok => theme::OK,
            Tone::Warning => theme::WARNING,
            Tone::Error => theme::ERROR,
        };
        widgets::icon(ui, Icon::Info, color);
        let w = ui.available_width() - 110.0;
        ui.allocate_ui_with_layout(
            Vec2::new(w, 30.0),
            Layout::left_to_right(Align::Center),
            |ui| {
                truncated(ui, text(msg, 12.0, Weight::Regular, theme::TEXT_SECONDARY));
            },
        );
        ui.with_layout(Layout::right_to_left(Align::Center), |ui| {
            let label = format!("Activity ({})", app.log.len());
            let chevron = if app.log_open {
                Icon::ChevronDown
            } else {
                Icon::ChevronUp
            };
            // The design puts the chevron after the label; a right-to-left row puts it first.
            ui.spacing_mut().item_spacing.x = 0.0;
            if ui
                .add(Button::new(Kind::Ghost, "").icon(chevron).tooltip(&label))
                .clicked()
                | ui.add(Button::new(Kind::Ghost, &label)).clicked()
            {
                app.log_open = !app.log_open;
            }
        });
    });
}

fn log(app: &App, ui: &mut Ui) {
    ScrollArea::vertical()
        .auto_shrink([false, false])
        .stick_to_bottom(true)
        .show(ui, |ui| {
            ui.spacing_mut().item_spacing.y = 0.0;
            if app.log.is_empty() {
                Frame::new()
                    .inner_margin(Margin::symmetric(16, 3))
                    .show(ui, |ui| {
                        ui.label(text("Nothing yet.", 12.0, Weight::Regular, theme::MUTED));
                    });
            }
            for l in &app.log {
                Frame::new()
                    .inner_margin(Margin::symmetric(16, 3))
                    .show(ui, |ui| {
                        ui.horizontal(|ui| {
                            ui.spacing_mut().item_spacing.x = 10.0;
                            ui.label(mono(&l.time, theme::FAINT));
                            let (level, color) = match l.tone {
                                Tone::Warning => ("Warning", theme::WARNING),
                                Tone::Error => ("Error", theme::ERROR),
                                _ => ("Info", theme::MUTED),
                            };
                            ui.add_sized(
                                [60.0, 16.0],
                                egui::Label::new(text(level, 12.0, Weight::Medium, color)),
                            );
                            ui.add(
                                egui::Label::new(text(
                                    &l.message,
                                    12.0,
                                    Weight::Regular,
                                    theme::TEXT_SECONDARY,
                                ))
                                .wrap(),
                            );
                        });
                    });
            }
        });
}
