//! Runs the embedded `wam` binary with `-json` and turns its output into events.
//! The protocol is described in internal/cli/json.go.

use std::{
    fs,
    io::{self, BufRead, BufReader, Read, Write},
    path::{Path, PathBuf},
    process::{ChildStdin, Command, Stdio},
    sync::mpsc,
    thread,
};

use serde::Deserialize;

/// The wam binary for this platform, built by build.rs.
static EMBEDDED: &[u8] = include_bytes!(env!("WAM_EMBED"));
pub const VERSION: &str = env!("WAM_VERSION");

#[derive(Debug, Clone, Deserialize)]
#[serde(tag = "event", rename_all = "snake_case")]
pub enum Event {
    Log {
        level: String,
        message: String,
        #[serde(default)]
        code: Option<String>,
    },
    Checking {
        addons: Vec<Row>,
    },
    Checked {
        addon: Row,
        done: usize,
        total: usize,
    },
    Item {
        item: Item,
    },
    Result {
        ok: bool,
        #[serde(default)]
        data: Option<serde_json::Value>,
        #[serde(default)]
        error: Option<ErrInfo>,
    },
}

#[derive(Debug, Clone, Default, PartialEq, Deserialize)]
pub struct ErrInfo {
    pub message: String,
    pub code: String,
    #[serde(default)]
    pub status: u16,
    #[serde(default)]
    pub retry_at: Option<String>,
    #[serde(default)]
    pub path: Option<String>,
}

#[derive(Debug, Clone, Default, PartialEq, Deserialize)]
pub struct Row {
    pub name: String,
    pub installed: String,
    pub latest: String,
    pub source: String,
    #[serde(rename = "ref")]
    pub reference: String,
    pub status: String,
    pub adopted: bool,
    #[serde(default, deserialize_with = "null_as_empty")]
    pub folders: Vec<String>,
    #[serde(default)]
    pub error: Option<ErrInfo>,
}

