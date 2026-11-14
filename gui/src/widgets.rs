//! The design's components (docs/gui-design/components.md) as egui widgets.

use eframe::egui::{
    self, Align, Color32, CornerRadius, Galley, Layout, Rect, Response, RichText, Sense, Stroke,
    StrokeKind, Ui, UiBuilder, Vec2, Widget, WidgetInfo, accesskit::Role,
};
use std::sync::Arc;

use crate::theme::{self, Icon, Tint, Weight, font};

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Kind {
    Primary,
    /// Refresh / Try again: a green button for the safe, repeatable action.
    Success,
    Secondary,
    Danger,
    /// Row actions: Update, Track…, Fix.
    Small,
    /// The bottom strip's Activity toggle.
    Ghost,
    /// A 26×26 button with only an icon, like a row's Remove.
    IconOnly,
}

/// A button as the design draws it. `ui.add(Button::new(Kind::Primary, "Update all"))`.
pub struct Button<'a> {
    kind: Kind,
    label: &'a str,
    icon: Option<Icon>,
    enabled: bool,
    busy: bool,
    tooltip: Option<&'a str>,
}

impl<'a> Button<'a> {
    pub fn new(kind: Kind, label: &'a str) -> Self {
        Button {
            kind,
            label,
            icon: None,
            enabled: true,
            busy: false,
            tooltip: None,
        }
    }

    pub fn icon(mut self, icon: Icon) -> Self {
        self.icon = Some(icon);
        self
    }

    pub fn enabled(mut self, enabled: bool) -> Self {
        self.enabled = enabled;
        self
    }

    /// A spinner replaces the icon and the button can't be pressed.
    pub fn busy(mut self, busy: bool) -> Self {
        self.busy = busy;
        self
    }

    /// For icon-only buttons: the tooltip and the screen-reader label.
    pub fn tooltip(mut self, tooltip: &'a str) -> Self {
        self.tooltip = Some(tooltip);
        self
    }

    /// The width `ui.add` will give this button, for centring a row of them.
    pub fn width(&self, ui: &Ui) -> f32 {
        self.measure(ui).4
    }

    /// Height, side padding, label galley, icon width (with its gap) and total width.
    fn measure(&self, ui: &Ui) -> (f32, f32, Option<Arc<Galley>>, f32, f32) {
        let (height, pad, size) = match self.kind {
            Kind::Small => (26.0, 10.0, 12.0),
            Kind::Ghost => (24.0, 8.0, 12.0),
            Kind::IconOnly => (26.0, 5.0, 12.0),
            _ => (30.0, 12.0, 13.0),
        };
        let galley = (!self.label.is_empty() && self.kind != Kind::IconOnly).then(|| {
            ui.painter().layout_no_wrap(
                self.label.to_string(),
                font(size, Weight::Medium),
                Color32::WHITE,
            )
        });
        let text_w = galley.as_ref().map_or(0.0, |g| g.size().x);
        let icon_w = if self.icon.is_some() || self.busy {
            16.0 + if galley.is_some() { 6.0 } else { 0.0 }
        } else {
            0.0
        };
        let width = if self.kind == Kind::IconOnly {
            26.0
        } else {
            pad * 2.0 + icon_w + text_w
        };
        (height, pad, galley, icon_w, width)
    }
}

