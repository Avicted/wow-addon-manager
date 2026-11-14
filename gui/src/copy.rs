//! Words shown to the player (docs/gui-design/copy.md) that depend on data.

use chrono::{DateTime, Local};

use crate::wam::ErrInfo;

/// "1 addon", "2 addons".
pub fn plural(n: usize, word: &str) -> String {
    if n == 1 {
        format!("1 {word}")
    } else {
        format!("{n} {word}s")
    }
}

/// Local "HH:MM".
pub fn clock(t: DateTime<Local>) -> String {
    t.format("%H:%M").to_string()
}

/// When GitHub checks resume: "at 15:00" in local time, or "later" if wam didn't say.
pub fn retry_time(err: &ErrInfo) -> String {
    err.retry_at
        .as_deref()
        .and_then(|t| DateTime::parse_from_rfc3339(t).ok())
        .map(|t| format!("at {}", clock(t.with_timezone(&Local))))
        .unwrap_or_else(|| "later".into())
}

/// 2.1M, 310K, 950.
pub fn downloads(n: u64) -> String {
    let short = |v: f64, unit: &str| {
        if v >= 10.0 {
            format!("{v:.0}{unit}")
        } else {
            format!("{v:.1}{unit}").replace(".0", "")
        }
    };
    match n {
        0..1_000 => n.to_string(),
        1_000..1_000_000 => short(n as f64 / 1e3, "K"),
        _ => short(n as f64 / 1e6, "M"),
    }
}

/// "CurseForge" or "GitHub" for a source or ref.
pub fn site(source_or_ref: &str) -> &'static str {
    if source_or_ref.starts_with("github") {
        "GitHub"
    } else {
        "CurseForge"
    }
}

/// What went wrong, in words a player understands. `source` is the addon's source ("curse",
/// "github") when the error belongs to one addon, so an HTTP error names the right site.
pub fn error(err: &ErrInfo, source: Option<&str>) -> String {
    match err.code.as_str() {
        "rate_limit" => format!(
            "GitHub limit reached. Checks resume {}, or add a token in Settings.",
            retry_time(err)
        ),
        "http" => {
            let site = match source {
                Some(s) => site(s),
                None if err.message.contains("github") => "GitHub",
                None => "CurseForge",
            };
            format!("{site} isn\u{2019}t responding (HTTP {})", err.status)
        }
        "network" => {
            "Can\u{2019}t reach CurseForge or GitHub. Check your internet connection.".into()
        }
        "in_use" => "A file is in use. Close the game and try again.".into(),
        "not_found" => "No version of this addon works with this game.".into(),
        "bad_ref" => "Couldn\u{2019}t find an addon at that address.".into(),
        "disk_full" => "Not enough disk space.".into(),
        "permission" => "wam isn\u{2019}t allowed to write to the AddOns folder.".into(),
        "locked" => "wam is already running in another window. Close it, then try again.".into(),
        "addons_dir" => "The AddOns folder wasn\u{2019}t found.".into(),
        _ => format!("Something went wrong: {}", err.message),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn err(code: &str) -> ErrInfo {
        ErrInfo {
            code: code.into(),
            message: "HTTP 503 from https://api.github.com/x".into(),
            status: 503,
            ..Default::default()
        }
    }

    #[test]
    fn download_counts_are_short() {
        let cases = [
            (950, "950"),
            (1_000, "1K"),
            (310_400, "310K"),
            (2_140_000, "2.1M"),
            (14_200_000, "14M"),
            (9_600_000, "9.6M"),
        ];
        for (n, want) in cases {
            assert_eq!(downloads(n), want, "{n}");
        }
    }

    #[test]
    fn http_errors_name_the_site() {
        assert_eq!(
            error(&err("http"), Some("curse")),
            "CurseForge isn\u{2019}t responding (HTTP 503)"
        );
        assert_eq!(
            error(&err("http"), None),
            "GitHub isn\u{2019}t responding (HTTP 503)"
        );
    }

    #[test]
    fn rate_limit_shows_a_time_and_unknown_codes_the_message() {
        let mut e = err("rate_limit");
        e.retry_at = Some("2026-11-14T15:00:00Z".into());
        assert!(error(&e, None).starts_with("GitHub limit reached. Checks resume at "));
        assert!(!error(&e, None).contains("later"));
        e.retry_at = None;
        assert_eq!(
            error(&e, None),
            "GitHub limit reached. Checks resume later, or add a token in Settings."
        );
        assert_eq!(
            error(&err("weird"), None),
            "Something went wrong: HTTP 503 from https://api.github.com/x"
        );
    }

    #[test]
    fn plurals() {
        assert_eq!(plural(1, "addon"), "1 addon");
        assert_eq!(plural(0, "addon"), "0 addons");
    }
}
