//! The real UI driven headlessly. Interaction tests click through accessibility labels and
//! need no GPU. `screenshots` renders every state from the design's mockup list to
//! target/ui-shots/ for comparing with docs/gui-design/; it needs a GPU, so it is ignored by
//! default: `cargo test screenshots -- --ignored`.

use eframe::egui::{self, Vec2, accesskit::Role};
use egui_kittest::{Harness, kittest::Queryable};
use serde_json::{Value, json};

use crate::{
    app::{App, Cmd, Dialog, FindPhase, Settings, Tab},
    theme,
    wam::{Item, SearchData},
};

fn harness(app: App, size: Vec2) -> Harness<'static, App> {
    let mut h = Harness::builder()
        .with_size(size)
        .with_pixels_per_point(1.0)
        .with_context_setup(|ctx| {
            theme::apply(ctx);
            egui_extras::install_image_loaders(ctx);
        })
        .build_ui_state(|ui, app: &mut App| crate::draw(app, ui), app);
    let ctx = h.ctx.clone();
    h.state_mut().ctx = ctx;
    h.run_steps(3);
    h
}

fn new_app() -> App {
    let mut a = App::new(
        egui::Context::default(),
        Settings::default(),
        Err("no wam in tests".into()),
    );
    a.outcome = (String::new(), crate::app::Tone::Muted);
    a
}

/// One status row: name, installed, latest, source ref, status, adopted, error code.
fn row(
    name: &str,
    installed: &str,
    latest: &str,
    reference: &str,
    status: &str,
    adopted: bool,
    err: Option<(&str, &str)>,
) -> Value {
    let source = reference.split(':').next().unwrap();
    let mut r = json!({"name": name, "installed": installed, "latest": latest, "source": source, "ref": reference,
        "status": status, "adopted": adopted, "folders": [name.replace([' ', '!', '.'], "")]});
    if let Some((code, message)) = err {
        r["error"] = json!({"code": code, "message": message, "status": 503});
    }
    r
}

/// The design's sample data: 14 addons, 2 updates, one failed check.
fn sample_rows() -> Vec<Value> {
    let ok = |n: &str, v: &str, r: &str| row(n, v, v, r, "ok", false, None);
    vec![
        ok("AtlasLootClassic", "2.4.1", "curse:atlaslootclassic"),
        row(
            "Auctionator",
            "341",
            "346",
            "curse:6124",
            "update",
            true,
            None,
        ),
        ok("Bagnon", "10.2.7", "curse:bagnon"),
        ok("Bartender4", "4.14.3", "curse:bartender4"),
        ok("ClassicCastbars", "1.9.2", "github:wardz/ClassicCastbars"),
        ok("Deadly Boss Mods", "1.15.31", "curse:deadly-boss-mods"),
        row(
            "Details! Damage Meter",
            "13.0.2",
            "",
            "curse:61284",
            "error",
            false,
            Some((
                "http",
                "HTTP 503 from https://www.curseforge.com/api/v1/mods/61284/files",
            )),
        ),
        row(
            "Leatrix Maps",
            "1.60.28",
            "1.60.31-forever",
            "curse:298842",
            "update",
            true,
            None,
        ),
        ok("Leatrix Plus", "1.60.40", "curse:leatrix-plus"),
        ok("MoveAny", "1.8.4", "curse:moveany"),
        ok("OmniCC", "10.2.4", "github:tullamods/OmniCC"),
        ok("Plater Nameplates", "612", "curse:plater-nameplates"),
        ok("Prat 3.0", "3.9.48", "curse:prat-3-0"),
        ok("Questie", "v12.9.0", "github:Questie/Questie"),
    ]
}

fn status(rows: Vec<Value>) -> Value {
    let updates = rows.iter().filter(|r| r["status"] == "update").count();
    json!({"addons_dir": r"C:\Program Files (x86)\World of Warcraft\_forever_\Interface\AddOns",
        "backups_dir": r"C:\Users\you\AppData\Local\wam\backups",
        "addons": rows, "updates": updates, "wow_running": false,
        "untracked": [{"root": "Journalator", "folders": ["Journalator", "Journalator_Config", "Journalator_Data", "Journalator_Options"]},
                      {"root": "ForeverArtisan", "folders": ["ForeverArtisan_Alchemy"]}],
        "broken": [{"folder": "WeakAuras", "curse_id": "65387", "ref": "curse:65387"}, {"folder": "OldMap"}]})
}

fn populated() -> App {
    let mut a = new_app();
    a.pretend_finished(Cmd::Status { rescan: false }, status(sample_rows()));
    a
}

