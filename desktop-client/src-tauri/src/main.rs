#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use base64::{
    engine::general_purpose::{URL_SAFE, URL_SAFE_NO_PAD},
    Engine as _,
};
use serde::Serialize;
use serde_json::Value;
#[cfg(windows)]
use std::os::windows::process::CommandExt;
#[cfg(windows)]
use std::sync::OnceLock;
use std::time::{SystemTime, UNIX_EPOCH};
use std::{
    env, fs,
    io::ErrorKind,
    path::{Path, PathBuf},
    process::{Command, Stdio},
};
use sysinfo::System;
use toml_edit::{DocumentMut, Item, Table};

const DEFAULT_BACKEND_API_BASE: &str = "http://localhost:8787/api";
const CODEXPPP_PROVIDER_ID: &str = "codexppp";
const CODEXPPP_PROVIDER_TOKEN_FILE: &str = "codex-provider-token";
const CODEXPPP_PROVIDER_TOKEN_SCRIPT_FILE: &str = "codex-provider-token.ps1";
const WINDOWS_STORE_CODEX_APP_ID: &str = "OpenAI.Codex_2p2nqsd0c76g0!App";
const WINDOWS_STORE_CODEX_PRODUCT_ID: &str = "9PLM9XGG6VKS";
#[cfg(windows)]
const CREATE_NO_WINDOW: u32 = 0x08000000;

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Diagnostics {
    codex_detected: String,
    codex_running: String,
    codex_account: String,
    codex_auth_mode: String,
    config_written: String,
    last_failure: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct PrepareResult {
    codex_detected: String,
    codex_running: String,
    config_written: String,
    last_failure: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct LaunchResult {
    launch_state: String,
    last_failure: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct DeviceIdentity {
    device_name: String,
    fingerprint: String,
}

enum CodexLaunchTarget {
    Command(PathBuf),
    AppUserModelId(String),
}

#[tauri::command]
fn codex_diagnostics() -> Diagnostics {
    let detected = find_codex_launch_target().is_some();
    let running = codex_process_running();
    Diagnostics {
        codex_detected: cn_available(detected),
        codex_running: cn_available(running),
        codex_account: codex_account_from_auth().unwrap_or_default(),
        codex_auth_mode: codex_auth_mode(),
        config_written: cn_available(codexppp_config_clean()),
        last_failure: String::new(),
    }
}

#[tauri::command]
fn device_identity() -> DeviceIdentity {
    build_device_identity()
}

#[tauri::command]
fn backend_api_base() -> String {
    configured_backend_api_base()
}

#[tauri::command]
fn desktop_version() -> String {
    env!("CARGO_PKG_VERSION").to_string()
}

#[tauri::command]
fn open_update_download(download_url: String) -> Result<(), String> {
    let mut cmd = build_update_download_command(&download_url)?;
    cmd.spawn()
        .map_err(|_| "update_download_failed".to_string())?;
    Ok(())
}

#[tauri::command]
fn prepare_codex(backend_url: String, provider_token: String) -> Result<PrepareResult, String> {
    let target = find_codex_launch_target().ok_or_else(|| "codex_not_detected".to_string())?;
    let running = codex_process_running();
    prepare_codex_for_target(&target, &backend_url, provider_token.trim())
        .map_err(|err| public_desktop_error(&err.to_string()))?;
    Ok(PrepareResult {
        codex_detected: cn_available(true),
        codex_running: cn_available(running),
        config_written: cn_available(true),
        last_failure: String::new(),
    })
}

#[tauri::command]
fn launch_codex() -> Result<LaunchResult, String> {
    if codex_process_running() {
        return Ok(LaunchResult {
            launch_state: "运行中".to_string(),
            last_failure: String::new(),
        });
    }
    let target = find_codex_launch_target().ok_or_else(|| "codex_not_detected".to_string())?;
    let mut cmd = build_codex_launch_command(&target)?;
    apply_managed_codex_environment(&mut cmd, &target)?;
    cmd.spawn()
        .map_err(|_| launch_failure_code(&target).to_string())?;
    Ok(LaunchResult {
        launch_state: "已启动".to_string(),
        last_failure: String::new(),
    })
}

#[tauri::command]
fn stop_codex() -> Result<Diagnostics, String> {
    stop_codex_processes().map_err(|_| "codex_stop_failed".to_string())?;
    Ok(codex_diagnostics())
}

#[tauri::command]
fn install_codex() -> Result<Diagnostics, String> {
    run_codex_install_command()?;
    Ok(codex_diagnostics())
}

fn public_desktop_error(raw: &str) -> String {
    if raw.contains("解析失败") || raw.contains("parse") {
        return "codex_config_unreadable".to_string();
    }
    "codex_config_write_failed".to_string()
}

fn launch_failure_code(target: &CodexLaunchTarget) -> &'static str {
    match target {
        CodexLaunchTarget::AppUserModelId(_) => "codex_appid_launch_failed",
        CodexLaunchTarget::Command(_) => "codex_command_launch_failed",
    }
}

fn build_codex_launch_command(target: &CodexLaunchTarget) -> Result<Command, String> {
    let mut cmd = match target {
        CodexLaunchTarget::AppUserModelId(app_id) => {
            let mut shell = Command::new("explorer");
            shell.arg(format!(r"shell:AppsFolder\{app_id}"));
            shell
        }
        CodexLaunchTarget::Command(command) if cfg!(windows) => {
            build_windows_launch_command(command)
        }
        CodexLaunchTarget::Command(command) => Command::new(command),
    };
    cmd.stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    Ok(cmd)
}

fn build_windows_launch_command(command: &Path) -> Command {
    let extension = command
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or_default()
        .to_ascii_lowercase();
    if matches!(extension.as_str(), "cmd" | "bat") {
        let mut shell = Command::new("cmd");
        shell.arg("/C").arg(windows_start_command(command));
        suppress_command_window(&mut shell);
        return shell;
    }
    let mut process = Command::new(command);
    suppress_command_window(&mut process);
    process
}

fn windows_start_command(command: &Path) -> String {
    let command_value = quote_windows_cmd_value(&command.to_string_lossy());
    let working_dir = command
        .parent()
        .map(|path| quote_windows_cmd_value(&path.to_string_lossy()))
        .unwrap_or_else(|| quote_windows_cmd_value("."));
    let extension = command
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or_default()
        .to_ascii_lowercase();
    if matches!(extension.as_str(), "cmd" | "bat") {
        return format!("start \"Codex\" /D {working_dir} cmd /K \"{command_value}\"");
    }
    format!("start \"Codex\" /D {working_dir} {command_value}")
}

fn quote_windows_cmd_value(value: &str) -> String {
    format!("\"{}\"", value.replace('"', "\"\""))
}

fn build_update_download_command(download_url: &str) -> Result<Command, String> {
    let url = validate_update_download_url(download_url)?;
    if !cfg!(windows) {
        return Err("update_download_unavailable".to_string());
    }
    let mut cmd = Command::new("cmd");
    cmd.arg("/C")
        .arg(format!("start \"\" {}", quote_windows_cmd_value(&url)))
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    Ok(cmd)
}

fn build_codex_install_command() -> Result<Command, String> {
    if !cfg!(windows) {
        return Err("codex_install_unavailable".to_string());
    }
    let winget =
        winget_command_path().ok_or_else(|| "codex_install_component_missing".to_string())?;
    Ok(build_codex_install_command_with_winget(winget))
}

fn build_codex_install_command_with_winget(winget: PathBuf) -> Command {
    let mut cmd = Command::new(winget);
    cmd.args([
        "install",
        "--id",
        WINDOWS_STORE_CODEX_PRODUCT_ID,
        "--source",
        "msstore",
        "--exact",
        "--silent",
        "--disable-interactivity",
        "--accept-package-agreements",
        "--accept-source-agreements",
    ])
    .stdin(Stdio::null())
    .stdout(Stdio::null())
    .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    cmd
}

fn run_codex_install_command() -> Result<(), String> {
    let mut cmd = build_codex_install_command()?;
    let status = cmd
        .status()
        .map_err(|_| "codex_install_failed".to_string())?;
    if status.success() || find_codex_launch_target().is_some() {
        return Ok(());
    }
    Err("codex_install_failed".to_string())
}

fn winget_command_path() -> Option<PathBuf> {
    find_command_on_path("winget").or_else(windows_winget_app_execution_alias)
}

fn windows_winget_app_execution_alias() -> Option<PathBuf> {
    if !cfg!(windows) {
        return None;
    }
    env::var_os("LOCALAPPDATA")
        .map(PathBuf::from)
        .map(|path| {
            path.join("Microsoft")
                .join("WindowsApps")
                .join("winget.exe")
        })
        .filter(|path| path.is_file())
}

fn validate_update_download_url(raw: &str) -> Result<String, String> {
    let value = raw.trim();
    let lower = value.to_ascii_lowercase();
    if value.is_empty()
        || !(lower.starts_with("https://") || lower.starts_with("http://"))
        || value.chars().any(|ch| ch.is_control() || ch == '"')
    {
        return Err("update_download_unavailable".to_string());
    }
    Ok(value.to_string())
}

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            codex_diagnostics,
            device_identity,
            backend_api_base,
            desktop_version,
            open_update_download,
            install_codex,
            prepare_codex,
            launch_codex,
            stop_codex
        ])
        .run(tauri::generate_context!())
        .expect("failed to run Codex+++ desktop client");
}

fn prepare_codex_for_target(
    _target: &CodexLaunchTarget,
    backend_url: &str,
    provider_token: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    remove_data_home_legacy_artifacts()?;
    remove_managed_codex_home()?;
    let home = codex_home()?;
    fs::create_dir_all(&home)?;
    if codex_chatgpt_auth_present_at(&home) {
        remove_codexppp_config(&home)?;
        remove_codexppp_api_auth(&home, true)?;
    } else {
        if provider_token.trim().is_empty() {
            return Err("login_failed".into());
        }
        write_codexppp_api_login(&home, backend_url, provider_token)?;
    }
    Ok(())
}

