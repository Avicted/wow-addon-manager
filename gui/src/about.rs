//! The About tab: what wam is, which version, who made it, and where to find the source.

use eframe::egui::{self, Align, Rect, Ui, Vec2};

use crate::{
    theme::{self, Icon, Weight},
    wam,
    widgets::{self, in_rect, mono, text},
};

const REPO: &str = "https://github.com/Avicted/wow-addon-manager";
const AUTHOR_PAGE: &str = "https://github.com/Avicted";

pub fn show(ui: &mut Ui) {
    ui.vertical_centered(|ui| {
        ui.set_max_width(480.0);
        ui.add_space((ui.available_height() / 2.0 - 140.0).max(24.0));
        ui.spacing_mut().item_spacing.y = 8.0;
        ui.add(Icon::Logo.image(48.0, theme::ACCENT_TEXT));
        ui.label(text("wam", 22.0, Weight::SemiBold, theme::TEXT));
        ui.label(mono(format!("v{}", wam::VERSION), theme::TEXT_SECONDARY));
        ui.label(text(
            "Keeps your World of Warcraft Forever 1.60 addons up to date.",
            13.0,
            Weight::Regular,
            theme::MUTED,
        ));
        ui.add_space(8.0);
        ui.label(text(
            "Made by Victor Anderss\u{e9}n",
            13.0,
            Weight::Medium,
            theme::TEXT,
        ));
        centred_link(ui, "Source code on GitHub", REPO);
        centred_link(ui, "github.com/Avicted", AUTHOR_PAGE);
        ui.add_space(8.0);
        ui.label(text("MIT licence", 12.0, Weight::Regular, theme::MUTED));
    });
}

/// `widgets::link` fills the width it is given, so place it in a rect of its own width.
fn centred_link(ui: &mut Ui, label: &str, url: &str) {
    let galley = ui.painter().layout_no_wrap(
        label.to_string(),
        theme::font(12.0, Weight::Medium),
        theme::ACCENT_TEXT,
    );
    let size = Vec2::new(galley.size().x + 4.0 + 16.0, 20.0);
    let (row, _) = ui.allocate_exact_size(
        Vec2::new(ui.available_width(), size.y),
        egui::Sense::hover(),
    );
    in_rect(
        ui,
        Rect::from_center_size(row.center(), size),
        Align::Min,
        |ui| widgets::link(ui, label, url, None),
    );
}