impl Widget for Button<'_> {
    fn ui(self, ui: &mut Ui) -> Response {
        let (height, pad, galley, icon_w, width) = self.measure(ui);
        let interactive = self.enabled && !self.busy;
        let has_icon = self.icon.is_some() || self.busy;
        let sense = if interactive {
            Sense::click()
        } else {
            Sense::hover()
        };
        let (rect, response) = ui.allocate_exact_size(Vec2::new(width, height), sense);
        let label = self.tooltip.unwrap_or(self.label).to_string();
        response.widget_info(|| WidgetInfo::labeled(Role::Button, interactive, &label));

        if ui.is_rect_visible(rect) {
            let pressed = response.is_pointer_button_down_on();
            let hovered = response.hovered() && interactive;
            let white = Color32::WHITE;
            let clear = Color32::TRANSPARENT;
            let (fill, border, text) = match self.kind {
                Kind::Primary => {
                    let f = pick(
                        pressed,
                        hovered,
                        theme::ACCENT_FILL,
                        theme::PRIMARY_HOVER,
                        theme::PRIMARY_PRESSED,
                    );
                    (f, f, white)
                }
                Kind::Success => {
                    let f = pick(
                        pressed,
                        hovered,
                        theme::SUCCESS_FILL,
                        theme::SUCCESS_HOVER,
                        theme::SUCCESS_PRESSED,
                    );
                    (f, f, white)
                }
                Kind::Danger => {
                    let f = pick(
                        pressed,
                        hovered,
                        theme::DANGER_FILL,
                        theme::DANGER_HOVER,
                        theme::DANGER_PRESSED,
                    );
                    (f, f, white)
                }
                Kind::Secondary | Kind::Small => (
                    pick(
                        pressed,
                        hovered,
                        theme::CONTROL,
                        theme::SECONDARY_HOVER,
                        theme::SECONDARY_PRESSED,
                    ),
                    theme::CONTROL_BORDER,
                    theme::TEXT,
                ),
                Kind::Ghost => (
                    pick(
                        pressed,
                        hovered,
                        clear,
                        theme::CONTROL,
                        theme::SECONDARY_PRESSED,
                    ),
                    clear,
                    theme::TEXT_SECONDARY,
                ),
                Kind::IconOnly => (
                    pick(
                        pressed,
                        hovered,
                        clear,
                        theme::CONTROL,
                        theme::SECONDARY_PRESSED,
                    ),
                    clear,
                    if hovered { theme::TEXT } else { theme::MUTED },
                ),
            };
            let opacity = match (self.enabled, self.busy) {
                (false, _) => theme::DISABLED_OPACITY,
                (true, true) if !matches!(self.kind, Kind::Primary | Kind::Success) => 0.8,
                _ => 1.0,
            };
            let [fill, border, text] = [fill, border, text].map(|c| c.gamma_multiply(opacity));
            let p = ui.painter();
            p.rect(
                rect,
                CornerRadius::same(4),
                fill,
                Stroke::new(1.0, border),
                StrokeKind::Inside,
            );
            let mut x = if self.kind == Kind::IconOnly {
                rect.center().x - 8.0
            } else {
                rect.left() + pad
            };
            if has_icon {
                let r =
                    Rect::from_min_size(egui::pos2(x, rect.center().y - 8.0), Vec2::splat(16.0));
                if self.busy {
                    egui::Spinner::new().size(16.0).color(text).paint_at(ui, r);
                } else if let Some(icon) = self.icon {
                    icon.paint(ui, r, text);
                }
                x += icon_w;
            }
            if let Some(g) = galley {
                p.galley(egui::pos2(x, rect.center().y - g.size().y / 2.0), g, text);
            }
            if response.has_focus() {
                focus_ring(ui, rect);
            }
        }
        match self.tooltip {
            Some(t) if self.kind == Kind::IconOnly => response.on_hover_text(t),
            _ => response,
        }
    }
}

fn pick(pressed: bool, hovered: bool, normal: Color32, hover: Color32, press: Color32) -> Color32 {
    if pressed {
        press
    } else if hovered {
        hover
    } else {
        normal
    }
}

/// The 2 px focus outline, 2 px outside `rect`.
pub fn focus_ring(ui: &Ui, rect: Rect) {
    ui.painter().rect_stroke(
        rect.expand(2.0),
        CornerRadius::same(6),
        Stroke::new(2.0, theme::FOCUS),
        StrokeKind::Outside,
    );
}

/// A label in a given size, weight and colour.
pub fn text(s: impl Into<String>, size: f32, weight: Weight, color: Color32) -> RichText {
    RichText::new(s).font(font(size, weight)).color(color)
}