fn write_codexppp_api_login(
    home: &Path,
    backend_url: &str,
    api_key: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    write_codexppp_api_config(home, backend_url)?;
    write_codexppp_api_auth(home, api_key)?;
    Ok(())
}

fn write_codexppp_api_config(
    home: &Path,
    backend_url: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let config_path = home.join("config.toml");
    let original = match fs::read_to_string(&config_path) {
        Ok(content) => content,
        Err(err) if err.kind() == ErrorKind::NotFound => String::new(),
        Err(err) => return Err(Box::new(err)),
    };
    let mut doc = if original.trim().is_empty() {
        DocumentMut::new()
    } else {
        original.parse::<DocumentMut>()?
    };
    remove_codexppp_legacy_config(&mut doc);
    doc.remove("forced_login_method");
    doc.remove("cli_auth_credentials_store");
    doc.remove("openai_base_url");
    doc.remove("forced_chatgpt_workspace_id");
    doc["model_provider"] = toml_edit::value(CODEXPPP_PROVIDER_ID);
    if doc
        .get("model_providers")
        .is_some_and(|item| !item.is_table())
    {
        doc.remove("model_providers");
    }
    if doc.get("model_providers").is_none() {
        doc["model_providers"] = Item::Table(Table::new());
    }
    let providers = doc["model_providers"]
        .as_table_mut()
        .ok_or("model_providers must be a TOML table")?;
    providers.insert(CODEXPPP_PROVIDER_ID, Item::Table(Table::new()));
    let provider = providers
        .get_mut(CODEXPPP_PROVIDER_ID)
        .and_then(Item::as_table_mut)
        .ok_or("model_providers.codexppp must be a TOML table")?;
    provider["name"] = toml_edit::value("Codex+++");
    provider["wire_api"] = toml_edit::value("responses");
    provider["base_url"] = toml_edit::value(codex_responses_base_url(backend_url));
    provider["requires_openai_auth"] = toml_edit::value(true);
    provider.remove("auth");
    provider.remove("env_key");
    provider.remove("experimental_bearer_token");
    if config_path.is_file() {
        backup_codex_file(&config_path)?;
    }
    fs::write(&config_path, doc.to_string())?;
    Ok(())
}

fn write_codexppp_api_auth(home: &Path, api_key: &str) -> Result<(), Box<dyn std::error::Error>> {
    let key = api_key.trim();
    if key.is_empty() {
        return Err("login_failed".into());
    }
    fs::create_dir_all(home)?;
    let auth_path = home.join("auth.json");
    if auth_path.is_file() {
        backup_codex_file(&auth_path)?;
    }
    let auth = serde_json::json!({
        "auth_mode": "apikey",
        "OPENAI_API_KEY": key
    });
    fs::write(
        auth_path,
        format!("{}\n", serde_json::to_string_pretty(&auth)?),
    )?;
    Ok(())
}

fn remove_codexppp_config(home: &Path) -> Result<(), Box<dyn std::error::Error>> {
    let config_path = home.join("config.toml");
    let original = match fs::read_to_string(&config_path) {
        Ok(content) => content,
        Err(err) if err.kind() == ErrorKind::NotFound => return Ok(()),
        Err(err) => return Err(Box::new(err)),
    };
    let mut doc = original.parse::<DocumentMut>()?;
    if !remove_codexppp_legacy_config(&mut doc) {
        return Ok(());
    }
    backup_codex_file(&config_path)?;
    fs::write(&config_path, doc.to_string())?;
    Ok(())
}

fn remove_codexppp_legacy_config(doc: &mut DocumentMut) -> bool {
    let mut changed = false;
    if doc.remove("codexppp").is_some() {
        changed = true;
    }
    if doc
        .get("model_provider")
        .and_then(Item::as_str)
        .is_some_and(|value| value == CODEXPPP_PROVIDER_ID)
    {
        doc.remove("model_provider");
        changed = true;
    }
    if let Some(providers) = doc.get_mut("model_providers").and_then(Item::as_table_mut) {
        if providers.remove(CODEXPPP_PROVIDER_ID).is_some() {
            changed = true;
        }
    }
    if doc
        .get("model_providers")
        .and_then(Item::as_table)
        .is_some_and(|providers| providers.is_empty())
    {
        doc.remove("model_providers");
        changed = true;
    }
    changed
}

fn backup_codex_file(path: &Path) -> Result<(), Box<dyn std::error::Error>> {
    if !path.is_file() {
        return Ok(());
    }
    let backup_dir = codexppp_backup_root()?.join(format!("{}", now_unix_nanos()));
    fs::create_dir_all(&backup_dir)?;
    let file_name = path
        .file_name()
        .ok_or("Codex backup file name is invalid")?;
    fs::copy(path, backup_dir.join(file_name))?;
    Ok(())
}

fn codexppp_backup_root() -> Result<PathBuf, Box<dyn std::error::Error>> {
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_TEST_CODEX_BACKUP_HOME") {
        if !value.trim().is_empty() {
            return Ok(PathBuf::from(value));
        }
    }
    Ok(local_app_data_root()?
        .join("Codex+++")
        .join("codex-backups"))
}

fn remove_managed_codex_home() -> Result<(), Box<dyn std::error::Error>> {
    let Ok(home) = managed_codex_home() else {
        return Ok(());
    };
    match fs::remove_dir_all(home) {
        Ok(()) => {}
        Err(err) if err.kind() == ErrorKind::NotFound => {}
        Err(err) => return Err(Box::new(err)),
    }
    Ok(())
}

fn remove_data_home_legacy_artifacts() -> Result<(), Box<dyn std::error::Error>> {
    let Ok(local_app_data) = local_app_data_root() else {
        return Ok(());
    };
    let home = local_app_data.join("Codex+++");
    for name in [
        "provider-token.txt",
        CODEXPPP_PROVIDER_TOKEN_FILE,
        CODEXPPP_PROVIDER_TOKEN_SCRIPT_FILE,
        "codex-provider-account.txt",
    ] {
        match fs::remove_file(home.join(name)) {
            Ok(()) => {}
            Err(err) if err.kind() == ErrorKind::NotFound => {}
            Err(err) => return Err(Box::new(err)),
        }
    }
    Ok(())
}

fn codexppp_config_clean() -> bool {
    let user_ready = codex_home()
        .ok()
        .as_deref()
        .is_none_or(codexppp_config_ready_at);
    let managed_clean = managed_codex_home()
        .ok()
        .is_none_or(|home| !home.exists());
    user_ready && managed_clean
}

fn codexppp_config_ready_at(home: &Path) -> bool {
    let Ok(original) = fs::read_to_string(home.join("config.toml")) else {
        return true;
    };
    let Ok(doc) = original.parse::<DocumentMut>() else {
        return false;
    };
    if doc.get("codexppp").is_some() {
        return false;
    }
    let active_provider = doc
        .get("model_provider")
        .and_then(Item::as_str)
        .is_some_and(|value| value == CODEXPPP_PROVIDER_ID);
    let provider = doc
        .get("model_providers")
        .and_then(|providers| providers.get(CODEXPPP_PROVIDER_ID));
    if !active_provider && provider.is_none() {
        return true;
    }
    active_provider
        && provider.is_some_and(codexppp_api_provider_complete)
        && codex_auth_api_key_at(home).is_some()
}

fn codexppp_api_provider_complete(provider: &Item) -> bool {
    provider
        .get("wire_api")
        .and_then(Item::as_str)
        .is_some_and(|value| value == "responses")
        && provider
            .get("base_url")
            .and_then(Item::as_str)
            .map(str::trim)
            .is_some_and(|value| !value.is_empty())
        && provider
            .get("requires_openai_auth")
            .and_then(Item::as_bool)
            .is_some_and(|value| value)
        && provider.get("auth").is_none()
        && provider.get("env_key").is_none()
        && provider.get("experimental_bearer_token").is_none()
}

fn remove_codexppp_api_auth(
    home: &Path,
    backup_existing: bool,
) -> Result<(), Box<dyn std::error::Error>> {
    let auth_path = home.join("auth.json");
    let mut auth = match fs::read_to_string(&auth_path) {
        Ok(content) => match serde_json::from_str::<Value>(&content)? {
            Value::Object(map) => map,
            _ => return Ok(()),
        },
        Err(err) if err.kind() == ErrorKind::NotFound => return Ok(()),
        Err(err) => return Err(Box::new(err)),
    };
    let mut changed = false;
    for key in ["OPENAI_API_KEY", "openai_api_key"] {
        let should_remove = auth
            .get(key)
            .and_then(Value::as_str)
            .is_some_and(is_codexppp_provider_key);
        if should_remove {
            auth.remove(key);
            changed = true;
        }
    }
    if !changed {
        return Ok(());
    }
    if backup_existing {
        backup_codex_file(&auth_path)?;
    }
    if auth.is_empty() {
        match fs::remove_file(auth_path) {
            Ok(()) => {}
            Err(err) if err.kind() == ErrorKind::NotFound => {}
            Err(err) => return Err(Box::new(err)),
        }
        return Ok(());
    }
    fs::write(
        auth_path,
        format!("{}\n", serde_json::to_string_pretty(&Value::Object(auth))?),
    )?;
    Ok(())
}

fn is_codexppp_provider_key(value: &str) -> bool {
    let trimmed = value.trim();
    trimmed.starts_with("cxp_")
        || (trimmed.len() == 67
            && trimmed.starts_with("sk-")
            && trimmed[3..].chars().all(|ch| ch.is_ascii_hexdigit()))
}

fn codex_responses_base_url(backend_url: &str) -> String {
    format!("{}/codex/v1", backend_url.trim_end_matches('/'))
}

fn apply_managed_codex_environment(
    cmd: &mut Command,
    target: &CodexLaunchTarget,
) -> Result<(), String> {
    let _ = (cmd, target);
    Ok(())
}

fn codex_account_from_auth() -> Option<String> {
    for path in codex_auth_candidate_paths() {
        let Ok(content) = fs::read_to_string(path) else {
            continue;
        };
        let Ok(auth) = serde_json::from_str::<Value>(&content) else {
            continue;
        };
        if let Some(account) = codex_account_from_auth_value(&auth) {
            return Some(account);
        }
    }
    None
}