#[derive(Debug, Clone, Default, PartialEq, Deserialize)]
pub struct Item {
    pub name: String,
    pub from: String,
    pub to: String,
    pub state: String,
    #[serde(default)]
    pub error: Option<ErrInfo>,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct StatusData {
    pub addons_dir: String,
    pub backups_dir: String,
    pub addons: Vec<Row>,
    pub untracked: Vec<Untracked>,
    pub broken: Vec<Broken>,
    pub updates: usize,
    pub wow_running: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Untracked {
    pub root: String,
    pub folders: Vec<String>,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Broken {
    pub folder: String,
    #[serde(default, rename = "ref")]
    pub reference: Option<String>,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct UpdateData {
    pub items: Vec<Item>,
    pub updated: usize,
    pub failed: usize,
    pub skipped: usize,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct InstallData {
    pub name: String,
    pub version: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct RemoveData {
    pub name: String,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct SearchData {
    pub query: String,
    pub hits: Vec<Hit>,
    pub wago_unavailable: bool,
}

#[derive(Debug, Clone, Default, Deserialize)]
pub struct Hit {
    pub title: String,
    #[serde(default, rename = "ref")]
    pub reference: Option<String>,
    #[serde(default)]
    pub downloads: Option<u64>,
    pub installed: bool,
    #[serde(default)]
    pub url: Option<String>,
}

/// Go writes an empty list as `null`.
fn null_as_empty<'de, D, T>(d: D) -> Result<Vec<T>, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Deserialize<'de>,
{
    Ok(Option::<Vec<T>>::deserialize(d)?.unwrap_or_default())
}

/// Parses one line of wam's output.
pub fn parse(line: &str) -> Result<Event, serde_json::Error> {
    serde_json::from_str(line)
}

/// `$XDG_DATA_HOME`, or `~/.local/share`.
pub fn data_home() -> PathBuf {
    std::env::var_os("XDG_DATA_HOME")
        .filter(|v| !v.is_empty())
        .map(PathBuf::from)
        .unwrap_or_else(|| {
            std::env::home_dir()
                .unwrap_or_default()
                .join(".local")
                .join("share")
        })
}

/// Where wam keeps its state and backups: the same rule as the CLI's.
pub fn data_dir() -> PathBuf {
    let var = |k| {
        std::env::var_os(k)
            .filter(|v| !v.is_empty())
            .map(PathBuf::from)
    };
    let home = || std::env::home_dir().unwrap_or_default();
    if let Some(d) = var("WAM_DATA_DIR") {
        return d;
    }
    if cfg!(windows) {
        var("LOCALAPPDATA")
            .unwrap_or_else(|| home().join("AppData").join("Local"))
            .join("wam")
    } else {
        data_home().join("wam")
    }
}

/// The window icon, also installed as the launcher icon.
pub static ICON_PNG: &[u8] = include_bytes!("../assets/icon.png");

/// Gives the app a launcher entry and icon under `data_home` (`$XDG_DATA_HOME`).
///
/// Wayland compositors take the window and taskbar icon from the `.desktop` file that matches the
/// app id (`wam`), not from the window itself. Files are only written when missing or different,
/// so this is cheap at every start. Returns the paths it wrote.
pub fn install_desktop_entry(data_home: &Path, exe: &Path) -> io::Result<Vec<PathBuf>> {
    let exec = exe
        .to_string_lossy()
        .replace('\\', "\\\\")
        .replace('"', "\\\"");
    let entry = format!(
        "[Desktop Entry]\nType=Application\nName=wam\nComment=Keeps your World of Warcraft addons up to date\n\
         Exec=\"{exec}\"\nIcon=wam\nCategories=Game;Utility;\nStartupWMClass=wam\nTerminal=false\n"
    );
    let files = [
        (
            data_home.join("applications/wam.desktop"),
            entry.into_bytes(),
        ),
        (
            data_home.join("icons/hicolor/256x256/apps/wam.png"),
            ICON_PNG.to_vec(),
        ),
    ];
    let mut written = Vec::new();
    for (path, content) in files {
        if fs::read(&path).ok().as_deref() == Some(content.as_slice()) {
            continue;
        }
        if let Some(dir) = path.parent() {
            fs::create_dir_all(dir)?;
        }
        fs::write(&path, content)?;
        written.push(path);
    }
    Ok(written)
}

/// Writes the embedded wam to `<data dir>/bin/wam-<version>` unless it is already there,
/// and returns its path.
pub fn unpack(data_dir: &Path) -> io::Result<PathBuf> {
    let dir = data_dir.join("bin");
    fs::create_dir_all(&dir)?;
    let path = dir.join(format!("wam-{VERSION}{}", std::env::consts::EXE_SUFFIX));
    if fs::read(&path).ok().as_deref() == Some(EMBEDDED) {
        return Ok(path);
    }
    let tmp = path.with_extension("tmp");
    fs::write(&tmp, EMBEDDED)?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        fs::set_permissions(&tmp, fs::Permissions::from_mode(0o755))?;
    }
    fs::rename(&tmp, &path)?;
    Ok(path)
}

/// What a running wam sends back.
pub enum Msg {
    Event(Box<Event>),
    /// The process ended. `stderr` explains a crash that produced no result.
    Exited {
        stderr: String,
    },
}

/// One running wam command.
pub struct Job {
    stdin: Option<ChildStdin>,
    pub rx: mpsc::Receiver<Msg>,
}

impl Job {
    /// A job that never sends anything, for tests.
    #[cfg(test)]
    pub fn idle() -> Job {
        let (_tx, rx) = mpsc::channel();
        Job { stdin: None, rx }
    }

    /// Asks wam to finish the addon it is installing and skip the rest.
    pub fn cancel(&mut self) {
        if let Some(stdin) = &mut self.stdin {
            let _ = stdin.write_all(b"cancel\n").and_then(|()| stdin.flush());
        }
    }
}

/// Starts `wam -json [-addons-dir DIR] ARGS...`. `wake` is called after each message so the
/// UI repaints.
pub fn spawn(
    bin: &Path,
    addons_dir: Option<&str>,
    github_token: &str,
    args: &[String],
    wake: impl Fn() + Send + 'static,
) -> io::Result<Job> {
    let mut cmd = Command::new(bin);
    cmd.arg("-json");
    if let Some(dir) = addons_dir {
        cmd.arg("-addons-dir").arg(dir);
    }
    cmd.args(args)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped());
    if !github_token.is_empty() {
        cmd.env("GITHUB_TOKEN", github_token);
    }
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        const CREATE_NO_WINDOW: u32 = 0x0800_0000;
        cmd.creation_flags(CREATE_NO_WINDOW);
    }
    let mut child = cmd.spawn()?;
    let stdout = child.stdout.take().expect("piped stdout");
    let mut stderr = child.stderr.take().expect("piped stderr");
    let stdin = child.stdin.take();
    let (tx, rx) = mpsc::channel();
    thread::spawn(move || {
        let errors = thread::spawn(move || {
            let mut s = String::new();
            let _ = stderr.read_to_string(&mut s);
            s
        });
        for line in BufReader::new(stdout).lines() {
            let Ok(line) = line else { break };
            if let Ok(ev) = parse(&line) {
                if tx.send(Msg::Event(Box::new(ev))).is_err() {
                    break;
                }
                wake();
            }
        }
        let _ = child.wait();
        let stderr = errors.join().unwrap_or_default();
        let _ = tx.send(Msg::Exited { stderr });
        wake();
    });
    Ok(Job { stdin, rx })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_every_event_kind() {
        let lines = [
            r#"{"event":"log","level":"warning","message":"WoW is running","code":"wow_running"}"#,
            r#"{"event":"checking","addons":[{"name":"Questie","installed":"10","latest":"","source":"curse","ref":"curse:1","status":"checking","adopted":false,"folders":["Questie"]}]}"#,
            r#"{"event":"checked","addon":{"name":"Questie","installed":"10","latest":"11","source":"curse","ref":"curse:1","status":"update","adopted":false,"folders":null},"done":1,"total":1}"#,
            r#"{"event":"item","item":{"name":"Questie","from":"10","to":"11","state":"failed","error":{"message":"boom","code":"in_use"}}}"#,
            r#"{"event":"result","ok":false,"error":{"message":"rate","code":"rate_limit","status":403,"retry_at":"2026-11-14T15:00:00Z"}}"#,
        ];
        let evs: Vec<Event> = lines.iter().map(|l| parse(l).unwrap()).collect();
        assert!(matches!(&evs[0], Event::Log { code: Some(c), .. } if c == "wow_running"));
        assert!(matches!(&evs[1], Event::Checking { addons } if addons[0].folders == ["Questie"]));
        assert!(
            matches!(&evs[2], Event::Checked { addon, done: 1, total: 1 } if addon.status == "update")
        );
        assert!(
            matches!(&evs[3], Event::Item { item } if item.error.as_ref().unwrap().code == "in_use")
        );
        assert!(
            matches!(&evs[4], Event::Result { ok: false, error: Some(e), .. } if e.status == 403 && e.retry_at.is_some())
        );
    }

    #[test]
    fn parses_search_data_with_missing_counts() {
        let v = serde_json::json!({"query":"map","wago_unavailable":false,"hits":[
            {"title":"Details","ref":"curse:1","downloads":null,"installed":true},
            {"title":"basicminimap","downloads":66660,"installed":false,"url":"https://addons.wago.io/addons/basicminimap"}]});
        let d: SearchData = serde_json::from_value(v).unwrap();
        assert_eq!(d.hits[0].downloads, None);
        assert_eq!(d.hits[1].reference, None);
        assert_eq!(d.hits[1].downloads, Some(66660));
    }

    #[test]
    fn desktop_entry_is_written_once_and_rewritten_when_the_exe_moves() {
        let dir = std::env::temp_dir().join(format!("wam-desktop-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        let exe = Path::new("/opt/my apps/wam-gui");
        assert_eq!(install_desktop_entry(&dir, exe).unwrap().len(), 2);
        let entry = fs::read_to_string(dir.join("applications/wam.desktop")).unwrap();
        assert!(entry.contains("Exec=\"/opt/my apps/wam-gui\"\n"));
        assert!(entry.contains("Icon=wam\n") && entry.contains("StartupWMClass=wam\n"));
        assert_eq!(
            fs::read(dir.join("icons/hicolor/256x256/apps/wam.png")).unwrap(),
            ICON_PNG
        );
        assert!(install_desktop_entry(&dir, exe).unwrap().is_empty());
        let moved = install_desktop_entry(&dir, Path::new("/usr/bin/wam-gui")).unwrap();
        assert_eq!(moved, vec![dir.join("applications/wam.desktop")]);
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn data_dir_honours_wam_data_dir() {
        // SAFETY: tests in this module don't read the environment concurrently.
        unsafe { std::env::set_var("WAM_DATA_DIR", "/x/wam") };
        assert_eq!(data_dir(), PathBuf::from("/x/wam"));
        unsafe { std::env::remove_var("WAM_DATA_DIR") };
    }
}