fn item(name: &str, from: &str, to: &str, state: &str) -> Item {
    Item {
        name: name.into(),
        from: from.into(),
        to: to.into(),
        state: state.into(),
        error: None,
    }
}

/// The design's busy state: 4 of 6 updating, one failed, the game running.
fn busy(cancelling: bool) -> App {
    let mut a = populated();
    let rest = if cancelling { "skipped" } else { "waiting" };
    let mut failed = item("Leatrix Maps", "1.60.28", "1.60.31-forever", "failed");
    failed.error = Some(crate::wam::ErrInfo {
        code: "in_use".into(),
        ..Default::default()
    });
    a.items = vec![
        item("Auctionator", "341", "346", "updated"),
        item("Bagnon", "10.2.6", "10.2.7", "updated"),
        failed,
        item("Plater Nameplates", "598", "612", "updating"),
        item(
            "Prat 3.0",
            "3.9.47",
            "3.9.48",
            if cancelling { "waiting" } else { rest },
        ),
        item(
            "Questie",
            "v12.8.2",
            "v12.9.0",
            if cancelling { "waiting" } else { rest },
        ),
    ];
    a.wow_running = true;
    a.pretend_running(Cmd::Update { names: vec![] }, cancelling);
    a
}

#[test]
fn remove_opens_a_dialog_that_names_the_folders_and_escape_closes_it() {
    let mut h = harness(populated(), Vec2::new(960.0, 640.0));
    h.get_by_label("Remove Auctionator").click();
    h.run_steps(2);
    assert!(h.query_by_label("Remove Auctionator?").is_some());
    // In the table and in the dialog's folder list.
    assert_eq!(h.query_all_by_label("Auctionator").count(), 2);
    h.key_press(egui::Key::Escape);
    h.run_steps(2);
    assert!(h.state().dialog.is_none());
}

#[test]
fn not_tracked_expands_and_track_asks_where_the_folders_come_from() {
    let mut h = harness(populated(), Vec2::new(960.0, 640.0));
    h.get_by_role_and_label(Role::Button, "Not tracked").click();
    h.run_steps(2);
    assert_eq!(h.state().open_section, Some(crate::app::Section::Untracked));
    h.get_all_by_label("Track\u{2026}").next().unwrap().click();
    h.run_steps(2);
    assert!(matches!(&h.state().dialog, Some(Dialog::Track { folders, .. }) if folders.len() == 4));
    assert!(
        h.query_by_label("Where does Journalator come from?")
            .is_some()
    );
}

#[test]
fn update_all_counts_updates_and_a_running_update_offers_cancel() {
    let h = harness(populated(), Vec2::new(960.0, 640.0));
    assert!(h.query_by_label("Update all (2)").is_some());
    let h = harness(busy(false), Vec2::new(960.0, 640.0));
    assert!(h.query_by_label("Update all (2)").is_none());
    assert!(h.query_by_label("Cancel").is_some());
    assert!(h.query_by_label("Updating 4 of 6").is_some());
}

#[test]
fn tabs_switch_screens() {
    let mut h = harness(populated(), Vec2::new(960.0, 640.0));
    h.get_by_label("Find addons").click();
    h.run_steps(2);
    assert_eq!(h.state().tab, Tab::Find);
    assert!(h.query_by_label("Find an addon").is_some());
    h.get_by_label("Settings").click();
    h.run_steps(2);
    assert!(h.query_by_label("Rescan unidentified folders").is_some());
}