fn codex_account_from_auth_value(auth: &Value) -> Option<String> {
    if let Some(tokens) = auth.get("tokens") {
        for key in ["id_token", "access_token"] {
            if let Some(token) = tokens.get(key).and_then(Value::as_str) {
                if let Some(account) = account_from_jwt_claims(token) {
                    return Some(account);
                }
            }
        }
        if let Some(account_id) = tokens.get("account_id").and_then(Value::as_str) {
            if !account_id.trim().is_empty() {
                return Some(account_id.trim().to_string());
            }
        }
    }
    auth.get("account_id")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(ToOwned::to_owned)
        .or_else(|| recursive_jwt_account(auth))
        .or_else(|| recursive_named_account(auth))
        .or_else(|| codex_auth_api_key(auth).map(|key| mask_api_key(&key)))
}

fn codex_auth_mode() -> String {
    for path in codex_auth_candidate_paths() {
        let Ok(content) = fs::read_to_string(path) else {
            continue;
        };
        let Ok(auth) = serde_json::from_str::<Value>(&content) else {
            continue;
        };
        if auth_value_has_chatgpt_login(&auth) {
            return "chatgpt".to_string();
        }
        if codex_auth_api_key(&auth).is_some() {
            return "api_key".to_string();
        }
    }
    String::new()
}

fn codex_chatgpt_auth_present_at(home: &Path) -> bool {
    let Ok(content) = fs::read_to_string(home.join("auth.json")) else {
        return false;
    };
    let Ok(auth) = serde_json::from_str::<Value>(&content) else {
        return false;
    };
    auth_value_has_chatgpt_login(&auth)
}

fn auth_value_has_chatgpt_login(auth: &Value) -> bool {
    if let Some(tokens) = auth.get("tokens") {
        if ["id_token", "access_token", "refresh_token"]
            .iter()
            .any(|key| {
                tokens
                    .get(*key)
                    .and_then(Value::as_str)
                    .map(str::trim)
                    .is_some_and(|value| !value.is_empty())
            })
        {
            return true;
        }
    }
    auth.get("auth_mode")
        .and_then(Value::as_str)
        .is_some_and(|mode| mode.eq_ignore_ascii_case("chatgpt"))
}

fn codex_auth_api_key(auth: &Value) -> Option<String> {
    auth.get("OPENAI_API_KEY")
        .or_else(|| auth.get("openai_api_key"))
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(ToOwned::to_owned)
}

fn codex_auth_api_key_at(home: &Path) -> Option<String> {
    let content = fs::read_to_string(home.join("auth.json")).ok()?;
    let auth = serde_json::from_str::<Value>(&content).ok()?;
    codex_auth_api_key(&auth)
}

fn mask_api_key(value: &str) -> String {
    let trimmed = value.trim();
    if trimmed.len() <= 12 {
        return trimmed.to_string();
    }
    let prefix_len = trimmed.len().min(10);
    let suffix_len = 4;
    format!(
        "{}...{}",
        &trimmed[..prefix_len],
        &trimmed[trimmed.len() - suffix_len..]
    )
}

fn codex_auth_candidate_paths() -> Vec<PathBuf> {
    let mut paths = Vec::new();
    if let Ok(home) = codex_home() {
        push_unique_path(&mut paths, home.join("auth.json"));
    }
    paths
}

fn push_unique_path(paths: &mut Vec<PathBuf>, path: PathBuf) {
    if !paths.iter().any(|existing| existing == &path) {
        paths.push(path);
    }
}

fn recursive_jwt_account(value: &Value) -> Option<String> {
    match value {
        Value::String(token) => account_from_jwt_claims(token),
        Value::Array(items) => items.iter().find_map(recursive_jwt_account),
        Value::Object(map) => {
            for key in ["id_token", "access_token", "refresh_token", "token"] {
                if let Some(account) = map
                    .get(key)
                    .and_then(Value::as_str)
                    .and_then(account_from_jwt_claims)
                {
                    return Some(account);
                }
            }
            map.values().find_map(recursive_jwt_account)
        }
        _ => None,
    }
}

fn recursive_named_account(value: &Value) -> Option<String> {
    match value {
        Value::Array(items) => items.iter().find_map(recursive_named_account),
        Value::Object(map) => {
            for key in ["email", "account_email", "user_email"] {
                if let Some(account) = map
                    .get(key)
                    .and_then(Value::as_str)
                    .map(str::trim)
                    .filter(|value| !value.is_empty() && value.contains('@'))
                {
                    return Some(account.to_string());
                }
            }
            for key in ["account_id", "chatgpt_account_id", "user_id"] {
                if let Some(account) = map
                    .get(key)
                    .and_then(Value::as_str)
                    .map(str::trim)
                    .filter(|value| !value.is_empty())
                {
                    return Some(account.to_string());
                }
            }
            map.values().find_map(recursive_named_account)
        }
        _ => None,
    }
}

fn account_from_jwt_claims(token: &str) -> Option<String> {
    let claims = jwt_claims(token)?;
    for key in ["email", "name"] {
        if let Some(value) = claims.get(key).and_then(Value::as_str) {
            if !value.trim().is_empty() {
                return Some(value.trim().to_string());
            }
        }
    }
    let auth = claims.get("https://api.openai.com/auth")?;
    for key in ["chatgpt_account_id", "user_id"] {
        if let Some(value) = auth.get(key).and_then(Value::as_str) {
            if !value.trim().is_empty() {
                return Some(value.trim().to_string());
            }
        }
    }
    None
}

fn jwt_claims(token: &str) -> Option<Value> {
    let mut parts = token.split('.');
    let _header = parts.next()?;
    let payload = parts.next()?;
    let decoded = URL_SAFE_NO_PAD
        .decode(payload.as_bytes())
        .or_else(|_| URL_SAFE.decode(payload.as_bytes()))
        .ok()?;
    serde_json::from_slice(&decoded).ok()
}

fn codex_home() -> Result<PathBuf, Box<dyn std::error::Error>> {
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_TEST_CODEX_HOME") {
        if !value.trim().is_empty() {
            return Ok(PathBuf::from(value));
        }
    }
    let user_profile = env::var("USERPROFILE").or_else(|_| env::var("HOME"))?;
    Ok(PathBuf::from(user_profile).join(".codex"))
}

fn managed_codex_home() -> Result<PathBuf, Box<dyn std::error::Error>> {
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME") {
        if !value.trim().is_empty() {
            return Ok(PathBuf::from(value));
        }
    }
    Ok(local_app_data_root()?
        .join("Codex+++")
        .join("codex-home"))
}

fn local_app_data_root() -> Result<PathBuf, Box<dyn std::error::Error>> {
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_TEST_LOCAL_APP_DATA") {
        if !value.trim().is_empty() {
            return Ok(PathBuf::from(value));
        }
    }
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_TEST_CODEX_HOME") {
        if !value.trim().is_empty() {
            let codex_home = PathBuf::from(value);
            if let Some(parent) = codex_home.parent() {
                return Ok(parent.join("local-appdata"));
            }
        }
    }
    let local_app_data = env::var_os("LOCALAPPDATA").ok_or("LOCALAPPDATA is not set")?;
    Ok(PathBuf::from(local_app_data))
}

fn find_codex_launch_target() -> Option<CodexLaunchTarget> {
    if let Ok(value) = env::var("CODEXPPP_CODEX_COMMAND") {
        if !value.trim().is_empty() {
            return resolve_command_value(value.trim()).map(codex_launch_target_from_command);
        }
    }
    find_command_on_path("codex")
        .or_else(find_common_codex_command)
        .map(codex_launch_target_from_command)
        .or_else(|| {
            #[cfg(windows)]
            {
                installed_codex_app_user_model_id().map(CodexLaunchTarget::AppUserModelId)
            }
            #[cfg(not(windows))]
            {
                None
            }
        })
}

#[cfg(test)]
fn find_codex_command() -> Option<PathBuf> {
    match find_codex_launch_target()? {
        CodexLaunchTarget::Command(command) => Some(command),
        CodexLaunchTarget::AppUserModelId(_) => None,
    }
}

fn resolve_command_value(value: &str) -> Option<PathBuf> {
    let candidate = PathBuf::from(value);
    if command_value_has_directory(&candidate) {
        return existing_command_path(candidate);
    }
    find_command_on_path(value)
}

fn command_value_has_directory(path: &Path) -> bool {
    path.is_absolute()
        || path
            .parent()
            .map(|parent| !parent.as_os_str().is_empty())
            .unwrap_or(false)
}

fn existing_command_path(path: PathBuf) -> Option<PathBuf> {
    if path.is_file() {
        return Some(path);
    }
    if cfg!(windows) && path.extension().is_none() {
        for ext in ["exe", "cmd", "bat"] {
            let with_ext = path.with_extension(ext);
            if with_ext.is_file() {
                return Some(with_ext);
            }
        }
    }
    None
}

fn find_command_on_path(command: &str) -> Option<PathBuf> {
    let path = env::var_os("PATH")?;
    let command_path = Path::new(command);
    let has_extension = command_path.extension().is_some();
    for dir in env::split_paths(&path) {
        if cfg!(windows) && !has_extension {
            for ext in ["exe", "cmd", "bat"] {
                let candidate = dir.join(format!("{command}.{ext}"));
                if candidate.is_file() {
                    return Some(candidate);
                }
            }
        }
        let direct = dir.join(command);
        if direct.is_file() {
            return Some(direct);
        }
    }
    None
}

fn codex_launch_target_from_command(command: PathBuf) -> CodexLaunchTarget {
    #[cfg(windows)]
    if let Some(app_id) = codex_app_user_model_id_from_command(&command) {
        return CodexLaunchTarget::AppUserModelId(app_id);
    }
    #[cfg(windows)]
    if is_windows_store_codex_alias(&command) {
        if let Some(app_id) = installed_codex_app_user_model_id() {
            return CodexLaunchTarget::AppUserModelId(app_id);
        }
    }
    CodexLaunchTarget::Command(command)
}

