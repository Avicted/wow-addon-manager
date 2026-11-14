//! The design tokens (docs/gui-design/tokens.json, dark theme) as egui colours, fonts and icons.

use std::{
    collections::HashMap,
    sync::{Arc, LazyLock, Mutex},
};

use eframe::egui::{
    self, Color32, CornerRadius, FontData, FontDefinitions, FontFamily, FontId, Shadow, Stroke,
    TextStyle, Vec2, load::Bytes,
};

const fn hex(rgb: u32) -> Color32 {
    Color32::from_rgb((rgb >> 16) as u8, (rgb >> 8) as u8, rgb as u8)
}

pub const BACKGROUND: Color32 = hex(0x16191C);
pub const PANEL: Color32 = hex(0x1A1D21);
pub const ROW_STRIPE: Color32 = hex(0x1A1D21);
pub const RAISED: Color32 = hex(0x1F2327);
pub const CONTROL: Color32 = hex(0x262A30);
pub const BORDER: Color32 = hex(0x2E3339);
pub const CONTROL_BORDER: Color32 = hex(0x3A4048);
pub const TEXT: Color32 = hex(0xE7E9EC);
pub const TEXT_SECONDARY: Color32 = hex(0xC9CED4);
pub const MUTED: Color32 = hex(0x9BA3AD);
pub const FAINT: Color32 = hex(0x8A929C);
pub const ACCENT_FILL: Color32 = hex(0x2F6FD0);
pub const ACCENT_TEXT: Color32 = hex(0x7FB0FA);
pub const OK: Color32 = hex(0x5BC58A);
pub const WARNING: Color32 = hex(0xE8B04B);
pub const ERROR: Color32 = hex(0xF07A6E);
pub const DANGER_FILL: Color32 = hex(0xB83A30);
pub const SELECTION: Color32 = hex(0x23426E);
pub const FOCUS: Color32 = hex(0x7FB0FA);
pub const TABLE_HEADER: Color32 = hex(0x1E2226);
pub const ROW_HOVER: Color32 = hex(0x22272D);
pub const LOG_BG: Color32 = hex(0x121417);
pub const FIELD_BG: Color32 = hex(0x121417);
pub const FIELD_HOVER_BORDER: Color32 = hex(0x4A515A);
pub const FIELD_ERROR_BORDER: Color32 = hex(0xB5544B);
pub const PROGRESS_TRACK: Color32 = hex(0x2A2F35);
pub const PROGRESS_FILL: Color32 = hex(0x4C8DF0);
pub const DIALOG_BORDER: Color32 = hex(0x343A41);
pub const LIST_DIVIDER: Color32 = hex(0x23272C);
pub const SCRIM: Color32 = Color32::from_rgba_premultiplied(5, 6, 7, 158); // rgba(8,10,12,.62)
pub const PRIMARY_HOVER: Color32 = hex(0x3A7BDD);
pub const PRIMARY_PRESSED: Color32 = hex(0x2860B8);
pub const SECONDARY_HOVER: Color32 = hex(0x2D3238);
pub const SECONDARY_PRESSED: Color32 = hex(0x1F2328);
pub const DANGER_HOVER: Color32 = hex(0xC4463B);
pub const DANGER_PRESSED: Color32 = hex(0xA03228);

/// A banner's or pill's background, border and text colours.
pub struct Tint {
    pub bg: Color32,
    pub border: Color32,
    pub text: Color32,
}

pub const BANNER_WARNING: Tint = Tint {
    bg: hex(0x2A2416),
    border: hex(0x4A3D1F),
    text: hex(0xF0C979),
};
pub const BANNER_ERROR: Tint = Tint {
    bg: hex(0x2C1A19),
    border: hex(0x55302C),
    text: hex(0xF4A79F),
};
pub const PILL_NEUTRAL: Tint = Tint {
    bg: hex(0x2A2F35),
    border: hex(0x2A2F35),
    text: hex(0xC9CED4),
};
pub const PILL_WARNING: Tint = Tint {
    bg: hex(0x3A2A14),
    border: hex(0x3A2A14),
    text: hex(0xF0C979),
};
pub const PILL_ERROR: Tint = Tint {
    bg: hex(0x3A1F1D),
    border: hex(0x3A1F1D),
    text: hex(0xF4A79F),
};