#[test]
#[ignore = "renders with the GPU; run with --ignored to write target/ui-shots"]
fn screenshots() {
    let dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("target/ui-shots");
    std::fs::create_dir_all(&dir).unwrap();
    let shot =
        |name: &str, app: App, size: (f32, f32), prepare: &dyn Fn(&mut Harness<'static, App>)| {
            let mut h = harness(app, Vec2::new(size.0, size.1));
            prepare(&mut h);
            h.run_steps(3);
            h.render()
                .unwrap()
                .save(dir.join(format!("{name}.png")))
                .unwrap();
        };
    let none = &|_: &mut Harness<'static, App>| {};
    let base = (960.0, 640.0);

    shot("addons", populated(), base, none);
    shot("addons-720", populated(), (720.0, 480.0), none);
    shot("addons-1600", populated(), (1600.0, 900.0), none);
    shot("addons-busy", busy(false), base, &|h| {
        h.state_mut().log_open = true
    });
    shot("addons-cancelling", busy(true), base, none);
    shot("addons-broken-open", populated(), base, &|h| {
        h.state_mut().open_section = Some(crate::app::Section::Broken)
    });

    let mut loading = populated();
    for r in loading.rows.iter_mut().skip(6) {
        r.status = "checking".into();
    }
    loading.pretend_running(Cmd::Status { rescan: false }, false);
    loading.check = Some((6, 14));
    shot("addons-loading", loading, base, none);

    let mut down = new_app();
    let rows = sample_rows()
        .into_iter()
        .map(|mut r| {
            r["status"] = "error".into();
            r["error"] = json!({"code": "network", "message": "dial tcp: no route to host"});
            r
        })
        .collect();
    down.pretend_finished(Cmd::Status { rescan: false }, status(rows));
    shot("addons-error", down, base, none);

    let mut long = new_app();
    let names = [
        "AtlasLootClassic",
        "Auctionator",
        "Bagnon",
        "Bartender4",
        "Deadly Boss Mods",
        "Questie",
    ];
    let extra = [
        "",
        " Classic Era Port",
        " \u{2014} Extended Tooltips and Vendor Price Overlay",
        " (Forever Edition, community-maintained fork)",
    ];
    let versions = [
        "1.60.31-forever",
        "v12.9.0-beta3-142-gabc1234",
        "346",
        "2026.11.03-release-candidate-2",
    ];
    let rows = (0..60)
        .map(|i| {
            let name = format!("{}{}", names[i % names.len()], extra[i / 15]);
            let v = versions[i % versions.len()];
            match i % 7 {
                3 => row(
                    &name,
                    v,
                    versions[(i + 1) % 4],
                    "curse:1",
                    "update",
                    i % 5 == 1,
                    None,
                ),
                6 => row(
                    &name,
                    v,
                    "",
                    "github:a/b",
                    "error",
                    false,
                    Some(("rate_limit", "rate")),
                ),
                _ => row(&name, v, v, "curse:1", "ok", i % 5 == 1, None),
            }
        })
        .collect();
    long.pretend_finished(Cmd::Status { rescan: false }, status(rows));
    shot("addons-long", long, base, none);

    let mut empty = new_app();
    empty.pretend_finished(
        Cmd::Status { rescan: false },
        json!({"addons_dir": "/x", "backups_dir": "/b", "addons": [],
        "untracked": [], "broken": [], "updates": 0, "wow_running": false}),
    );
    shot("addons-empty", empty, base, none);

    let mut first = new_app();
    first.first_run =
        Some(r"C:\Program Files (x86)\World of Warcraft\_forever_\Interface\AddOns".into());
    shot("first-run", first, base, none);

    shot("dialog-remove", populated(), base, &|h| {
        h.state_mut().dialog = Some(Dialog::Remove {
            name: "Auctionator".into(),
            folders: vec!["Auctionator".into()],
        });
    });
    shot("dialog-track-error", populated(), base, &|h| {
        h.state_mut().dialog = Some(Dialog::Track {
            name: "Journalator".into(),
            folders: vec!["Journalator".into(), "Journalator_Config".into()],
            input: "journalatorr".into(),
            error: Some("Couldn\u{2019}t find an addon at that address.".into()),
            focus: false,
        });
    });

    let find = |phase: FindPhase| {
        let mut a = populated();
        a.tab = Tab::Find;
        a.find.query = "map".into();
        a.find.phase = phase;
        a
    };
    let hits: SearchData = serde_json::from_value(json!({"query": "map", "wago_unavailable": false, "hits": [
        {"title": "Leatrix Maps", "ref": "curse:298842", "downloads": 2100000, "installed": true},
        {"title": "Mapster", "ref": "curse:14376", "downloads": 4800000, "installed": false},
        {"title": "Magnify", "ref": "curse:1", "downloads": 310000, "installed": false},
        {"title": "HandyNotes", "ref": "curse:2", "downloads": 9600000, "installed": false},
        {"title": "Map Coordinates", "downloads": 52000, "installed": false, "url": "https://addons.wago.io/addons/mapcoords"},
        {"title": "WorldMapTrackingEnhanced", "ref": "curse:3", "downloads": null, "installed": false}]}))
    .unwrap();
    shot("find-results", find(FindPhase::Results(hits)), base, none);
    let mut idle = find(FindPhase::Idle);
    idle.find.query.clear();
    shot("find-idle", idle, base, none);
    shot(
        "find-searching",
        find(FindPhase::Searching("map".into())),
        base,
        none,
    );
    shot(
        "find-none",
        find(FindPhase::Nothing("journalator classic forever".into())),
        base,
        none,
    );
    shot("find-down", find(FindPhase::Down), base, none);

    let mut settings = populated();
    settings.tab = Tab::Settings;
    shot("settings", settings, base, none);
}