#[cfg(windows)]
fn is_windows_store_codex_alias(command: &Path) -> bool {
    let normalized = command
        .to_string_lossy()
        .replace('/', "\\")
        .to_ascii_lowercase();
    normalized.ends_with("\\microsoft\\windowsapps\\codex.exe")
}

fn codex_app_user_model_id_from_command(command: &Path) -> Option<String> {
    if !cfg!(windows) {
        return None;
    }
    command.components().find_map(|component| {
        let value = component.as_os_str().to_string_lossy();
        if !value.starts_with("OpenAI.Codex_") {
            return None;
        }
        let publisher = value.split("__").nth(1)?;
        if publisher.trim().is_empty() {
            return None;
        }
        Some(format!("OpenAI.Codex_{publisher}!App"))
    })
}

#[cfg(windows)]
fn find_common_codex_command() -> Option<PathBuf> {
    let direct_candidates = [
        env::var_os("LOCALAPPDATA")
            .map(PathBuf::from)
            .map(|path| path.join("Microsoft").join("WindowsApps").join("codex.exe")),
        env::var_os("APPDATA")
            .map(PathBuf::from)
            .map(|path| path.join("npm").join("codex.cmd")),
        env::var_os("APPDATA")
            .map(PathBuf::from)
            .map(|path| path.join("npm").join("codex.exe")),
    ];
    direct_candidates
        .into_iter()
        .flatten()
        .find(|path| path.is_file())
}

#[cfg(windows)]
fn installed_codex_desktop_app_command() -> Option<PathBuf> {
    static INSTALLED_CODEX_DESKTOP_APP_COMMAND: OnceLock<PathBuf> = OnceLock::new();
    if let Some(command) = INSTALLED_CODEX_DESKTOP_APP_COMMAND.get() {
        return Some(command.clone());
    }
    let command = installed_codex_desktop_app_command_from_appx_package()
        .or_else(installed_codex_desktop_app_command_from_windows_apps)?;
    let _ = INSTALLED_CODEX_DESKTOP_APP_COMMAND.set(command.clone());
    Some(command)
}

#[cfg(windows)]
fn installed_codex_desktop_app_command_from_appx_package() -> Option<PathBuf> {
    let mut cmd = Command::new("powershell");
    cmd.args([
        "-NoProfile",
        "-NonInteractive",
        "-ExecutionPolicy",
        "Bypass",
        "-Command",
        "Get-AppxPackage -Name OpenAI.Codex | Sort-Object Version -Descending | Select-Object -First 1 -ExpandProperty InstallLocation",
    ])
    .stdin(Stdio::null())
    .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    let output = cmd.output().ok()?;
    if !output.status.success() {
        return None;
    }
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .map(|line| line.trim().trim_start_matches('\u{feff}'))
        .filter(|line| !line.is_empty())
        .map(PathBuf::from)
        .find_map(codex_desktop_command_from_package_root)
}

#[cfg(windows)]
fn installed_codex_desktop_app_command_from_windows_apps() -> Option<PathBuf> {
    let program_files = env::var_os("ProgramFiles").map(PathBuf::from)?;
    let windows_apps = program_files.join("WindowsApps");
    let mut packages = fs::read_dir(windows_apps)
        .ok()?
        .filter_map(Result::ok)
        .map(|entry| entry.path())
        .filter(|path| {
            path.file_name()
                .map(|name| name.to_string_lossy().starts_with("OpenAI.Codex_"))
                .unwrap_or(false)
        })
        .collect::<Vec<_>>();
    packages.sort_by(|a, b| {
        let a_name = a.file_name().and_then(|name| name.to_str()).unwrap_or("");
        let b_name = b.file_name().and_then(|name| name.to_str()).unwrap_or("");
        b_name.cmp(a_name)
    });
    packages
        .into_iter()
        .find_map(codex_desktop_command_from_package_root)
}

#[cfg(windows)]
fn codex_desktop_command_from_package_root(package_root: PathBuf) -> Option<PathBuf> {
    let command = package_root.join("app").join("Codex.exe");
    command.is_file().then_some(command)
}

#[cfg(windows)]
fn installed_codex_app_user_model_id() -> Option<String> {
    installed_codex_desktop_app_command()
        .as_deref()
        .and_then(codex_app_user_model_id_from_command)
        .or_else(installed_codex_app_user_model_id_from_start_apps)
}

#[cfg(windows)]
fn installed_codex_app_user_model_id_from_start_apps() -> Option<String> {
    static START_APPS_CODEX_APP_ID: OnceLock<String> = OnceLock::new();
    if let Some(app_id) = START_APPS_CODEX_APP_ID.get() {
        return Some(app_id.clone());
    }
    let app_id = query_codex_app_user_model_id_from_start_apps()?;
    let _ = START_APPS_CODEX_APP_ID.set(app_id.clone());
    Some(app_id)
}

#[cfg(windows)]
fn query_codex_app_user_model_id_from_start_apps() -> Option<String> {
    let mut cmd = Command::new("powershell");
    cmd.args([
        "-NoProfile",
        "-NonInteractive",
        "-ExecutionPolicy",
        "Bypass",
        "-Command",
        "Get-StartApps | ForEach-Object { $_.AppID }",
    ])
    .stdin(Stdio::null())
    .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    let output = cmd.output().ok()?;
    if !output.status.success() {
        return None;
    }
    codex_app_user_model_id_from_start_apps_output(&String::from_utf8_lossy(&output.stdout))
}

fn codex_app_user_model_id_from_start_apps_output(output: &str) -> Option<String> {
    output
        .lines()
        .map(str::trim)
        .find(|line| line.eq_ignore_ascii_case(WINDOWS_STORE_CODEX_APP_ID))
        .map(|_| WINDOWS_STORE_CODEX_APP_ID.to_string())
}

#[cfg(not(windows))]
fn find_common_codex_command() -> Option<PathBuf> {
    None
}

fn codex_process_running() -> bool {
    if let Ok(value) = env::var("CODEXPPP_CODEX_RUNNING_FOR_TEST") {
        match value.trim() {
            "1" | "true" | "TRUE" => return true,
            "0" | "false" | "FALSE" => return false,
            _ => {}
        }
    }
    codex_process_running_from_system()
}

fn stop_codex_processes() -> Result<(), String> {
    let ids = codex_process_ids_from_system();
    for pid in ids {
        stop_process_tree(pid)?;
    }
    Ok(())
}

#[cfg(windows)]
fn codex_process_running_from_system() -> bool {
    codex_process_snapshots()
        .iter()
        .any(|snapshot| is_codex_process(&snapshot.name, &snapshot.command_line))
}

#[cfg(windows)]
fn codex_process_ids_from_system() -> Vec<u32> {
    codex_process_snapshots()
        .into_iter()
        .filter(|snapshot| is_codex_process(&snapshot.name, &snapshot.command_line))
        .map(|snapshot| snapshot.pid)
        .collect()
}

#[cfg(windows)]
struct ProcessSnapshot {
    pid: u32,
    name: String,
    command_line: String,
}

#[cfg(windows)]
fn codex_process_snapshots() -> Vec<ProcessSnapshot> {
    let mut system = System::new();
    system.refresh_processes();
    system
        .processes()
        .iter()
        .map(|(pid, process)| ProcessSnapshot {
            pid: pid.as_u32(),
            name: process.name().to_string(),
            command_line: process.cmd().join(" "),
        })
        .collect()
}

#[cfg(windows)]
fn stop_process_tree(pid: u32) -> Result<(), String> {
    let mut cmd = Command::new("taskkill");
    cmd.args(["/PID", &pid.to_string(), "/T", "/F"])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    cmd.status()
        .map_err(|_| "codex_stop_failed".to_string())
        .map(|_| ())
}

#[cfg(not(windows))]
fn codex_process_running_from_system() -> bool {
    Command::new("pgrep")
        .args(["-f", "codex"])
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

#[cfg(not(windows))]
fn codex_process_ids_from_system() -> Vec<u32> {
    Vec::new()
}

#[cfg(not(windows))]
fn stop_process_tree(_pid: u32) -> Result<(), String> {
    Ok(())
}

#[cfg(test)]
fn codex_process_ids_from_output(output: &str) -> Vec<u32> {
    output
        .lines()
        .filter_map(|line| {
            let mut parts = line.splitn(3, '\t');
            let pid = parts.next()?.trim().parse::<u32>().ok()?;
            let name = parts.next().unwrap_or_default();
            let command_line = parts.next().unwrap_or_default();
            if is_codex_process(name, command_line) {
                Some(pid)
            } else {
                None
            }
        })
        .collect()
}

fn is_codex_process(name: &str, command_line: &str) -> bool {
    let process_name = name.trim().to_ascii_lowercase();
    if process_name == "codex" || process_name == "codex.exe" {
        return true;
    }
    command_line_mentions_codex(command_line)
}

fn command_line_mentions_codex(value: &str) -> bool {
    value
        .to_ascii_lowercase()
        .replace('/', "\\")
        .split(|ch: char| ch.is_whitespace() || ch == '"' || ch == '\'')
        .map(|token| {
            token.trim_matches(|ch: char| {
                matches!(ch, '(' | ')' | ',' | ';' | '[' | ']' | '{' | '}')
            })
        })
        .any(token_is_codex_entry)
}

fn token_is_codex_entry(token: &str) -> bool {
    const CODEX_ENTRIES: [&str; 5] = [
        "codex.exe",
        "codex.cmd",
        "codex.bat",
        "codex.ps1",
        "codex.js",
    ];
    CODEX_ENTRIES
        .iter()
        .any(|entry| token == *entry || token.ends_with(&format!("\\{entry}")))
}

fn suppress_command_window(cmd: &mut Command) {
    #[cfg(windows)]
    {
        cmd.creation_flags(CREATE_NO_WINDOW);
    }
    #[cfg(not(windows))]
    {
        let _ = cmd;
    }
}

fn now_unix_nanos() -> u128 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos()
}

fn cn_available(value: bool) -> String {
    if value {
        "可用".to_string()
    } else {
        "不可用".to_string()
    }
}