/// Version strings, paths and refs.
pub fn mono(s: impl Into<String>, color: Color32) -> RichText {
    RichText::new(s).font(theme::mono()).color(color)
}

/// A label that ends in "…" when it doesn't fit and shows the whole text on hover.
pub fn truncated(ui: &mut Ui, text: RichText) -> Response {
    ui.add(egui::Label::new(text).truncate())
}

/// Runs `add` in `rect`, laid out left to right and clipped to it.
pub fn in_rect<R>(ui: &mut Ui, rect: Rect, align: Align, add: impl FnOnce(&mut Ui) -> R) -> R {
    let layout = match align {
        Align::Max => Layout::right_to_left(Align::Center),
        _ => Layout::left_to_right(Align::Center),
    };
    // A bare child Ui, so the parent's layout cursor is never moved by the cell's content.
    let mut child = ui.new_child(UiBuilder::new().max_rect(rect).layout(layout));
    child.set_clip_rect(rect.intersect(ui.clip_rect()));
    child.spacing_mut().item_spacing.x = 6.0;
    add(&mut child)
}

/// A table row's background: `base`, or the hover colour while the pointer is over `rect`.
/// Unlike `Response::hovered` this stays on while a mouse button is held down.
pub fn row_fill(ui: &Ui, rect: Rect, base: Color32) -> Color32 {
    if ui.rect_contains_pointer(rect) {
        theme::ROW_HOVER
    } else {
        base
    }
}

/// A count in a rounded pill.
pub fn pill(ui: &mut Ui, count: usize, tint: &Tint) {
    let galley =
        ui.painter()
            .layout_no_wrap(count.to_string(), font(11.0, Weight::SemiBold), tint.text);
    let w = (galley.size().x + 12.0).max(20.0);
    let (rect, _) = ui.allocate_exact_size(Vec2::new(w, 18.0), Sense::hover());
    ui.painter()
        .rect_filled(rect, CornerRadius::same(9), tint.bg);
    ui.painter()
        .galley(rect.center() - galley.size() / 2.0, galley, tint.text);
}

/// An icon at 16 px in the flow of a layout.
pub fn icon(ui: &mut Ui, icon: Icon, color: Color32) {
    ui.add(icon.image(16.0, color));
}

/// A row's status: icon, word in the state's colour, and an optional muted detail.
pub fn status(ui: &mut Ui, look: &StatusLook, detail: &str) {
    match look.icon {
        None => {
            ui.add(egui::Spinner::new().size(16.0).color(look.color));
        }
        Some(i) => icon(ui, i, look.color),
    }
    ui.label(text(look.label, 13.0, Weight::Medium, look.color));
    if !detail.is_empty() {
        truncated(ui, text(detail, 13.0, Weight::Regular, theme::MUTED));
    }
}

/// How a status is drawn. `icon: None` means a spinner.
pub struct StatusLook {
    pub label: &'static str,
    pub color: Color32,
    pub icon: Option<Icon>,
}

pub fn status_look(state: &str) -> StatusLook {
    let (label, color, icon) = match state {
        "ok" => ("Up to date", theme::OK, Some(Icon::Check)),
        "update" => ("Update available", theme::ACCENT_TEXT, Some(Icon::Update)),
        "error" => ("Couldn\u{2019}t check", theme::ERROR, Some(Icon::Alert)),
        "checking" => ("Checking\u{2026}", theme::MUTED, None),
        "updating" => ("Updating\u{2026}", theme::ACCENT_TEXT, None),
        "waiting" => ("Waiting", theme::MUTED, Some(Icon::Clock)),
        "updated" => ("Updated", theme::OK, Some(Icon::Check)),
        "failed" => ("Failed", theme::ERROR, Some(Icon::Alert)),
        "skipped" => ("Skipped", theme::MUTED, Some(Icon::Skip)),
        _ => ("Not checked", theme::MUTED, Some(Icon::Skip)),
    };
    StatusLook { label, color, icon }
}