pub const DISABLED_OPACITY: f32 = 0.45;

/// Inter weights and the mono face, as font ids.
#[derive(Clone, Copy)]
pub enum Weight {
    Regular,
    Medium,
    SemiBold,
}

pub fn font(size: f32, weight: Weight) -> FontId {
    let family = match weight {
        Weight::Regular => FontFamily::Proportional,
        Weight::Medium => FontFamily::Name("medium".into()),
        Weight::SemiBold => FontFamily::Name("semibold".into()),
    };
    FontId::new(size, family)
}

pub fn mono() -> FontId {
    FontId::new(12.0, FontFamily::Monospace)
}

fn fonts() -> FontDefinitions {
    let mut defs = FontDefinitions::default();
    let mut add = |name: &str, bytes: &'static [u8]| {
        defs.font_data
            .insert(name.into(), Arc::new(FontData::from_static(bytes)));
    };
    add("inter", include_bytes!("../assets/fonts/Inter-Regular.ttf"));
    add(
        "inter-medium",
        include_bytes!("../assets/fonts/Inter-Medium.ttf"),
    );
    add(
        "inter-semibold",
        include_bytes!("../assets/fonts/Inter-SemiBold.ttf"),
    );
    add(
        "jetbrains-mono",
        include_bytes!("../assets/fonts/JetBrainsMono-Regular.ttf"),
    );
    // egui's own fonts stay as fallbacks for glyphs these lack.
    let fallbacks = defs.families[&FontFamily::Proportional].clone();
    let mut family = |family: FontFamily, first: &str| {
        let mut list = vec![first.to_string()];
        list.extend(fallbacks.iter().cloned());
        defs.families.insert(family, list);
    };
    family(FontFamily::Proportional, "inter");
    family(FontFamily::Name("medium".into()), "inter-medium");
    family(FontFamily::Name("semibold".into()), "inter-semibold");
    family(FontFamily::Monospace, "jetbrains-mono");
    defs
}

/// Installs fonts, the dark visuals and the spacing.
pub fn apply(ctx: &egui::Context) {
    ctx.set_fonts(fonts());
    ctx.set_theme(egui::Theme::Dark);
    ctx.all_styles_mut(|style| {
        style.text_styles = [
            (TextStyle::Heading, font(18.0, Weight::SemiBold)),
            (TextStyle::Body, font(13.0, Weight::Regular)),
            (TextStyle::Button, font(13.0, Weight::Medium)),
            (TextStyle::Small, font(12.0, Weight::Regular)),
            (TextStyle::Monospace, mono()),
        ]
        .into();
        style.url_in_tooltip = false;
        // Text sits on top of clickable rows and headers; selectable labels would take the clicks.
        style.interaction.selectable_labels = false;
        let s = &mut style.spacing;
        s.item_spacing = Vec2::new(8.0, 8.0);
        s.button_padding = Vec2::new(12.0, 6.0);

        let v = &mut style.visuals;
        v.dark_mode = true;
        v.override_text_color = None;
        v.panel_fill = BACKGROUND;
        v.window_fill = RAISED;
        v.window_stroke = Stroke::new(1.0, DIALOG_BORDER);
        v.window_corner_radius = CornerRadius::same(8);
        v.window_shadow = Shadow {
            offset: [0, 12],
            blur: 32,
            spread: 0,
            color: Color32::from_black_alpha(128),
        };
        v.extreme_bg_color = FIELD_BG;
        v.text_edit_bg_color = Some(FIELD_BG);
        v.faint_bg_color = ROW_STRIPE;
        v.hyperlink_color = ACCENT_TEXT;
        v.warn_fg_color = WARNING;
        v.error_fg_color = ERROR;
        v.selection.bg_fill = SELECTION;
        v.selection.stroke = Stroke::new(1.0, PROGRESS_FILL); // focused text field border
        let r = CornerRadius::same(4);
        let w = &mut v.widgets;
        w.noninteractive.bg_fill = PANEL;
        w.noninteractive.weak_bg_fill = PANEL;
        w.noninteractive.bg_stroke = Stroke::new(1.0, BORDER);
        w.noninteractive.fg_stroke = Stroke::new(1.0, TEXT);
        w.noninteractive.corner_radius = r;
        for (wv, fill, stroke) in [
            (&mut w.inactive, CONTROL, CONTROL_BORDER),
            (&mut w.hovered, SECONDARY_HOVER, FIELD_HOVER_BORDER),
            (&mut w.active, SECONDARY_PRESSED, PROGRESS_FILL),
            (&mut w.open, CONTROL, CONTROL_BORDER),
        ] {
            wv.bg_fill = fill;
            wv.weak_bg_fill = fill;
            wv.bg_stroke = Stroke::new(1.0, stroke);
            wv.fg_stroke = Stroke::new(1.0, TEXT);
            wv.corner_radius = r;
            wv.expansion = 0.0;
        }
    });
}