fn configured_backend_api_base() -> String {
    let runtime = env::var("CODEXPPP_BACKEND_API_BASE").ok();
    let buildtime = option_env!("CODEXPPP_BACKEND_API_BASE").map(str::to_string);
    runtime
        .into_iter()
        .chain(buildtime)
        .find_map(|value| normalize_backend_api_base(&value))
        .unwrap_or_else(|| DEFAULT_BACKEND_API_BASE.to_string())
}

fn normalize_backend_api_base(raw: &str) -> Option<String> {
    let trimmed = raw.trim().trim_end_matches('/');
    if trimmed.is_empty() {
        return None;
    }
    if trimmed.ends_with("/api") {
        return Some(trimmed.to_string());
    }
    Some(format!("{trimmed}/api"))
}

fn build_device_identity() -> DeviceIdentity {
    let name = device_display_name();
    let seed = stable_device_seed();
    DeviceIdentity {
        device_name: name,
        fingerprint: format!("codexppp-{}", fnv1a64_hex(seed.as_bytes())),
    }
}

fn device_display_name() -> String {
    let computer =
        first_env(&["COMPUTERNAME", "HOSTNAME"]).unwrap_or_else(|| "Windows".to_string());
    format!("Windows 设备 {}", computer)
}

fn stable_device_seed() -> String {
    let machine = windows_machine_guid()
        .or_else(|| first_env(&["COMPUTERNAME", "HOSTNAME"]))
        .unwrap_or_else(|| "unknown-machine".to_string());
    let user = first_env(&["USERNAME", "USER"]).unwrap_or_else(|| "unknown-user".to_string());
    format!("codexppp-device:{machine}:{user}")
}

fn first_env(keys: &[&str]) -> Option<String> {
    keys.iter()
        .filter_map(|key| env::var(key).ok())
        .map(|value| value.trim().to_string())
        .find(|value| !value.is_empty())
}

fn windows_machine_guid() -> Option<String> {
    if !cfg!(windows) {
        return None;
    }
    let output = Command::new("reg")
        .args([
            "query",
            r"HKLM\SOFTWARE\Microsoft\Cryptography",
            "/v",
            "MachineGuid",
        ])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let stdout = String::from_utf8_lossy(&output.stdout);
    stdout.lines().find_map(parse_machine_guid_line)
}