/// Steps the UI until no wam command runs, or panics after `secs`.
fn settle(h: &mut Harness<'static, App>, secs: u64) {
    let end = std::time::Instant::now() + std::time::Duration::from_secs(secs);
    h.run_steps(2);
    while h.state().busy() {
        assert!(
            std::time::Instant::now() < end,
            "wam still running after {secs}s"
        );
        std::thread::sleep(std::time::Duration::from_millis(50));
        h.run_steps(1);
    }
    h.run_steps(2);
}

/// The GUI with the real embedded wam against a copy of an AddOns folder: WAM_E2E_DIR holds
/// Interface/AddOns and data/. It changes that copy and reaches CurseForge.
#[test]
#[ignore = "needs WAM_E2E_DIR and the network"]
fn real_wam() {
    let root = std::path::PathBuf::from(std::env::var("WAM_E2E_DIR").expect("WAM_E2E_DIR"));
    let data = root.join("data");
    // SAFETY: the only test that sets this, and wam reads it in its own process.
    unsafe { std::env::set_var("WAM_DATA_DIR", &data) };
    let bin = crate::wam::unpack(&data).map_err(|e| e.to_string());
    let settings = Settings {
        addons_dir: Some(root.join("Interface/AddOns").display().to_string()),
        ..Default::default()
    };
    let app = App::new(egui::Context::default(), settings, bin);
    let mut h = harness(app, Vec2::new(960.0, 640.0));
    settle(&mut h, 120);
    let rows = h.state().rows.len();
    assert!(rows > 0, "no addons: {:?}", h.state().outcome);
    eprintln!(
        "status: {rows} addons, {} updates; {}",
        h.state().updates(),
        h.state().outcome.0
    );

    if let Some(name) = h
        .state()
        .rows
        .iter()
        .find(|r| r.status == "update")
        .map(|r| r.name.clone())
    {
        h.state_mut().run(Cmd::Update {
            names: vec![name.clone()],
        });
        settle(&mut h, 300);
        eprintln!("update {name}: {}", h.state().outcome.0);
        assert!(
            h.state().outcome.0.starts_with("Updated 1 addon"),
            "{:?}",
            h.state().outcome
        );
        assert_eq!(
            h.state()
                .rows
                .iter()
                .find(|r| r.name == name)
                .unwrap()
                .status,
            "ok"
        );
    }

    h.state_mut().run(Cmd::Search {
        query: "map".into(),
    });
    settle(&mut h, 120);
    match &h.state().find.phase {
        FindPhase::Results(d) => eprintln!("search map: {} hits", d.hits.len()),
        _ => panic!("search found nothing"),
    }

    h.state_mut().run(Cmd::Install {
        reference: "zzzz-no-such-addon-zzzz".into(),
        from: crate::app::Origin::Link,
    });
    settle(&mut h, 120);
    let note = h.state().find.link_note.clone();
    eprintln!("bad install: {note:?}");
    assert!(
        matches!(note, Some((m, crate::app::Tone::Error)) if m.starts_with("Couldn\u{2019}t find"))
    );

    // Cancel: three addons look outdated; cancel once the first one is being installed.
    let state = data.join("state.json");
    let mut st: Value = serde_json::from_str(&std::fs::read_to_string(&state).unwrap()).unwrap();
    for a in st["addons"].as_array_mut().unwrap().iter_mut().take(3) {
        a["release"] = "1".into();
    }
    std::fs::write(&state, serde_json::to_string_pretty(&st).unwrap()).unwrap();
    h.state_mut().run(Cmd::Status { rescan: false });
    settle(&mut h, 120);
    assert!(h.state().updates() >= 3, "{} updates", h.state().updates());
    h.state_mut().run(Cmd::Update { names: vec![] });
    let end = std::time::Instant::now() + std::time::Duration::from_secs(120);
    while !h.state().items.iter().any(|i| i.state == "updating") {
        assert!(std::time::Instant::now() < end, "no addon started updating");
        std::thread::sleep(std::time::Duration::from_millis(10));
        h.run_steps(1);
    }
    h.state_mut().cancel();
    settle(&mut h, 300);
    eprintln!("cancel: {}", h.state().outcome.0);
    assert!(
        h.state().outcome.0.starts_with("Stopped: 1 updated, "),
        "{:?}",
        h.state().outcome
    );
}