/// The design's single-colour icons.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Icon {
    Alert,
    Check,
    ChevronDown,
    ChevronRight,
    ChevronUp,
    Clock,
    Download,
    External,
    Eye,
    Folder,
    Info,
    Logo,
    Refresh,
    Search,
    Skip,
    Trash,
    Update,
    X,
}

impl Icon {
    fn svg(self) -> (&'static str, &'static str) {
        macro_rules! svg {
            ($name:literal) => {
                (
                    $name,
                    include_str!(concat!("../assets/icons/", $name, ".svg")),
                )
            };
        }
        match self {
            Icon::Alert => svg!("alert"),
            Icon::Check => svg!("check"),
            Icon::ChevronDown => svg!("chevron-down"),
            Icon::ChevronRight => svg!("chevron-right"),
            Icon::ChevronUp => svg!("chevron-up"),
            Icon::Clock => svg!("clock"),
            Icon::Download => svg!("download"),
            Icon::External => svg!("external"),
            Icon::Eye => svg!("eye"),
            Icon::Folder => svg!("folder"),
            Icon::Info => svg!("info"),
            Icon::Logo => svg!("logo"),
            Icon::Refresh => svg!("refresh"),
            Icon::Search => svg!("search"),
            Icon::Skip => svg!("skip"),
            Icon::Trash => svg!("trash"),
            Icon::Update => svg!("update"),
            Icon::X => svg!("x"),
        }
    }

    /// The icon drawn in `color` at `size` points. The SVGs stroke with currentColor, which
    /// the renderer draws black; drawing them white makes the tint the colour.
    pub fn image(self, size: f32, color: Color32) -> egui::Image<'static> {
        type Cache = Mutex<HashMap<&'static str, Arc<[u8]>>>;
        static WHITE: LazyLock<Cache> = LazyLock::new(Default::default);
        let (name, svg) = self.svg();
        let bytes = WHITE
            .lock()
            .unwrap()
            .entry(name)
            .or_insert_with(|| svg.replace("currentColor", "#FFFFFF").into_bytes().into())
            .clone();
        egui::Image::new(egui::ImageSource::Bytes {
            uri: format!("bytes://icon-{name}.svg").into(),
            bytes: Bytes::Shared(bytes),
        })
        .fit_to_exact_size(Vec2::splat(size))
        .tint(color)
    }

    /// Paints the icon into `rect`.
    pub fn paint(self, ui: &egui::Ui, rect: egui::Rect, color: Color32) {
        self.image(rect.width(), color).paint_at(ui, rect);
    }
}