fn parse_machine_guid_line(line: &str) -> Option<String> {
    if !line.contains("MachineGuid") {
        return None;
    }
    line.split_whitespace()
        .last()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

fn fnv1a64_hex(bytes: &[u8]) -> String {
    let mut hash = 0xcbf29ce484222325u64;
    for byte in bytes {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(0x100000001b3);
    }
    format!("{hash:016x}")
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::{Mutex, OnceLock};

    fn env_lock() -> std::sync::MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
            .lock()
            .unwrap_or_else(|err| err.into_inner())
    }

    fn assert_no_codexppp_provider(written: &str) {
        assert!(!written.contains("model_provider = \"codexppp\""));
        assert!(!written.contains("[model_providers.codexppp]"));
        assert!(!written.contains("[model_providers.codexppp.auth]"));
        assert!(!written.contains("Codex+++"));
        assert!(!written.contains("codex-provider-token"));
        assert!(!written.contains("requires_openai_auth"));
        assert!(!written.contains("experimental_bearer_token"));
        assert!(!written.contains("[codexppp]"));
        assert!(!written.contains("auth_method"));
        assert!(!written.contains("token_helper"));
        assert!(!written.contains("token_helper_script"));
    }

    fn assert_codexppp_api_key_config(written: &str) {
        let doc = written.parse::<DocumentMut>().unwrap();
        assert_eq!(
            doc.get("model_provider").and_then(Item::as_str),
            Some("codexppp")
        );
        let provider = doc
            .get("model_providers")
            .and_then(Item::as_table)
            .and_then(|providers| providers.get("codexppp"))
            .and_then(Item::as_table)
            .expect("codexppp provider table missing");
        assert_eq!(provider.get("name").and_then(Item::as_str), Some("Codex+++"));
        assert_eq!(
            provider.get("wire_api").and_then(Item::as_str),
            Some("responses")
        );
        assert_eq!(
            provider.get("base_url").and_then(Item::as_str),
            Some("http://localhost:8787/api/codex/v1")
        );
        assert_eq!(
            provider
                .get("requires_openai_auth")
                .and_then(Item::as_bool),
            Some(true)
        );
        assert!(written.contains("[model_providers.codexppp]"));
        assert!(!written.contains("[model_providers.codexppp.auth]"));
        assert!(!written.contains("model_providers = {"));
        assert!(!written.contains("forced_login_method"));
        assert!(!written.contains("cli_auth_credentials_store"));
        assert!(!written.contains("openai_base_url"));
        assert!(written.contains("requires_openai_auth = true"));
        assert!(!written.contains("env_key"));
        assert!(!written.contains("experimental_bearer_token"));
        assert!(!written.contains("secret-provider-token"));
    }

    #[test]
    fn prepare_removes_legacy_managed_home_without_touching_clean_user_config() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-test-{}", now_unix_nanos()));
        let managed = dir.join("managed");
        let user = dir.join("user");
        fs::create_dir_all(&managed).unwrap();
        fs::create_dir_all(&user).unwrap();
        let config = managed.join("config.toml");
        let user_config = user.join("config.toml");
        fs::write(&user_config, "model = \"user-normal\"\n").unwrap();
        fs::write(
            user.join("auth.json"),
            serde_json::json!({
                "tokens": {
                    "account_id": "user-account",
                    "access_token": "opaque-access-token"
                }
            })
            .to_string(),
        )
        .unwrap();
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);

        fs::write(&config, "broken = [").unwrap();
        fs::write(managed.join("codex-provider-token"), "old-token").unwrap();
        let target =
            CodexLaunchTarget::Command(dir.join(if cfg!(windows) { "codex.exe" } else { "codex" }));
        prepare_codex_for_target(&target, "http://localhost:8787/api", "secret-provider-token")
            .unwrap();
        assert!(!managed.exists());
        assert_eq!(
            fs::read_to_string(&user_config).unwrap(),
            "model = \"user-normal\"\n"
        );

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn prepare_removes_only_codexppp_entries_from_user_config() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-preserve-test-{}", now_unix_nanos()));
        let user = dir.join("user");
        let managed = dir.join("managed");
        let backup_home = dir.join("backups");
        fs::create_dir_all(&user).unwrap();
        fs::create_dir_all(&managed).unwrap();
        let config = user.join("config.toml");
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        let previous_backup = env::var("CODEXPPP_TEST_CODEX_BACKUP_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);
        env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", &backup_home);

        let original = r#"
model = "gpt-5.4"
model_provider = "codexppp"

[features]
web_search = true

[desktop]
notifications = false

[memories]
enabled = true

[hooks]
on_start = "echo ready"

[projects.demo]
trust_level = "trusted"

[codexppp]
managed = true
local_proxy_base_url = "http://legacy-proxy.invalid"
proxy = "http://legacy-proxy.invalid"
endpoint = "http://legacy-endpoint.invalid"
base_url = "http://legacy-base.invalid"
api_key = "legacy-secret"
route_id = "legacy-route"
gatewayPath = "/legacy-gateway"
session_token_hint = "legacy-token-fragment"

[mcp_servers.filesystem]
command = "npx"
args = ["-y", "@modelcontextprotocol/server-filesystem"]

[model_providers.existing]
base_url = "https://example.invalid/v1"
wire_api = "responses"

[model_providers.codexppp]
name = "Codex+++"
base_url = "http://legacy.invalid/codex/v1"
wire_api = "responses"
"#
        .trim_start();
        fs::write(&config, original).unwrap();
        fs::write(
            user.join("auth.json"),
            serde_json::json!({
                "tokens": {
                    "account_id": "user-account",
                    "access_token": "opaque-access-token"
                }
            })
            .to_string(),
        )
        .unwrap();
        fs::write(managed.join("codexppp-state.txt"), "legacy-state").unwrap();
        fs::write(managed.join("provider-token.txt"), "legacy-provider-token").unwrap();
        fs::create_dir_all(managed.join("codexppp-backups")).unwrap();
        fs::write(
            managed.join("codexppp-backups").join("config.toml"),
            "legacy-backup",
        )
        .unwrap();

        let target =
            CodexLaunchTarget::Command(dir.join(if cfg!(windows) { "codex.exe" } else { "codex" }));
        prepare_codex_for_target(&target, "http://localhost:8787/api", "secret-provider-token")
            .unwrap();
        let written = fs::read_to_string(&config).unwrap();

        assert!(written.contains("model = \"gpt-5.4\""));
        assert!(written.contains("[features]"));
        assert!(written.contains("[desktop]"));
        assert!(written.contains("[memories]"));
        assert!(written.contains("[hooks]"));
        assert!(written.contains("[projects.demo]"));
        assert!(written.contains("[mcp_servers.filesystem]"));
        assert!(written.contains("[model_providers.existing]"));
        assert_no_codexppp_provider(&written);
        for legacy in [
            "legacy-proxy.invalid",
            "legacy-endpoint.invalid",
            "legacy-base.invalid",
            "legacy-secret",
            "legacy-route",
            "legacy-gateway",
            "legacy-token-fragment",
            "secr...oken",
        ] {
            assert!(
                !written.contains(legacy),
                "managed Codex+++ config kept legacy field value {legacy}"
            );
        }
        assert!(!managed.exists());
        let backup_files = fs::read_dir(&backup_home)
            .unwrap()
            .filter_map(Result::ok)
            .flat_map(|entry| fs::read_dir(entry.path()).unwrap().filter_map(Result::ok))
            .map(|entry| entry.file_name().to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert!(backup_files.iter().any(|name| name == "config.toml"));

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        if let Some(value) = previous_backup {
            env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_BACKUP_HOME");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn codexppp_config_clean_tracks_stale_user_and_managed_state() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-restore-missing-{}", now_unix_nanos()));
        let user = dir.join("user");
        let managed = dir.join("managed");
        fs::create_dir_all(&user).unwrap();
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);

        assert!(codexppp_config_clean());
        fs::write(
            user.join("config.toml"),
            "model_provider = \"codexppp\"\n[model_providers.codexppp]\nbase_url = \"http://legacy.invalid\"\n",
        )
        .unwrap();
        assert!(!codexppp_config_clean());
        remove_codexppp_config(&user).unwrap();
        assert!(codexppp_config_clean());
        fs::create_dir_all(&managed).unwrap();
        assert!(!codexppp_config_clean());
        remove_managed_codex_home().unwrap();
        assert!(codexppp_config_clean());
        fs::write(
            user.join("config.toml"),
            r#"
model_provider = "codexppp"

[model_providers.codexppp]
name = "Codex+++"
wire_api = "responses"
base_url = "http://localhost:8787/api/codex/v1"

[model_providers.codexppp.auth]
command = "powershell.exe"
args = ["-NoProfile", "-WindowStyle", "Hidden", "-ExecutionPolicy", "Bypass", "-File", "C:\\Users\\test\\AppData\\Local\\Codex+++\\codex-provider-token.ps1"]
"#
            .trim_start(),
        )
        .unwrap();
        write_codexppp_api_auth(&user, "secret-provider-token").unwrap();
        assert!(!codexppp_config_clean());
        fs::write(
            user.join("config.toml"),
            r#"
model_provider = "codexppp"

[model_providers.codexppp]
name = "Codex+++"
wire_api = "responses"
base_url = "http://localhost:8787/api/codex/v1"
requires_openai_auth = true
"#
            .trim_start(),
        )
        .unwrap();
        assert!(codexppp_config_clean());

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn device_identity_is_stable_and_redacted() {
        let first = build_device_identity();
        let second = build_device_identity();

        assert_eq!(first.fingerprint, second.fingerprint);
        assert!(first.fingerprint.starts_with("codexppp-"));
        assert_eq!(first.fingerprint.len(), "codexppp-".len() + 16);
        assert!(!first.device_name.trim().is_empty());
    }

    #[test]
    fn machine_guid_parser_reads_registry_output_line() {
        assert_eq!(
            parse_machine_guid_line("    MachineGuid    REG_SZ    abc-def-123"),
            Some("abc-def-123".to_string())
        );
        assert_eq!(parse_machine_guid_line("    Other    REG_SZ    abc"), None);
    }

    #[test]
    fn configured_codex_command_must_resolve_before_prepare() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-command-test-{}", now_unix_nanos()));
        fs::create_dir_all(&dir).unwrap();
        let managed = dir.join("managed");
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        let previous_command = env::var("CODEXPPP_CODEX_COMMAND").ok();
        let previous_running = env::var("CODEXPPP_CODEX_RUNNING_FOR_TEST").ok();
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);
        env::set_var("CODEXPPP_CODEX_COMMAND", dir.join("missing-codex.exe"));
        env::set_var("CODEXPPP_CODEX_RUNNING_FOR_TEST", "0");

        assert!(find_codex_command().is_none());
        match prepare_codex(
            "http://localhost:8787/api".to_string(),
            "secret-provider-token".to_string(),
        ) {
            Ok(_) => panic!("prepare should fail when configured Codex command is missing"),
            Err(err) => assert_eq!(err, "codex_not_detected"),
        }
        assert!(!managed.join("config.toml").exists());

        if let Some(value) = previous_command {
            env::set_var("CODEXPPP_CODEX_COMMAND", value);
        } else {
            env::remove_var("CODEXPPP_CODEX_COMMAND");
        }
        if let Some(value) = previous_running {
            env::set_var("CODEXPPP_CODEX_RUNNING_FOR_TEST", value);
        } else {
            env::remove_var("CODEXPPP_CODEX_RUNNING_FOR_TEST");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn prepare_codex_writes_api_key_login_when_chatgpt_auth_is_missing() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-prepare-login-test-{}", now_unix_nanos()));
        fs::create_dir_all(&dir).unwrap();
        let user_home = dir.join("user-codex");
        let managed = dir.join("managed");
        fs::create_dir_all(&user_home).unwrap();
        fs::create_dir_all(&managed).unwrap();
        fs::write(
            user_home.join("config.toml"),
            "model_provider = \"codexppp\"\n[model_providers.codexppp]\nbase_url = \"http://legacy.invalid\"\n",
        )
        .unwrap();
        fs::write(managed.join("config.toml"), "legacy = true\n").unwrap();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        let previous_command = env::var("CODEXPPP_CODEX_COMMAND").ok();
        let previous_running = env::var("CODEXPPP_CODEX_RUNNING_FOR_TEST").ok();
        let command = dir.join(if cfg!(windows) { "codex.exe" } else { "codex" });
        fs::write(&command, "").unwrap();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user_home);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);
        env::set_var("CODEXPPP_CODEX_COMMAND", &command);
        env::set_var("CODEXPPP_CODEX_RUNNING_FOR_TEST", "0");

        let prepared = prepare_codex(
            "http://localhost:8787/api".to_string(),
            "secret-provider-token".to_string(),
        )
        .expect("detected Codex should allow api login preparation");
        assert_eq!(prepared.codex_running, "不可用");
        assert_eq!(prepared.config_written, "可用");
        assert!(!managed.exists());
        let written = fs::read_to_string(user_home.join("config.toml")).unwrap();
        assert_codexppp_api_key_config(&written);
        let auth: Value =
            serde_json::from_str(&fs::read_to_string(user_home.join("auth.json")).unwrap())
                .unwrap();
        assert_eq!(auth.get("auth_mode").and_then(Value::as_str), Some("apikey"));
        assert_eq!(
            auth.get("OPENAI_API_KEY").and_then(Value::as_str),
            Some("secret-provider-token")
        );
        assert_eq!(
            codex_account_from_auth(),
            Some("secret-pro...oken".to_string())
        );
        assert_eq!(codex_auth_mode(), "api_key");

        if let Some(value) = previous_command {
            env::set_var("CODEXPPP_CODEX_COMMAND", value);
        } else {
            env::remove_var("CODEXPPP_CODEX_COMMAND");
        }
        if let Some(value) = previous_running {
            env::set_var("CODEXPPP_CODEX_RUNNING_FOR_TEST", value);
        } else {
            env::remove_var("CODEXPPP_CODEX_RUNNING_FOR_TEST");
        }
        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn app_id_prepare_cleans_codexppp_config_and_preserves_auth_tokens() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-appid-prepare-test-{}", now_unix_nanos()));
        let user_home = dir.join("user-codex");
        let managed_home = dir.join("managed-codex");
        let backup_home = dir.join("backups");
        fs::create_dir_all(&user_home).unwrap();
        fs::write(
            user_home.join("config.toml"),
            r#"
model = "gpt-5.4"

[features]
web_search = true

[mcp_servers.filesystem]
command = "npx"

[model_providers.codexppp]
base_url = "http://legacy.invalid"
"#
            .trim_start(),
        )
        .unwrap();
        fs::write(
            user_home.join("auth.json"),
            serde_json::json!({
                "tokens": {
                    "account_id": "user-account",
                    "access_token": "opaque-access-token"
                }
            })
            .to_string(),
        )
        .unwrap();
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        let previous_backup = env::var("CODEXPPP_TEST_CODEX_BACKUP_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user_home);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed_home);
        env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", &backup_home);

        let target = CodexLaunchTarget::AppUserModelId("OpenAI.Codex_test!App".to_string());
        prepare_codex_for_target(&target, "http://localhost:8787/api", "secret-provider-token")
            .unwrap();

        let written = fs::read_to_string(user_home.join("config.toml")).unwrap();
        assert!(written.contains("model = \"gpt-5.4\""));
        assert!(written.contains("[features]"));
        assert!(written.contains("[mcp_servers.filesystem]"));
        assert_no_codexppp_provider(&written);
        let auth: Value =
            serde_json::from_str(&fs::read_to_string(user_home.join("auth.json")).unwrap())
                .unwrap();
        assert_eq!(auth.get("OPENAI_API_KEY").and_then(Value::as_str), None);
        assert_eq!(
            auth.pointer("/tokens/account_id").and_then(Value::as_str),
            Some("user-account")
        );
        assert_eq!(codex_account_from_auth(), Some("user-account".to_string()));
        assert!(codexppp_config_clean());
        let backup_files = fs::read_dir(&backup_home)
            .unwrap()
            .filter_map(Result::ok)
            .flat_map(|entry| fs::read_dir(entry.path()).unwrap().filter_map(Result::ok))
            .map(|entry| entry.file_name().to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert!(backup_files.iter().any(|name| name == "config.toml"));
        assert!(!backup_files.iter().any(|name| name == "auth.json"));

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        if let Some(value) = previous_backup {
            env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_BACKUP_HOME");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn old_provider_api_key_is_removed_from_auth_json_without_removing_chatgpt_tokens() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-auth-clean-test-{}", now_unix_nanos()));
        let home = dir.join("codex-home");
        let backup_home = dir.join("backups");
        fs::create_dir_all(&home).unwrap();
        let old_key = format!("sk-{}", "a".repeat(64));
        fs::write(
            home.join("auth.json"),
            serde_json::json!({
                "OPENAI_API_KEY": old_key,
                "openai_api_key": "cxp_legacy-provider-token",
                "tokens": {
                    "account_id": "user-account"
                }
            })
            .to_string(),
        )
        .unwrap();
        let previous_backup = env::var("CODEXPPP_TEST_CODEX_BACKUP_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", &backup_home);

        remove_codexppp_api_auth(&home, true).unwrap();

        let auth: Value =
            serde_json::from_str(&fs::read_to_string(home.join("auth.json")).unwrap()).unwrap();
        assert_eq!(auth.get("OPENAI_API_KEY").and_then(Value::as_str), None);
        assert_eq!(auth.get("openai_api_key").and_then(Value::as_str), None);
        assert_eq!(
            auth.pointer("/tokens/account_id").and_then(Value::as_str),
            Some("user-account")
        );
        let backup_files = fs::read_dir(&backup_home)
            .unwrap()
            .filter_map(Result::ok)
            .flat_map(|entry| fs::read_dir(entry.path()).unwrap().filter_map(Result::ok))
            .map(|entry| entry.file_name().to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert!(backup_files.iter().any(|name| name == "auth.json"));

        if let Some(value) = previous_backup {
            env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_BACKUP_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn data_home_cleanup_removes_stale_provider_account_cache() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-data-clean-test-{}", now_unix_nanos()));
        let data_home = dir.join("Codex+++");
        fs::create_dir_all(&data_home).unwrap();
        fs::write(data_home.join("codex-provider-token"), "old-token").unwrap();
        fs::write(data_home.join("codex-provider-account.txt"), "old-account").unwrap();
        let previous_local_appdata = env::var_os("LOCALAPPDATA");
        env::set_var("LOCALAPPDATA", &dir);

        remove_data_home_legacy_artifacts().unwrap();

        assert!(!data_home.join("codex-provider-token").exists());
        assert!(!data_home.join("codex-provider-account.txt").exists());

        if let Some(value) = previous_local_appdata {
            env::set_var("LOCALAPPDATA", value);
        } else {
            env::remove_var("LOCALAPPDATA");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn configured_codex_command_resolves_existing_path_and_path_name() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-command-path-test-{}", now_unix_nanos()));
        fs::create_dir_all(&dir).unwrap();
        let previous_command = env::var("CODEXPPP_CODEX_COMMAND").ok();
        let previous_path = env::var_os("PATH");
        let file_name = if cfg!(windows) { "codex.exe" } else { "codex" };
        let command = dir.join(file_name);
        fs::write(&command, "").unwrap();
        if cfg!(windows) {
            fs::write(dir.join("codex"), "").unwrap();
            fs::write(dir.join("codex.cmd"), "").unwrap();
        }

        env::set_var("CODEXPPP_CODEX_COMMAND", &command);
        assert_eq!(find_codex_command(), Some(command.clone()));

        env::set_var("CODEXPPP_CODEX_COMMAND", "codex");
        env::set_var("PATH", &dir);
        assert_eq!(find_codex_command(), Some(command.clone()));

        if let Some(value) = previous_command {
            env::set_var("CODEXPPP_CODEX_COMMAND", value);
        } else {
            env::remove_var("CODEXPPP_CODEX_COMMAND");
        }
        if let Some(value) = previous_path {
            env::set_var("PATH", value);
        } else {
            env::remove_var("PATH");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn codex_resource_command_maps_to_store_app_id_on_windows() {
        let dir = env::temp_dir().join(format!(
            "codexppp-command-normalize-test-{}",
            now_unix_nanos()
        ));
        let app_dir = dir
            .join("OpenAI.Codex_26.623.13972.0_x64__2p2nqsd0c76g0")
            .join("app");
        let resources_dir = app_dir.join("resources");
        fs::create_dir_all(&resources_dir).unwrap();
        let resource_command = resources_dir.join("codex.exe");
        fs::write(&resource_command, "").unwrap();

        let target = codex_launch_target_from_command(resource_command.clone());
        if cfg!(windows) {
            let CodexLaunchTarget::AppUserModelId(app_id) = target else {
                panic!("Windows Store Codex paths must launch by AppID");
            };
            assert_eq!(app_id, "OpenAI.Codex_2p2nqsd0c76g0!App");
        } else {
            let CodexLaunchTarget::Command(actual) = target else {
                panic!("non-Windows paths stay command targets");
            };
            assert_eq!(actual, resource_command);
        }

        fs::remove_dir_all(dir).unwrap();
    }

    #[cfg(windows)]
    #[test]
    fn codex_desktop_command_is_detected_from_store_package_root() {
        let dir = env::temp_dir().join(format!("codexppp-store-package-test-{}", now_unix_nanos()));
        let package_root = dir.join("OpenAI.Codex_test");
        let app_dir = package_root.join("app");
        fs::create_dir_all(&app_dir).unwrap();
        let desktop_command = app_dir.join("Codex.exe");
        fs::write(&desktop_command, "").unwrap();

        assert_eq!(
            codex_desktop_command_from_package_root(package_root),
            Some(desktop_command)
        );

        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn public_desktop_error_hides_raw_details() {
        assert_eq!(
            public_desktop_error("Codex config.toml 解析失败：expected value at line 1"),
            "codex_config_unreadable"
        );
        assert_eq!(
            public_desktop_error("Access is denied. (os error 5)"),
            "codex_config_write_failed"
        );
    }

    #[test]
    fn codex_account_is_read_from_local_auth_claims() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-auth-test-{}", now_unix_nanos()));
        fs::create_dir_all(&dir).unwrap();
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &dir);
        let claims = serde_json::json!({
            "email": "codex-user@example.com",
            "https://api.openai.com/auth": {
                "chatgpt_account_id": "account-from-claims"
            }
        });
        let token = format!(
            "header.{}.signature",
            URL_SAFE_NO_PAD.encode(claims.to_string().as_bytes())
        );
        fs::write(
            dir.join("auth.json"),
            serde_json::json!({
                "auth_mode": "chatgpt",
                "tokens": {
                    "id_token": token,
                    "account_id": "fallback-account"
                }
            })
            .to_string(),
        )
        .unwrap();

        assert_eq!(
            codex_account_from_auth(),
            Some("codex-user@example.com".to_string())
        );

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn codex_account_falls_back_to_auth_account_id() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-auth-fallback-test-{}", now_unix_nanos()));
        fs::create_dir_all(&dir).unwrap();
        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &dir);
        fs::write(
            dir.join("auth.json"),
            serde_json::json!({
                "auth_mode": "chatgpt",
                "tokens": {
                    "account_id": "fallback-account"
                }
            })
            .to_string(),
        )
        .unwrap();

        assert_eq!(
            codex_account_from_auth(),
            Some("fallback-account".to_string())
        );

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn codex_account_ignores_appdata_and_managed_auth_files() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-auth-source-test-{}", now_unix_nanos()));
        let codex_home = dir.join("codex-home");
        let managed_home = dir.join("managed-home");
        let appdata = dir.join("appdata");
        let local_appdata = dir.join("local-appdata");
        fs::create_dir_all(&codex_home).unwrap();
        fs::create_dir_all(&managed_home).unwrap();
        fs::create_dir_all(appdata.join("Codex")).unwrap();
        fs::create_dir_all(local_appdata.join("OpenAI").join("Codex")).unwrap();
        fs::write(
            managed_home.join("auth.json"),
            serde_json::json!({ "account_id": "managed-account" }).to_string(),
        )
        .unwrap();
        fs::write(
            appdata.join("Codex").join("auth.json"),
            serde_json::json!({ "account_id": "appdata-account" }).to_string(),
        )
        .unwrap();
        fs::write(
            local_appdata.join("OpenAI").join("Codex").join("auth.json"),
            serde_json::json!({ "account_id": "local-appdata-account" }).to_string(),
        )
        .unwrap();

        let previous_home = env::var("CODEXPPP_TEST_CODEX_HOME").ok();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        let previous_appdata = env::var_os("APPDATA");
        let previous_local_appdata = env::var_os("LOCALAPPDATA");
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &codex_home);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed_home);
        env::set_var("APPDATA", &appdata);
        env::set_var("LOCALAPPDATA", &local_appdata);

        assert_eq!(codex_account_from_auth(), None);

        if let Some(value) = previous_home {
            env::set_var("CODEXPPP_TEST_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_HOME");
        }
        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        if let Some(value) = previous_appdata {
            env::set_var("APPDATA", value);
        } else {
            env::remove_var("APPDATA");
        }
        if let Some(value) = previous_local_appdata {
            env::set_var("LOCALAPPDATA", value);
        } else {
            env::remove_var("LOCALAPPDATA");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn codex_account_is_read_from_nested_auth_shapes() {
        let claims = serde_json::json!({
            "https://api.openai.com/auth": {
                "chatgpt_account_id": "account-from-nested-token"
            }
        });
        let token = format!(
            "header.{}.signature",
            URL_SAFE_NO_PAD.encode(claims.to_string().as_bytes())
        );
        let auth = serde_json::json!({
            "session": {
                "access_token": token
            },
            "profile": {
                "email": "nested-user@example.com"
            }
        });

        assert_eq!(
            codex_account_from_auth_value(&auth),
            Some("account-from-nested-token".to_string())
        );

        let auth = serde_json::json!({
            "session": {
                "profile": {
                    "email": "nested-user@example.com"
                }
            }
        });
        assert_eq!(
            codex_account_from_auth_value(&auth),
            Some("nested-user@example.com".to_string())
        );
    }

    #[test]
    fn launch_cmd_command_uses_windows_shell_without_token_argument() {
        let command = PathBuf::from(r"C:\Program Files\Codex\codex.cmd");
        let cmd = build_codex_launch_command(&CodexLaunchTarget::Command(command.clone())).unwrap();
        let program = cmd.get_program().to_string_lossy().to_string();
        let args = cmd
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();
        let command_line = args.join(" ");

        if cfg!(windows) {
            assert_eq!(program.to_ascii_lowercase(), "cmd");
            assert_eq!(args.first().map(String::as_str), Some("/C"));
            let start = args.get(1).expect("missing cmd start command");
            assert!(start.contains("start \"Codex\""));
            assert!(start.contains("\"C:\\Program Files\\Codex\\codex.cmd\""));
        } else {
            assert_eq!(program, command.to_string_lossy());
        }
        assert!(!command_line.contains("secret-token"));
        assert!(!cmd
            .get_envs()
            .any(|(key, _)| { key.to_string_lossy() == "CODEXPPP_SESSION_TOKEN" }));
        assert!(!cmd
            .get_envs()
            .any(|(key, _)| { key.to_string_lossy() == "CODEX_HOME" }));
    }

    #[test]
    fn launch_exe_command_uses_direct_process_without_shell_reparse() {
        let command = PathBuf::from(r"C:\Program Files\Codex\Codex.exe");
        let cmd = build_codex_launch_command(&CodexLaunchTarget::Command(command.clone())).unwrap();
        let program = cmd.get_program().to_string_lossy().to_string();
        let args = cmd
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();

        assert_eq!(program, command.to_string_lossy());
        assert!(
            args.is_empty(),
            "exe launch must not go through cmd/start shell parsing"
        );
    }

    #[test]
    fn managed_codex_home_is_never_injected_into_codex_launch() {
        let _guard = env_lock();
        let dir = env::temp_dir().join(format!("codexppp-launch-env-test-{}", now_unix_nanos()));
        let managed = dir.join("managed");
        fs::create_dir_all(&managed).unwrap();
        let previous_managed = env::var("CODEXPPP_TEST_MANAGED_CODEX_HOME").ok();
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);

        let target =
            CodexLaunchTarget::Command(dir.join(if cfg!(windows) { "codex.exe" } else { "codex" }));
        let mut cmd = build_codex_launch_command(&target).unwrap();
        apply_managed_codex_environment(&mut cmd, &target).unwrap();
        assert!(!cmd
            .get_envs()
            .any(|(key, _)| key.to_string_lossy() == "CODEX_HOME"));

        let mut app_id_cmd =
            build_codex_launch_command(&CodexLaunchTarget::AppUserModelId("Codex!App".to_string()))
                .unwrap();
        apply_managed_codex_environment(
            &mut app_id_cmd,
            &CodexLaunchTarget::AppUserModelId("Codex!App".to_string()),
        )
        .unwrap();
        assert!(!app_id_cmd
            .get_envs()
            .any(|(key, _)| key.to_string_lossy() == "CODEX_HOME"));

        if let Some(value) = previous_managed {
            env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_MANAGED_CODEX_HOME");
        }
        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn packaged_codex_command_maps_to_store_app_id_on_windows() {
        let command = PathBuf::from(
            r"C:\Program Files\WindowsApps\OpenAI.Codex_26.623.13972.0_x64__2p2nqsd0c76g0\app\Codex.exe",
        );
        let target = codex_launch_target_from_command(command.clone());
        if cfg!(windows) {
            let CodexLaunchTarget::AppUserModelId(app_id) = target else {
                panic!("Windows Store Codex package paths must launch by AppID");
            };
            assert_eq!(app_id, "OpenAI.Codex_2p2nqsd0c76g0!App");
        } else {
            let CodexLaunchTarget::Command(actual) = target else {
                panic!("non-Windows paths stay command targets");
            };
            assert_eq!(actual, command);
        }
    }

    #[test]
    fn codex_process_detection_matches_codex_without_matching_codexppp() {
        assert!(is_codex_process("codex.exe", ""));
        assert!(is_codex_process(
            "node.exe",
            r#"node C:\Users\1\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js"#
        ));
        assert!(is_codex_process(
            "cmd.exe",
            r#""C:\Users\1\AppData\Roaming\npm\codex.cmd""#
        ));
        assert!(!is_codex_process(
            "codexppp-desktop.exe",
            r#""D:\object\codex+++\desktop-client\src-tauri\target\release\codexppp-desktop.exe""#
        ));
        assert!(!is_codex_process(
            "powershell.exe",
            r#"powershell -File D:\object\codex+++\scripts\build-windows-client.ps1"#
        ));
        assert!(!is_codex_process(
            "powershell.exe",
            r#"powershell -Command rg codex desktop-client"#
        ));
    }

    #[test]
    fn codex_process_id_detection_matches_only_codex_processes() {
        let output = [
			r#"100	codex.exe	C:\Users\1\AppData\Roaming\npm\codex.cmd"#,
			r#"101	node.exe	node C:\Users\1\AppData\Roaming\npm\node_modules\@openai\codex\bin\codex.js"#,
			r#"102	codexppp-desktop.exe	D:\object\codex+++\desktop-client\codexppp-desktop.exe"#,
			r#"103	powershell.exe	powershell -Command rg codex desktop-client"#,
		]
		.join("\n");
        assert_eq!(codex_process_ids_from_output(&output), vec![100, 101]);
    }

    #[test]
    fn desktop_version_reports_package_version() {
        assert_eq!(desktop_version(), env!("CARGO_PKG_VERSION"));
    }

    #[test]
    fn update_download_url_allows_only_http_urls() {
        assert_eq!(
            validate_update_download_url(" https://example.com/codexppp.exe ").unwrap(),
            "https://example.com/codexppp.exe"
        );
        assert!(validate_update_download_url("file:///tmp/codexppp.exe").is_err());
        assert!(validate_update_download_url("https://example.com/\"bad\".exe").is_err());
        assert!(validate_update_download_url("").is_err());
    }

    #[test]
    fn update_download_command_uses_windows_shell_without_secret_arguments() {
        let result = build_update_download_command("https://example.com/codexppp.exe?token=public");
        if cfg!(windows) {
            let cmd = result.unwrap();
            let program = cmd.get_program().to_string_lossy().to_string();
            let args = cmd
                .get_args()
                .map(|arg| arg.to_string_lossy().to_string())
                .collect::<Vec<_>>();
            assert_eq!(program.to_ascii_lowercase(), "cmd");
            assert_eq!(args.first().map(String::as_str), Some("/C"));
            let start = args.get(1).expect("missing update start command");
            assert!(start.contains("start \"\""));
            assert!(start.contains("\"https://example.com/codexppp.exe?token=public\""));
            assert!(!start.contains("CODEXPPP_SESSION_TOKEN"));
        } else {
            assert_eq!(result.unwrap_err(), "update_download_unavailable");
        }
    }

    #[test]
    fn codex_install_command_uses_noninteractive_winget_msstore_install() {
        let winget = PathBuf::from(r"C:\Users\1\AppData\Local\Microsoft\WindowsApps\winget.exe");
        let cmd = build_codex_install_command_with_winget(winget.clone());
        let program = cmd.get_program().to_string_lossy().to_string();
        let args = cmd
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert_eq!(program, winget.to_string_lossy());
        assert_eq!(
            args,
            vec![
                "install",
                "--id",
                WINDOWS_STORE_CODEX_PRODUCT_ID,
                "--source",
                "msstore",
                "--exact",
                "--silent",
                "--disable-interactivity",
                "--accept-package-agreements",
                "--accept-source-agreements",
            ]
        );
        let command_line = args.join(" ");
        assert!(!command_line.contains("ms-windows-store://"));
        assert!(!command_line.contains("codex:"));
        assert!(!command_line.contains("CODEXPPP_SESSION_TOKEN"));
    }

    #[test]
    fn start_apps_output_detects_windows_store_codex_app_id() {
        assert_eq!(
            codex_app_user_model_id_from_start_apps_output(
                "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App\r\nOpenAI.Codex_2p2nqsd0c76g0!App\r\n"
            ),
            Some(WINDOWS_STORE_CODEX_APP_ID.to_string())
        );
        assert!(codex_app_user_model_id_from_start_apps_output(
            "Microsoft.WindowsCalculator_8wekyb3d8bbwe!App\r\n"
        )
        .is_none());
    }

    #[test]
    fn backend_api_base_is_deployment_configurable_without_user_ui() {
        assert_eq!(
            normalize_backend_api_base(" https://ops.example.com/ "),
            Some("https://ops.example.com/api".to_string())
        );
        assert_eq!(
            normalize_backend_api_base("https://ops.example.com/api/"),
            Some("https://ops.example.com/api".to_string())
        );
        assert_eq!(normalize_backend_api_base("  "), None);
    }

    #[test]
    fn desktop_client_does_not_start_a_local_network_service() {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let source = fs::read_to_string(manifest_dir.join("src").join("main.rs")).unwrap();
        let cargo_toml = fs::read_to_string(manifest_dir.join("Cargo.toml")).unwrap();
        let tauri_config = fs::read_to_string(manifest_dir.join("tauri.conf.json")).unwrap();

        let source_forbidden = [
            ["Tcp", "Listener"].concat(),
            ["Tcp", "Stream"].concat(),
            ["Udp", "Socket"].concat(),
            ["std", "::", "net"].concat(),
            [".", "bi", "nd("].concat(),
            ["lis", "ten("].concat(),
            ["tauri", "_", "plugin", "_", "localhost"].concat(),
        ];
        for pattern in source_forbidden {
            assert!(
                !source.contains(&pattern),
                "desktop client source must not start a local network service: {pattern}"
            );
        }

        for dependency in [
            "tauri-plugin-localhost",
            "tiny_http",
            "axum",
            "warp",
            "actix-web",
        ] {
            assert!(
                !cargo_toml.contains(dependency),
                "desktop client must not depend on local server package {dependency}"
            );
        }
        assert!(tauri_config.contains("\"frontendDist\": \"../ui\""));
        assert!(
            !tauri_config.contains("\"devUrl\""),
            "desktop client must ship static Tauri UI instead of a dev web server"
        );
    }

    #[test]
    fn desktop_ui_does_not_restore_stale_codex_account_text() {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let ui = fs::read_to_string(manifest_dir.join("..").join("ui").join("index.html")).unwrap();
        assert!(!ui.contains("previousAccount"));
    }

    #[test]
    fn windows_client_bundle_configuration_is_enabled() {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let source = fs::read_to_string(manifest_dir.join("src").join("main.rs")).unwrap();
        let tauri_config = fs::read_to_string(manifest_dir.join("tauri.conf.json")).unwrap();

        assert!(
            source.contains(r#"#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]"#),
            "Windows release client must use the GUI subsystem so opening the app does not show a console window"
        );
        for required in [
            "\"productName\": \"Codex+++\"",
            "\"identifier\": \"com.codexppp.desktop\"",
            "\"active\": true",
            "\"targets\": [\"nsis\"]",
            "\"icon\": [\"icons/icon.ico\"]",
            "\"publisher\": \"Codex+++\"",
        ] {
            assert!(
                tauri_config.contains(required),
                "Windows client release config missing {required}"
            );
        }
        assert!(
            !tauri_config.contains("\"active\": false"),
            "Windows client bundle must stay enabled"
        );
    }
}