/// A 4 px progress bar, at most 420 wide.
pub fn progress(ui: &mut Ui, fraction: f32) {
    let w = ui.available_width().min(420.0);
    let (rect, _) = ui.allocate_exact_size(Vec2::new(w, 4.0), Sense::hover());
    let r = CornerRadius::same(2);
    ui.painter().rect_filled(rect, r, theme::PROGRESS_TRACK);
    let mut fill = rect;
    fill.set_width(rect.width() * fraction.clamp(0.0, 1.0));
    ui.painter().rect_filled(fill, r, theme::PROGRESS_FILL);
}

/// A full-width banner: icon and one sentence.
pub fn banner(ui: &mut Ui, tint: &Tint, icon_: Icon, message: &str) {
    egui::Frame::new()
        .fill(tint.bg)
        .stroke(Stroke::new(1.0, tint.border))
        .corner_radius(6)
        .inner_margin(egui::Margin::symmetric(12, 8))
        .show(ui, |ui| {
            ui.set_width(ui.available_width());
            ui.horizontal(|ui| {
                icon(ui, icon_, tint.text);
                ui.add(egui::Label::new(text(message, 13.0, Weight::Regular, tint.text)).wrap());
            });
        });
}

/// A 32 px single-line text field. `error` draws the error border.
pub fn field(ui: &mut Ui, value: &mut String, hint: &str, width: f32, error: bool) -> Response {
    let edit = egui::TextEdit::singleline(value).font(font(13.0, Weight::Regular));
    field_with(ui, edit, hint, width, error, 10)
}

/// Like [`field`], for a TextEdit configured by the caller (password, left padding for an icon).
pub fn field_with(
    ui: &mut Ui,
    edit: egui::TextEdit<'_>,
    hint: &str,
    width: f32,
    error: bool,
    left_pad: i8,
) -> Response {
    ui.scope(|ui| {
        if error {
            let w = &mut ui.visuals_mut().widgets;
            w.inactive.bg_stroke.color = theme::FIELD_ERROR_BORDER;
            w.hovered.bg_stroke.color = theme::FIELD_ERROR_BORDER;
        }
        let r = ui.add(
            edit.hint_text(text(hint, 13.0, Weight::Regular, theme::FAINT))
                .margin(egui::Margin {
                    left: left_pad,
                    right: 10,
                    top: 8,
                    bottom: 8,
                })
                .min_size(Vec2::new(0.0, 32.0))
                .desired_width(width),
        );
        if r.has_focus() {
            ui.painter().rect_stroke(
                r.rect.expand(1.0),
                CornerRadius::same(5),
                Stroke::new(2.0, Color32::from_rgba_unmultiplied(76, 141, 240, 90)),
                StrokeKind::Outside,
            );
        }
        r
    })
    .inner
}

/// The 12 px error line under a field.
pub fn field_error(ui: &mut Ui, message: &str) {
    ui.horizontal(|ui| {
        icon(ui, Icon::Alert, theme::ERROR);
        ui.add(egui::Label::new(text(message, 12.0, Weight::Regular, theme::ERROR)).wrap());
    });
}

/// A link that opens `url` in the browser, with an external-link icon after it.
pub fn link(ui: &mut Ui, label: &str, url: &str, tooltip: Option<&str>) {
    let r = ui
        .horizontal(|ui| {
            ui.spacing_mut().item_spacing.x = 4.0;
            let r = ui.add(
                egui::Label::new(text(label, 12.0, Weight::Medium, theme::ACCENT_TEXT))
                    .sense(Sense::click()),
            );
            icon(ui, Icon::External, theme::ACCENT_TEXT);
            r
        })
        .inner
        .on_hover_cursor(egui::CursorIcon::PointingHand);
    let r = match tooltip {
        Some(t) => r.on_hover_text(t),
        None => r,
    };
    if r.clicked() {
        ui.ctx().open_url(egui::OpenUrl::new_tab(url));
    }
}
