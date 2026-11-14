//! Embeds the wam binary for the target platform.
//!
//! `WAM_BIN=/path/to/wam` embeds that file (the Docker build passes the one it built).
//! Otherwise this runs `go build` on ../cmd/wam for the cargo target, so a plain
//! `cargo run` works wherever Go is installed.

use std::{env, fs, path::PathBuf, process::Command};

fn main() {
    let manifest = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let repo = manifest.parent().unwrap().to_path_buf();
    let out = PathBuf::from(env::var("OUT_DIR").unwrap());

    let version = fs::read_to_string(repo.join("VERSION"))
        .map(|v| v.trim().to_string())
        .unwrap_or_else(|_| "dev".into());
    println!("cargo:rustc-env=WAM_VERSION={version}");
    println!("cargo:rerun-if-changed=../VERSION");
    println!("cargo:rerun-if-env-changed=WAM_BIN");

    let bin = match env::var_os("WAM_BIN") {
        Some(path) => PathBuf::from(path),
        None => go_build(&repo, &out, &version),
    };
    println!("cargo:rerun-if-changed={}", bin.display());
    println!("cargo:rustc-env=WAM_EMBED={}", bin.display());

    embed_windows_icon();
}

/// The .exe's icon in Explorer and the taskbar. Cross-builds use MinGW's windres.
fn embed_windows_icon() {
    println!("cargo:rerun-if-changed=assets/icon.ico");
    if env::var("CARGO_CFG_TARGET_OS").as_deref() == Ok("windows") {
        winresource::WindowsResource::new()
            .set_icon("assets/icon.ico")
            .compile()
            .expect("embedding assets/icon.ico (needs windres, from MinGW or binutils)");
    }
}

fn go_build(repo: &std::path::Path, out: &std::path::Path, version: &str) -> PathBuf {
    for path in ["../cmd", "../internal", "../go.mod"] {
        println!("cargo:rerun-if-changed={path}");
    }
    let target_os = env::var("CARGO_CFG_TARGET_OS").unwrap();
    let goos = match target_os.as_str() {
        "macos" => "darwin",
        os => os,
    };
    let goarch = match env::var("CARGO_CFG_TARGET_ARCH").unwrap().as_str() {
        "x86_64" => "amd64",
        "aarch64" => "arm64",
        arch => panic!("no Go architecture known for {arch}; set WAM_BIN"),
    };
    let exe = out.join(if goos == "windows" { "wam.exe" } else { "wam" });
    let status = Command::new("go")
        .current_dir(repo)
        .env("GOOS", goos)
        .env("GOARCH", goarch)
        .env("CGO_ENABLED", "0")
        .args(["build", "-trimpath", "-ldflags"])
        .arg(format!("-s -w -X main.version={version}"))
        .arg("-o")
        .arg(&exe)
        .arg("./cmd/wam")
        .status()
        .expect("running go build (install Go, or set WAM_BIN to a wam binary)");
    assert!(status.success(), "go build failed");
    exe
}
