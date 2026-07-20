#![cfg_attr(all(windows, not(debug_assertions)), windows_subsystem = "windows")]

mod platform;

use base64::{
    engine::general_purpose::{URL_SAFE, URL_SAFE_NO_PAD},
    Engine as _,
};
use serde::{Deserialize, Serialize};
use serde_json::Value;
use sha2::{Digest, Sha256};
#[cfg(windows)]
use std::os::windows::process::CommandExt;
#[cfg(windows)]
use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::{Mutex, OnceLock};
use std::time::{SystemTime, UNIX_EPOCH};
use std::{
    env, fs,
    fs::File,
    io::{ErrorKind, Write},
    path::{Path, PathBuf},
    process::{self, Command, Stdio},
    thread,
    time::Duration,
};
#[cfg(windows)]
use sysinfo::System;
use tauri::{
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager, RunEvent, WindowEvent,
};
use toml_edit::{DocumentMut, Item, Table};

const DEFAULT_BACKEND_API_BASE: &str = "https://codex.52cx.top/api";
const DEFAULT_CODEX_LOCALE: &str = "zh-CN";
const CODEXPPP_PROVIDER_ID: &str = "codexppp";
const CODEXPPP_PROVIDER_TOKEN_FILE: &str = "codex-provider-token";
const CODEXPPP_PROVIDER_TOKEN_SCRIPT_FILE: &str = "codex-provider-token.ps1";
const WINDOWS_STORE_CODEX_APP_ID: &str = "OpenAI.Codex_2p2nqsd0c76g0!App";
const WINDOWS_STORE_CODEX_PACKAGE_FAMILY: &str = "OpenAI.Codex_2p2nqsd0c76g0";
const WINDOWS_STORE_CODEX_PRODUCT_ID: &str = "9PLM9XGG6VKS";
const CODEX_BROWSER_PLUGIN_ID: &str = "browser@openai-bundled";
const WINDOWS_STORE_CODEX_INSTALLER_URL: &str =
    "https://get.microsoft.com/installer/download/9PLM9XGG6VKS?cid=website_cta_psi";
const MAX_DESKTOP_UPDATE_BYTES: u64 = 256 * 1024 * 1024;
#[cfg(windows)]
const CREATE_NO_WINDOW: u32 = 0x08000000;
#[cfg(windows)]
static LAST_ACTIVATED_CODEX_PID: AtomicU32 = AtomicU32::new(0);
static VERIFIED_CODEX_STORE_VERSION: OnceLock<Mutex<Option<String>>> = OnceLock::new();

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct Diagnostics {
    codex_detected: String,
    codex_running: String,
    codex_version: String,
    codex_compatible: String,
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
    capability_warning: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct CodexUpdateResult {
    codex_version: String,
    latest_version: String,
    codex_compatible: String,
    update_available: bool,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct DeviceIdentity {
    device_name: String,
    fingerprint: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct BackendHttpResponse {
    status: u16,
    body: String,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct CodexUserStateManifest {
    config_existed: bool,
    auth_existed: bool,
}

enum CodexLaunchTarget {
    #[cfg(any(test, not(windows)))]
    Command(PathBuf),
    #[cfg(target_os = "macos")]
    MacOSApp(PathBuf),
    #[cfg(windows)]
    AppUserModelId(String),
}

#[tauri::command]
fn codex_diagnostics() -> Diagnostics {
    let target = find_codex_launch_target();
    let detected = target.is_some();
    let running = codex_process_running();
    let version = codex_version_from_launch_target(target.as_ref()).unwrap_or_default();
    Diagnostics {
        codex_detected: cn_available(detected),
        codex_running: cn_available(running),
        codex_compatible: codex_store_verification_status(detected, &version),
        codex_version: version,
        codex_account: codex_account_from_auth().unwrap_or_default(),
        codex_auth_mode: codex_auth_mode(),
        config_written: cn_available(codexppp_config_clean()),
        last_failure: String::new(),
    }
}

#[tauri::command]
fn check_codex_update() -> Result<CodexUpdateResult, String> {
    #[cfg(windows)]
    {
        return check_windows_codex_update();
    }
    #[cfg(target_os = "macos")]
    {
        let check = platform::macos::check_update()?;
        clear_codex_store_verification();
        if !check.update_available {
            mark_codex_store_version_verified(&check.installed_version)?;
        }
        return Ok(CodexUpdateResult {
            codex_version: check.installed_version,
            latest_version: String::new(),
            codex_compatible: if check.update_available {
                "不可用"
            } else {
                "可用"
            }
            .to_string(),
            update_available: check.update_available,
        });
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        Err("codex_version_check_unavailable".to_string())
    }
}

#[cfg(windows)]
fn check_windows_codex_update() -> Result<CodexUpdateResult, String> {
    if !codex_desktop_app_ready() {
        clear_codex_store_verification();
        return Err("codex_not_detected".to_string());
    }
    let installed_version = installed_codex_app_package_version().unwrap_or_default();
    if parse_codex_desktop_package_version(&installed_version).is_none() {
        clear_codex_store_verification();
        return Err("codex_version_unreadable".to_string());
    }
    clear_codex_store_verification();
    let winget =
        winget_command_path().ok_or_else(|| "codex_version_check_unavailable".to_string())?;
    let source_refresh = build_codex_store_source_refresh_command(winget.clone())
        .output()
        .map_err(|_| "codex_version_check_failed".to_string())?;
    if !source_refresh.status.success() {
        return Err("codex_version_check_failed".to_string());
    }
    let output = build_codex_update_check_command(winget)
        .output()
        .map_err(|_| "codex_version_check_failed".to_string())?;
    if !output.status.success() {
        return Err("codex_version_check_failed".to_string());
    }
    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    if stdout.trim().is_empty() && stderr.trim().is_empty() {
        return Err("codex_version_check_failed".to_string());
    }
    let (listed_version, available_version) =
        winget_codex_update_row(&stdout).ok_or_else(|| "codex_version_check_failed".to_string())?;
    if listed_version != installed_version {
        return Err("codex_version_check_failed".to_string());
    }
    let update_available = available_version.is_some();
    let latest_version = available_version.unwrap_or_default();
    if update_available {
        clear_codex_store_verification();
    } else {
        mark_codex_store_version_verified(&installed_version)?;
    }
    Ok(CodexUpdateResult {
        codex_version: installed_version,
        latest_version,
        codex_compatible: if update_available {
            "不可用"
        } else {
            "可用"
        }
        .to_string(),
        update_available,
    })
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
async fn backend_request(
    path: String,
    method: String,
    body: Option<String>,
    token: String,
) -> Result<BackendHttpResponse, String> {
    if !path.starts_with("/client/")
        || path.contains("..")
        || path.contains("\\")
        || path.chars().any(char::is_control)
    {
        return Err("backend_request_invalid".to_string());
    }
    if body.as_ref().is_some_and(|value| value.len() > 1_048_576) {
        return Err("backend_request_invalid".to_string());
    }
    let method = match method.trim().to_ascii_uppercase().as_str() {
        "GET" => reqwest::Method::GET,
        "POST" => reqwest::Method::POST,
        _ => return Err("backend_request_invalid".to_string()),
    };
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(30))
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "backend_request_invalid".to_string())?;
    let url = format!("{}{}", configured_backend_api_base(), path);
    let mut request = client
        .request(method, url)
        .header("Content-Type", "application/json")
        .header("X-CodexPPP-Interop-Major", "1")
        .header(
            "User-Agent",
            format!("CodexPPP-Desktop/{}", env!("CARGO_PKG_VERSION")),
        );
    if !token.trim().is_empty() {
        request = request.bearer_auth(token.trim());
    }
    if let Some(body) = body {
        request = request.body(body);
    }
    let response = request.send().await.map_err(|error| {
        if error.is_timeout() {
            "backend_timeout".to_string()
        } else if error.is_connect() {
            "backend_connect_failed".to_string()
        } else {
            "network_unavailable".to_string()
        }
    })?;
    let status = response.status().as_u16();
    let bytes = response
        .bytes()
        .await
        .map_err(|_| "backend_response_failed".to_string())?;
    if bytes.len() > 4 * 1024 * 1024 {
        return Err("backend_response_failed".to_string());
    }
    let body =
        String::from_utf8(bytes.to_vec()).map_err(|_| "backend_response_failed".to_string())?;
    Ok(BackendHttpResponse { status, body })
}

#[tauri::command]
fn desktop_version() -> String {
    env!("CARGO_PKG_VERSION").to_string()
}

#[tauri::command]
fn desktop_platform() -> String {
    if cfg!(target_os = "macos") {
        "macos"
    } else if cfg!(windows) {
        "windows"
    } else {
        "unsupported"
    }
    .to_string()
}

#[tauri::command]
async fn install_desktop_update(download_url: String, sha256: String) -> Result<(), String> {
    if !cfg!(any(windows, target_os = "macos")) {
        return Err("update_download_unavailable".to_string());
    }
    let url = validate_update_download_url(&download_url)?;
    let expected_sha256 = normalize_update_sha256(&sha256)?;
    let update_dir = env::temp_dir().join(format!(
        "codexppp-desktop-update-{}-{}",
        process::id(),
        now_unix_nanos()
    ));
    fs::create_dir_all(&update_dir).map_err(|_| "update_download_failed".to_string())?;
    let installer = update_dir.join(if cfg!(target_os = "macos") {
        "CodexPPP-update.dmg"
    } else {
        "CodexPPP-update.exe"
    });
    if let Err(err) = download_desktop_update(&url, &expected_sha256, &installer).await {
        let _ = fs::remove_dir_all(&update_dir);
        return Err(err);
    }
    if let Err(err) = schedule_desktop_update(&installer, &update_dir) {
        let _ = fs::remove_dir_all(&update_dir);
        return Err(err);
    }
    Ok(())
}

#[tauri::command]
fn exit_for_desktop_update(app: tauri::AppHandle) {
    app.exit(0);
}

#[tauri::command]
fn prepare_codex(backend_url: String, provider_token: String) -> Result<PrepareResult, String> {
    let target = find_codex_launch_target().ok_or_else(|| "codex_not_detected".to_string())?;
    let running = codex_process_running();
    if !running && !codex_store_version_verified() {
        return Err("codex_version_not_verified".to_string());
    }
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
fn launch_codex(app: tauri::AppHandle) -> Result<LaunchResult, String> {
    if codex_process_running() {
        hide_main_window(&app);
        return Ok(LaunchResult {
            launch_state: "运行中".to_string(),
            capability_warning: ensure_codex_browser_plugin_after_launch()
                .err()
                .unwrap_or_default(),
        });
    }
    if !codex_store_version_verified() {
        return Err("codex_version_not_verified".to_string());
    }
    let target = find_codex_launch_target().ok_or_else(|| "codex_not_detected".to_string())?;
    if let Err(err) = launch_codex_target(&target) {
        let _ = restore_codex_user_state();
        return Err(err);
    }
    hide_main_window(&app);
    Ok(LaunchResult {
        launch_state: "已启动".to_string(),
        capability_warning: ensure_codex_browser_plugin_after_launch()
            .err()
            .unwrap_or_default(),
    })
}

fn launch_codex_target(target: &CodexLaunchTarget) -> Result<(), String> {
    match target {
        #[cfg(windows)]
        CodexLaunchTarget::AppUserModelId(app_id) => {
            let pid = activate_windows_store_codex(app_id)?;
            LAST_ACTIVATED_CODEX_PID.store(pid, Ordering::Release);
            if wait_for_windows_codex_activation(pid) {
                Ok(())
            } else {
                LAST_ACTIVATED_CODEX_PID.store(0, Ordering::Release);
                Err("codex_activation_process_exited".to_string())
            }
        }
        #[cfg(all(test, windows))]
        CodexLaunchTarget::Command(command) => {
            let target = CodexLaunchTarget::Command(command.clone());
            let mut cmd = build_codex_launch_command(&target)?;
            apply_managed_codex_environment(&mut cmd, &target)?;
            cmd.spawn()
                .map(|_| ())
                .map_err(|_| "codex_command_launch_failed".to_string())
        }
        #[cfg(not(windows))]
        CodexLaunchTarget::Command(_) => {
            let mut cmd = build_codex_launch_command(target)?;
            apply_managed_codex_environment(&mut cmd, target)?;
            cmd.spawn()
                .map(|_| ())
                .map_err(|_| "codex_command_launch_failed".to_string())
        }
        #[cfg(target_os = "macos")]
        CodexLaunchTarget::MacOSApp(path) => platform::macos::launch(path),
    }
}

#[cfg(windows)]
fn activate_windows_store_codex(app_id: &str) -> Result<u32, String> {
    use windows::{
        core::PCWSTR,
        Win32::{
            Foundation::RPC_E_CHANGED_MODE,
            System::Com::{
                CoCreateInstance, CoInitializeEx, CoUninitialize, CLSCTX_LOCAL_SERVER,
                COINIT_APARTMENTTHREADED,
            },
            UI::Shell::{ApplicationActivationManager, IApplicationActivationManager, AO_NONE},
        },
    };

    if !app_id.eq_ignore_ascii_case(WINDOWS_STORE_CODEX_APP_ID) {
        return Err("codex_appid_invalid".to_string());
    }
    let wide_app_id = app_id
        .encode_utf16()
        .chain(std::iter::once(0))
        .collect::<Vec<_>>();
    unsafe {
        let initialized = CoInitializeEx(None, COINIT_APARTMENTTHREADED);
        if initialized.is_err() && initialized != RPC_E_CHANGED_MODE {
            return Err("codex_activation_com_init_failed".to_string());
        }
        let should_uninitialize = initialized.is_ok();
        let result = (|| {
            let manager: IApplicationActivationManager =
                CoCreateInstance(&ApplicationActivationManager, None, CLSCTX_LOCAL_SERVER)
                    .map_err(|_| "codex_activation_manager_failed".to_string())?;
            let pid = manager
                .ActivateApplication(PCWSTR(wide_app_id.as_ptr()), PCWSTR::null(), AO_NONE)
                .map_err(|_| "codex_activation_request_failed".to_string())?;
            (pid != 0)
                .then_some(pid)
                .ok_or_else(|| "codex_activation_no_process".to_string())
        })();
        if should_uninitialize {
            CoUninitialize();
        }
        result
    }
}

#[tauri::command]
fn stop_codex() -> Result<Diagnostics, String> {
    stop_codex_processes().map_err(|_| "codex_stop_failed".to_string())?;
    restore_codex_user_state().map_err(|_| "codex_restore_failed".to_string())?;
    Ok(codex_diagnostics())
}

#[tauri::command]
fn install_codex() -> Result<Diagnostics, String> {
    if codex_process_running() {
        return Err("codex_must_stop_before_update".to_string());
    }
    run_codex_install_command()?;
    ensure_codex_stopped_after_install()?;
    let version = installed_codex_app_package_version().unwrap_or_default();
    mark_codex_store_version_verified(&version)?;
    Ok(codex_diagnostics())
}

fn public_desktop_error(raw: &str) -> String {
    if raw.contains("解析失败") || raw.contains("parse") {
        return "codex_config_unreadable".to_string();
    }
    "codex_config_write_failed".to_string()
}

#[cfg(any(test, not(windows)))]
fn build_codex_launch_command(target: &CodexLaunchTarget) -> Result<Command, String> {
    let mut cmd = match target {
        #[cfg(windows)]
        CodexLaunchTarget::AppUserModelId(app_id) => {
            let mut shell = Command::new("explorer");
            shell.arg(format!(r"shell:AppsFolder\{app_id}"));
            shell
        }
        #[cfg(all(test, windows))]
        CodexLaunchTarget::Command(command) => build_windows_launch_command(command),
        #[cfg(not(windows))]
        CodexLaunchTarget::Command(command) => Command::new(command),
        #[cfg(target_os = "macos")]
        CodexLaunchTarget::MacOSApp(_) => {
            return Err("codex_command_launch_failed".to_string());
        }
    };
    cmd.stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    Ok(cmd)
}

#[cfg(all(test, windows))]
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

#[cfg(all(test, windows))]
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

#[cfg(all(test, windows))]
fn quote_windows_cmd_value(value: &str) -> String {
    format!("\"{}\"", value.replace('"', "\"\""))
}

async fn download_desktop_update(
    download_url: &str,
    expected_sha256: &str,
    installer: &Path,
) -> Result<(), String> {
    let client = reqwest::Client::builder()
        .connect_timeout(Duration::from_secs(20))
        .timeout(Duration::from_secs(300))
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "update_download_failed".to_string())?;
    let mut response = client
        .get(download_url)
        .header(
            "User-Agent",
            format!("CodexPPP-Desktop/{}", env!("CARGO_PKG_VERSION")),
        )
        .send()
        .await
        .map_err(|_| "update_download_failed".to_string())?;
    if !response.status().is_success()
        || response
            .content_length()
            .is_some_and(|length| length == 0 || length > MAX_DESKTOP_UPDATE_BYTES)
    {
        return Err("update_download_failed".to_string());
    }

    let mut file = File::create(installer).map_err(|_| "update_download_failed".to_string())?;
    let mut hasher = Sha256::new();
    let mut received = 0_u64;
    while let Some(chunk) = response
        .chunk()
        .await
        .map_err(|_| "update_download_failed".to_string())?
    {
        received = received
            .checked_add(chunk.len() as u64)
            .filter(|total| *total <= MAX_DESKTOP_UPDATE_BYTES)
            .ok_or_else(|| "update_download_failed".to_string())?;
        file.write_all(&chunk)
            .map_err(|_| "update_download_failed".to_string())?;
        hasher.update(&chunk);
    }
    if received == 0 {
        return Err("update_download_failed".to_string());
    }
    file.flush()
        .and_then(|_| file.sync_all())
        .map_err(|_| "update_download_failed".to_string())?;
    let actual_sha256 = format!("{:x}", hasher.finalize());
    if actual_sha256 != expected_sha256 {
        return Err("update_integrity_failed".to_string());
    }
    Ok(())
}

fn schedule_desktop_update(installer: &Path, update_dir: &Path) -> Result<(), String> {
    let mut helper = build_desktop_update_helper_command(installer, update_dir)?;
    helper
        .spawn()
        .map_err(|_| "update_install_failed".to_string())?;
    Ok(())
}

fn build_desktop_update_helper_command(
    installer: &Path,
    update_dir: &Path,
) -> Result<Command, String> {
    #[cfg(target_os = "macos")]
    {
        return platform::macos::desktop_update_helper(installer, update_dir, process::id());
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        let _ = (installer, update_dir);
        return Err("update_install_failed".to_string());
    }
    #[cfg(windows)]
    {
        let current_exe = env::current_exe().map_err(|_| "update_install_failed".to_string())?;
        let mut helper = Command::new("powershell");
        helper
            .args([
                "-NoProfile",
                "-NonInteractive",
                "-WindowStyle",
                "Hidden",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                DESKTOP_UPDATE_HELPER_SCRIPT,
            ])
            .env("CODEXPPP_UPDATE_PARENT_PID", process::id().to_string())
            .env("CODEXPPP_UPDATE_INSTALLER", installer)
            .env("CODEXPPP_UPDATE_CURRENT_EXE", current_exe)
            .env("CODEXPPP_UPDATE_TEMP_DIR", update_dir)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null());
        suppress_command_window(&mut helper);
        Ok(helper)
    }
}

const DESKTOP_UPDATE_HELPER_SCRIPT: &str = r#"
$ErrorActionPreference = 'SilentlyContinue'
$parentPid = [int]$env:CODEXPPP_UPDATE_PARENT_PID
$installer = $env:CODEXPPP_UPDATE_INSTALLER
$currentExe = $env:CODEXPPP_UPDATE_CURRENT_EXE
$tempDir = $env:CODEXPPP_UPDATE_TEMP_DIR
for ($i = 0; $i -lt 180; $i += 1) {
  if (-not (Get-Process -Id $parentPid -ErrorAction SilentlyContinue)) { break }
  Start-Sleep -Milliseconds 500
}
$exitCode = 1
if ((-not [string]::IsNullOrWhiteSpace($installer)) -and (Test-Path -LiteralPath $installer)) {
  $update = Start-Process -FilePath $installer -ArgumentList '/S' -Wait -PassThru
  if ($null -ne $update) { $exitCode = $update.ExitCode }
}
if ($exitCode -eq 0) {
  for ($i = 0; $i -lt 60; $i += 1) {
    if (Test-Path -LiteralPath $currentExe) { break }
    Start-Sleep -Milliseconds 500
  }
}
if ((-not [string]::IsNullOrWhiteSpace($currentExe)) -and (Test-Path -LiteralPath $currentExe)) {
  Start-Process -FilePath $currentExe | Out-Null
}
if ((-not [string]::IsNullOrWhiteSpace($tempDir)) -and (Test-Path -LiteralPath $tempDir)) {
  Remove-Item -LiteralPath $tempDir -Recurse -Force
}
"#;

fn build_codex_install_command(upgrade: bool) -> Result<Command, String> {
    if !cfg!(windows) {
        return Err("codex_install_unavailable".to_string());
    }
    let winget =
        winget_command_path().ok_or_else(|| "codex_install_component_missing".to_string())?;
    Ok(build_codex_install_command_with_winget(winget, upgrade))
}

fn build_codex_install_command_with_winget(winget: PathBuf, upgrade: bool) -> Command {
    let mut cmd = Command::new(winget);
    cmd.args([
        if upgrade { "upgrade" } else { "install" },
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

fn build_codex_update_check_command(winget: PathBuf) -> Command {
    let mut cmd = Command::new(winget);
    cmd.args([
        "list",
        "--id",
        WINDOWS_STORE_CODEX_PRODUCT_ID,
        "--exact",
        "--accept-source-agreements",
        "--disable-interactivity",
    ])
    .stdin(Stdio::null());
    suppress_command_window(&mut cmd);
    cmd
}

fn build_codex_store_source_refresh_command(winget: PathBuf) -> Command {
    let mut cmd = Command::new(winget);
    cmd.args([
        "source",
        "update",
        "--name",
        "msstore",
        "--disable-interactivity",
    ])
    .stdin(Stdio::null());
    suppress_command_window(&mut cmd);
    cmd
}

fn winget_codex_update_row(output: &str) -> Option<(String, Option<String>)> {
    output.lines().find_map(|line| {
        let fields = line.split_whitespace().collect::<Vec<_>>();
        let id_index = fields.iter().position(|field| {
            field.eq_ignore_ascii_case(WINDOWS_STORE_CODEX_PRODUCT_ID)
                || field.eq_ignore_ascii_case(WINDOWS_STORE_CODEX_PACKAGE_FAMILY)
        })?;
        let versions = fields[id_index + 1..]
            .iter()
            .map(|field| field.trim())
            .filter(|field| parse_codex_desktop_package_version(field).is_some())
            .take(2)
            .map(str::to_string)
            .collect::<Vec<_>>();
        let installed = versions.first()?.to_string();
        let latest = versions.get(1).cloned();
        Some((installed, latest))
    })
}

fn run_codex_install_command() -> Result<(), String> {
    #[cfg(windows)]
    {
        let installed = codex_desktop_app_ready();
        if let Ok(mut winget) = build_codex_install_command(installed) {
            if winget
                .status()
                .map(|status| status.success())
                .unwrap_or(false)
                && wait_for_codex_desktop_ready()
            {
                return Ok(());
            }
        }
        let mut web_installer = build_codex_store_web_installer_command()?;
        let installed = web_installer
            .status()
            .map(|status| status.success())
            .unwrap_or(false);
        if installed && wait_for_codex_desktop_ready() {
            return Ok(());
        }
        if codex_desktop_app_ready() {
            return Err("codex_update_failed".to_string());
        }
        return Err("codex_install_failed".to_string());
    }
    #[cfg(target_os = "macos")]
    {
        platform::macos::install_or_update()
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        Err("codex_install_unavailable".to_string())
    }
}

fn ensure_codex_stopped_after_install() -> Result<(), String> {
    observe_codex_install_quiet(
        codex_process_running,
        stop_codex_processes,
        || thread::sleep(Duration::from_millis(250)),
        40,
        8,
    )
}

fn observe_codex_install_quiet<Running, Stop, Wait>(
    mut running: Running,
    mut stop: Stop,
    mut wait: Wait,
    max_checks: usize,
    required_quiet_checks: usize,
) -> Result<(), String>
where
    Running: FnMut() -> bool,
    Stop: FnMut() -> Result<(), String>,
    Wait: FnMut(),
{
    let mut quiet_checks = 0;
    for _ in 0..max_checks {
        if running() {
            quiet_checks = 0;
            stop().map_err(|_| "codex_install_autolaunch_stop_failed".to_string())?;
        } else {
            quiet_checks += 1;
            if quiet_checks >= required_quiet_checks.max(1) {
                return Ok(());
            }
        }
        wait();
    }
    Err("codex_install_autolaunch_stop_failed".to_string())
}

fn ensure_codex_browser_plugin() -> Result<(), String> {
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_TEST_BROWSER_PLUGIN_ENABLED") {
        return if value.trim() == "1" {
            Ok(())
        } else {
            Err("codex_browser_plugin_install_failed".to_string())
        };
    }
    let command =
        codex_plugin_command().ok_or_else(|| "codex_browser_plugin_install_failed".to_string())?;
    if codex_browser_plugin_enabled(&command) {
        return Ok(());
    }
    let mut install = build_codex_browser_plugin_install_command(&command);
    let installed = install
        .status()
        .map(|status| status.success())
        .unwrap_or(false);
    if !installed || !codex_browser_plugin_enabled(&command) {
        return Err("codex_browser_plugin_install_failed".to_string());
    }
    Ok(())
}

fn ensure_codex_browser_plugin_after_launch() -> Result<(), String> {
    #[cfg(test)]
    return ensure_codex_browser_plugin();

    #[cfg(not(test))]
    {
        // The Store package's protected resources cannot be executed directly by
        // the launcher. The official desktop app publishes its CLI runtime under
        // LocalAppData on first launch, so wait briefly for that bootstrap before
        // configuring the bundled Browser plugin.
        for _ in 0..20 {
            if codex_plugin_command().is_some() {
                return ensure_codex_browser_plugin();
            }
            if !codex_process_running() {
                break;
            }
            thread::sleep(Duration::from_millis(500));
        }
        Err("codex_browser_plugin_install_failed".to_string())
    }
}

fn codex_plugin_command() -> Option<PathBuf> {
    #[cfg(test)]
    if let Ok(value) = env::var("CODEXPPP_CODEX_COMMAND") {
        if !value.trim().is_empty() {
            return resolve_command_value(value.trim());
        }
    }
    #[cfg(windows)]
    {
        let local_app_data = local_app_data_root().ok()?;
        official_codex_runtime_command(&local_app_data)
    }
    #[cfg(not(windows))]
    {
        find_command_on_path("codex").or_else(find_common_codex_command)
    }
}

#[cfg(windows)]
fn official_codex_runtime_command(local_app_data: &Path) -> Option<PathBuf> {
    let bin = local_app_data.join("OpenAI").join("Codex").join("bin");
    let direct = bin.join("codex.exe");
    if direct.is_file() {
        return Some(direct);
    }

    let mut commands = fs::read_dir(bin)
        .ok()?
        .filter_map(Result::ok)
        .filter_map(|entry| {
            let command = entry.path().join("codex.exe");
            command.is_file().then_some(command)
        })
        .collect::<Vec<_>>();
    commands.sort_by(|a, b| {
        let a_modified = fs::metadata(a)
            .and_then(|metadata| metadata.modified())
            .ok();
        let b_modified = fs::metadata(b)
            .and_then(|metadata| metadata.modified())
            .ok();
        b_modified
            .cmp(&a_modified)
            .then_with(|| b.as_os_str().cmp(a.as_os_str()))
    });
    commands.into_iter().next()
}

fn codex_browser_plugin_enabled(command: &Path) -> bool {
    let mut list = build_codex_plugin_list_command(command);
    let Ok(output) = list.output() else {
        return false;
    };
    output.status.success() && codex_browser_plugin_enabled_from_json(&output.stdout)
}

fn codex_browser_plugin_enabled_from_json(raw: &[u8]) -> bool {
    let Ok(payload) = serde_json::from_slice::<Value>(raw) else {
        return false;
    };
    payload
        .get("installed")
        .and_then(Value::as_array)
        .is_some_and(|plugins| {
            plugins.iter().any(|plugin| {
                plugin.get("pluginId").and_then(Value::as_str) == Some(CODEX_BROWSER_PLUGIN_ID)
                    && plugin.get("installed").and_then(Value::as_bool) == Some(true)
                    && plugin.get("enabled").and_then(Value::as_bool) == Some(true)
            })
        })
}

fn build_codex_plugin_list_command(command: &Path) -> Command {
    let mut cmd = Command::new(command);
    cmd.args(["plugin", "list", "--json"])
        .stdin(Stdio::null())
        .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    cmd
}

fn build_codex_browser_plugin_install_command(command: &Path) -> Command {
    let mut cmd = Command::new(command);
    cmd.args(["plugin", "add", CODEX_BROWSER_PLUGIN_ID, "--json"])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    cmd
}

fn codex_desktop_app_ready() -> bool {
    #[cfg(windows)]
    {
        installed_codex_app_user_model_id().is_some()
    }
    #[cfg(target_os = "macos")]
    {
        platform::macos::app_ready()
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        false
    }
}

fn wait_for_codex_desktop_ready() -> bool {
    for _ in 0..45 {
        if codex_desktop_app_ready()
            && codex_desktop_version_readable(
                &installed_codex_app_package_version().unwrap_or_default(),
            )
        {
            return true;
        }
        thread::sleep(Duration::from_secs(2));
    }
    false
}

fn build_codex_store_web_installer_command() -> Result<Command, String> {
    if !cfg!(windows) {
        return Err("codex_install_unavailable".to_string());
    }
    let mut cmd = Command::new("powershell");
    cmd.args([
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-Command",
        CODEX_STORE_WEB_INSTALL_SCRIPT,
    ])
    .env(
        "CODEXPPP_CODEX_DESKTOP_INSTALLER_URL",
        WINDOWS_STORE_CODEX_INSTALLER_URL,
    )
    .stdin(Stdio::null())
    .stdout(Stdio::null())
    .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    Ok(cmd)
}

const CODEX_STORE_WEB_INSTALL_SCRIPT: &str = r#"
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$url = $env:CODEXPPP_CODEX_DESKTOP_INSTALLER_URL
if ([string]::IsNullOrWhiteSpace($url) -or -not $url.StartsWith('https://get.microsoft.com/')) {
  throw 'invalid Codex desktop installer URL'
}
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) ('codex-desktop-install-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null
try {
  $installer = Join-Path $tempDir 'ChatGPT Installer.exe'
  Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $installer
  $signature = Get-AuthenticodeSignature -LiteralPath $installer
  if ($signature.Status -ne 'Valid') { throw 'invalid Microsoft Store installer signature' }
  $subject = [string]$signature.SignerCertificate.Subject
  if ($subject -notmatch '(?i)(^|,\s*)O=Microsoft Corporation(,|$)') {
    throw 'unexpected Microsoft Store installer signer'
  }
  $process = Start-Process -FilePath $installer -ArgumentList '--silent' -Wait -PassThru
  if ($process.ExitCode -ne 0) { throw "Microsoft Store installer failed: $($process.ExitCode)" }
} finally {
  Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
}
"#;

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
    let url = reqwest::Url::parse(value).map_err(|_| "update_download_unavailable".to_string())?;
    if value.is_empty()
        || url.scheme() != "https"
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.fragment().is_some()
        || !desktop_update_path_supported(url.path())
        || value
            .chars()
            .any(|ch| ch.is_control() || ch == '"' || ch == '\'')
    {
        return Err("update_download_unavailable".to_string());
    }
    Ok(url.to_string())
}

fn desktop_update_path_supported(path: &str) -> bool {
    let path = path.to_ascii_lowercase();
    if cfg!(target_os = "macos") {
        path.ends_with(".dmg")
    } else if cfg!(windows) {
        path.ends_with(".exe")
    } else {
        false
    }
}

fn normalize_update_sha256(raw: &str) -> Result<String, String> {
    let value = raw.trim().to_ascii_lowercase();
    if value.len() != 64 || !value.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err("update_metadata_invalid".to_string());
    }
    Ok(value)
}

fn main() {
    if !codex_process_running() {
        let _ = restore_codex_user_state();
    }
    let app = tauri::Builder::default()
        .setup(|app| {
            let open = MenuItem::with_id(app, "open", "打开客户端", true, None::<&str>)?;
            let quit = MenuItem::with_id(app, "quit", "停止 Codex 并退出", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&open, &quit])?;
            let mut tray = TrayIconBuilder::new()
                .tooltip("Codex+++ 正在保持 Codex 受控运行")
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "open" => show_main_window(app),
                    "quit" => {
                        if let Some(window) = app.get_webview_window("main") {
                            if window
                                .eval("window.__codexpppStopAndExitFromTray && window.__codexpppStopAndExitFromTray();")
                                .is_ok()
                            {
                                let fallback_app = app.clone();
                                thread::spawn(move || {
                                    thread::sleep(Duration::from_secs(15));
                                    cleanup_managed_codex_session();
                                    fallback_app.exit(0);
                                });
                                return;
                            }
                        }
                        cleanup_managed_codex_session();
                        app.exit(0);
                    }
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        show_main_window(tray.app_handle());
                    }
                });
            if let Some(icon) = app.default_window_icon() {
                tray = tray.icon(icon.clone());
            }
            tray.build(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .invoke_handler(tauri::generate_handler![
            codex_diagnostics,
            device_identity,
            backend_api_base,
            backend_request,
            desktop_version,
            desktop_platform,
            install_desktop_update,
            exit_for_desktop_update,
            check_codex_update,
            install_codex,
            prepare_codex,
            launch_codex,
            stop_codex,
            exit_managed_client
        ])
        .build(tauri::generate_context!())
        .expect("failed to build Codex+++ desktop client");
    app.run(|_, event| {
        if matches!(event, RunEvent::Exit) {
            cleanup_managed_codex_session();
        }
    });
}

fn show_main_window(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}

fn hide_main_window(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.hide();
    }
}

fn cleanup_managed_codex_session() {
    if !managed_codex_session_active() {
        return;
    }
    let _ = stop_codex_processes();
    let _ = restore_codex_user_state();
}

fn managed_codex_session_active() -> bool {
    codexppp_backup_root()
        .map(|root| root.join("active-session").join("manifest.json").is_file())
        .unwrap_or(false)
}

#[tauri::command]
fn exit_managed_client(app: tauri::AppHandle) {
    cleanup_managed_codex_session();
    app.exit(0);
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
    if provider_token.trim().is_empty() {
        return Err("login_failed".into());
    }
    capture_codex_user_state(&home)?;
    if let Err(err) = write_codexppp_api_login(&home, backend_url, provider_token) {
        let _ = restore_codex_user_state_at(&home);
        return Err(err);
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
    ensure_default_codex_locale(&mut doc);
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

fn ensure_default_codex_locale(doc: &mut DocumentMut) {
    if doc.get("desktop").is_none() {
        doc["desktop"] = Item::Table(Table::new());
    }
    let desktop = doc
        .get_mut("desktop")
        .expect("desktop config was created above");
    if let Some(table) = desktop.as_table_mut() {
        let has_explicit_locale = table
            .get("localeOverride")
            .and_then(Item::as_str)
            .is_some_and(|locale| !locale.trim().is_empty());
        if !has_explicit_locale {
            table["localeOverride"] = toml_edit::value(DEFAULT_CODEX_LOCALE);
        }
        return;
    }
    if let Some(table) = desktop.as_inline_table_mut() {
        let has_explicit_locale = table
            .get("localeOverride")
            .and_then(toml_edit::Value::as_str)
            .is_some_and(|locale| !locale.trim().is_empty());
        if !has_explicit_locale {
            table.insert(
                "localeOverride",
                toml_edit::Value::from(DEFAULT_CODEX_LOCALE),
            );
        }
        return;
    }
    *desktop = Item::Table(Table::new());
    desktop["localeOverride"] = toml_edit::value(DEFAULT_CODEX_LOCALE);
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

#[cfg(test)]
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

fn capture_codex_user_state(home: &Path) -> Result<(), Box<dyn std::error::Error>> {
    let backup_root = codexppp_backup_root()?;
    let active = backup_root.join("active-session");
    let manifest_path = active.join("manifest.json");
    if manifest_path.is_file() {
        return Ok(());
    }
    fs::create_dir_all(&backup_root)?;
    let staging = backup_root.join(format!("active-session-{}", now_unix_nanos()));
    fs::create_dir_all(&staging)?;
    let config_path = home.join("config.toml");
    let auth_path = home.join("auth.json");
    let manifest = CodexUserStateManifest {
        config_existed: config_path.is_file(),
        auth_existed: auth_path.is_file(),
    };
    if manifest.config_existed {
        fs::copy(&config_path, staging.join("config.toml"))?;
    }
    if manifest.auth_existed {
        fs::copy(&auth_path, staging.join("auth.json"))?;
    }
    fs::write(
        staging.join("manifest.json"),
        format!("{}\n", serde_json::to_string_pretty(&manifest)?),
    )?;
    match fs::rename(&staging, &active) {
        Ok(()) => Ok(()),
        Err(_err) if manifest_path.is_file() => {
            let _ = fs::remove_dir_all(staging);
            Ok(())
        }
        Err(err) => {
            let _ = fs::remove_dir_all(staging);
            Err(Box::new(err))
        }
    }
}

fn restore_codex_user_state() -> Result<(), Box<dyn std::error::Error>> {
    let home = codex_home()?;
    restore_codex_user_state_at(&home)
}

fn restore_codex_user_state_at(home: &Path) -> Result<(), Box<dyn std::error::Error>> {
    let active = codexppp_backup_root()?.join("active-session");
    let manifest_path = active.join("manifest.json");
    let manifest: CodexUserStateManifest = match fs::read_to_string(&manifest_path) {
        Ok(content) => serde_json::from_str(&content)?,
        Err(err) if err.kind() == ErrorKind::NotFound => return Ok(()),
        Err(err) => return Err(Box::new(err)),
    };
    fs::create_dir_all(home)?;
    restore_codex_user_file(
        &active.join("config.toml"),
        &home.join("config.toml"),
        manifest.config_existed,
    )?;
    restore_codex_user_file(
        &active.join("auth.json"),
        &home.join("auth.json"),
        manifest.auth_existed,
    )?;
    fs::remove_dir_all(active)?;
    Ok(())
}

fn restore_codex_user_file(
    backup: &Path,
    target: &Path,
    existed: bool,
) -> Result<(), Box<dyn std::error::Error>> {
    if existed {
        fs::copy(backup, target)?;
        return Ok(());
    }
    match fs::remove_file(target) {
        Ok(()) => Ok(()),
        Err(err) if err.kind() == ErrorKind::NotFound => Ok(()),
        Err(err) => Err(Box::new(err)),
    }
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
    let managed_clean = managed_codex_home().ok().is_none_or(|home| !home.exists());
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

#[cfg(test)]
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

#[cfg(test)]
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

#[cfg(any(test, not(windows)))]
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
        let mode = codex_auth_mode_from_value(&auth);
        if !mode.is_empty() {
            return mode;
        }
    }
    String::new()
}

fn codex_auth_mode_from_value(auth: &Value) -> String {
    if auth_value_has_chatgpt_login(auth) {
        return "chatgpt".to_string();
    }
    if codex_auth_api_key(auth).is_some() {
        return "api_key".to_string();
    }
    String::new()
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
    Ok(local_app_data_root()?.join("Codex+++").join("codex-home"))
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
    #[cfg(windows)]
    {
        let local_app_data = env::var_os("LOCALAPPDATA").ok_or("LOCALAPPDATA is not set")?;
        return Ok(PathBuf::from(local_app_data));
    }
    #[cfg(target_os = "macos")]
    {
        return platform::macos::application_support_root();
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        let home = env::var_os("HOME").ok_or("HOME is not set")?;
        Ok(PathBuf::from(home).join(".local").join("share"))
    }
}

fn find_codex_launch_target() -> Option<CodexLaunchTarget> {
    #[cfg(all(test, windows))]
    if let Ok(value) = env::var("CODEXPPP_CODEX_COMMAND") {
        if !value.trim().is_empty() {
            return resolve_command_value(value.trim()).map(codex_launch_target_from_command);
        }
    }
    #[cfg(windows)]
    {
        installed_codex_app_user_model_id().map(CodexLaunchTarget::AppUserModelId)
    }
    #[cfg(target_os = "macos")]
    {
        platform::macos::installed_app().map(CodexLaunchTarget::MacOSApp)
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        find_command_on_path("codex")
            .or_else(find_common_codex_command)
            .map(codex_launch_target_from_command)
    }
}

#[cfg(test)]
fn find_codex_command() -> Option<PathBuf> {
    match find_codex_launch_target()? {
        CodexLaunchTarget::Command(command) => Some(command),
        #[cfg(target_os = "macos")]
        CodexLaunchTarget::MacOSApp(_) => None,
        #[cfg(windows)]
        CodexLaunchTarget::AppUserModelId(_) => None,
    }
}

#[cfg(any(test, not(windows)))]
fn resolve_command_value(value: &str) -> Option<PathBuf> {
    let candidate = PathBuf::from(value);
    if command_value_has_directory(&candidate) {
        return existing_command_path(candidate);
    }
    find_command_on_path(value)
}

#[cfg(any(test, not(windows)))]
fn command_value_has_directory(path: &Path) -> bool {
    path.is_absolute()
        || path
            .parent()
            .map(|parent| !parent.as_os_str().is_empty())
            .unwrap_or(false)
}

#[cfg(any(test, not(windows)))]
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

#[cfg(any(test, not(windows)))]
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

#[cfg(all(test, windows))]
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
    let command = package_root.join("app").join("ChatGPT.exe");
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

fn codex_version_from_launch_target(target: Option<&CodexLaunchTarget>) -> Option<String> {
    match target? {
        #[cfg(any(test, not(windows)))]
        CodexLaunchTarget::Command(command) => codex_version_from_command(command),
        #[cfg(windows)]
        CodexLaunchTarget::AppUserModelId(_) => installed_codex_app_package_version(),
        #[cfg(target_os = "macos")]
        CodexLaunchTarget::MacOSApp(path) => platform::macos::app_version(path),
    }
}

#[cfg(any(test, not(windows)))]
fn codex_version_from_command(command: &Path) -> Option<String> {
    let mut cmd = Command::new(command);
    cmd.arg("--version")
        .stdin(Stdio::null())
        .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    let output = cmd.output().ok()?;
    if !output.status.success() {
        return None;
    }
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .map(sanitize_codex_version_line)
        .find(|line| !line.is_empty())
}

fn sanitize_codex_version_line(line: &str) -> String {
    line.chars()
        .filter(|ch| !ch.is_control())
        .collect::<String>()
        .trim()
        .chars()
        .take(96)
        .collect()
}

#[cfg(windows)]
fn installed_codex_app_package_version() -> Option<String> {
    let mut cmd = Command::new("powershell");
    cmd.args([
        "-NoProfile",
        "-NonInteractive",
        "-ExecutionPolicy",
        "Bypass",
        "-Command",
        "Get-AppxPackage -Name OpenAI.Codex | Sort-Object Version -Descending | Select-Object -First 1 -ExpandProperty Version",
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
        .map(sanitize_codex_version_line)
        .find(|line| !line.is_empty())
}

#[cfg(target_os = "macos")]
fn installed_codex_app_package_version() -> Option<String> {
    platform::macos::installed_app()
        .as_deref()
        .and_then(platform::macos::app_version)
}

#[cfg(not(any(windows, target_os = "macos")))]
fn installed_codex_app_package_version() -> Option<String> {
    None
}

fn codex_store_verification() -> &'static Mutex<Option<String>> {
    VERIFIED_CODEX_STORE_VERSION.get_or_init(|| Mutex::new(None))
}

fn clear_codex_store_verification() {
    if let Ok(mut verified) = codex_store_verification().lock() {
        *verified = None;
    }
}

fn mark_codex_store_version_verified(version: &str) -> Result<(), String> {
    let version = sanitize_codex_version_line(version);
    if !codex_desktop_version_readable(&version) {
        clear_codex_store_verification();
        return Err("codex_version_unreadable".to_string());
    }
    let mut verified = codex_store_verification()
        .lock()
        .map_err(|_| "codex_version_check_failed".to_string())?;
    *verified = Some(version);
    Ok(())
}

fn codex_store_version_verified() -> bool {
    #[cfg(test)]
    if env::var("CODEXPPP_TEST_CODEX_VERSION_VERIFIED")
        .ok()
        .as_deref()
        == Some("1")
    {
        return true;
    }

    let installed = installed_codex_app_package_version().unwrap_or_default();
    codex_store_verification()
        .lock()
        .ok()
        .and_then(|verified| verified.as_ref().cloned())
        .is_some_and(|verified| verified == installed)
}

fn codex_store_verification_status(detected: bool, version: &str) -> String {
    if !detected || !codex_desktop_version_readable(version) {
        return "不可用".to_string();
    }
    if codex_store_version_verified() {
        "可用".to_string()
    } else {
        "未检查".to_string()
    }
}

fn codex_desktop_version_readable(version: &str) -> bool {
    #[cfg(windows)]
    {
        return parse_codex_desktop_package_version(version).is_some();
    }
    #[cfg(target_os = "macos")]
    {
        return platform::macos::version_is_readable(version);
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        !version.trim().is_empty()
    }
}

fn parse_codex_desktop_package_version(version: &str) -> Option<[u64; 4]> {
    let parts = version
        .trim()
        .split('.')
        .map(|part| part.parse::<u64>().ok())
        .collect::<Option<Vec<_>>>()?;
    parts.try_into().ok()
}

#[cfg(target_os = "macos")]
fn find_common_codex_command() -> Option<PathBuf> {
    platform::macos::common_codex_command()
}

#[cfg(not(any(windows, target_os = "macos")))]
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
    #[cfg(windows)]
    LAST_ACTIVATED_CODEX_PID.store(0, Ordering::Release);
    Ok(())
}

#[cfg(windows)]
fn codex_process_running_from_system() -> bool {
    let activated_pid = LAST_ACTIVATED_CODEX_PID.load(Ordering::Acquire);
    if activated_pid != 0 {
        if windows_process_is_running(activated_pid) {
            return true;
        }
        LAST_ACTIVATED_CODEX_PID.store(0, Ordering::Release);
    }
    if !windows_store_codex_process_ids_from_tasklist_apps().is_empty() {
        return true;
    }
    // A packaged app can still be launched through its AppUserModelId when the
    // current user cannot traverse the protected WindowsApps install directory.
    // Process/package identity must therefore remain usable without a readable
    // executable path.
    let app_directory = installed_codex_desktop_app_command()
        .and_then(|command| command.parent().map(Path::to_path_buf));
    codex_process_snapshots()
        .iter()
        .any(|snapshot| is_codex_desktop_process(snapshot, app_directory.as_deref()))
}

#[cfg(windows)]
fn codex_process_ids_from_system() -> Vec<u32> {
    let mut ids = windows_store_codex_process_ids_from_tasklist_apps();
    let activated_pid = LAST_ACTIVATED_CODEX_PID.load(Ordering::Acquire);
    if activated_pid != 0
        && windows_process_is_running(activated_pid)
        && !ids.contains(&activated_pid)
    {
        ids.push(activated_pid);
    }
    let app_directory = installed_codex_desktop_app_command()
        .and_then(|command| command.parent().map(Path::to_path_buf));
    for pid in codex_process_snapshots()
        .into_iter()
        .filter(|snapshot| is_codex_desktop_process(snapshot, app_directory.as_deref()))
        .map(|snapshot| snapshot.pid)
    {
        if !ids.contains(&pid) {
            ids.push(pid);
        }
    }
    ids
}

#[cfg(windows)]
fn windows_process_is_running(pid: u32) -> bool {
    const PROCESS_QUERY_LIMITED_INFORMATION: u32 = 0x1000;
    const STILL_ACTIVE: u32 = 259;

    #[link(name = "kernel32")]
    extern "system" {
        fn OpenProcess(
            desired_access: u32,
            inherit_handle: i32,
            process_id: u32,
        ) -> *mut std::ffi::c_void;
        fn GetExitCodeProcess(process: *mut std::ffi::c_void, exit_code: *mut u32) -> i32;
        fn CloseHandle(handle: *mut std::ffi::c_void) -> i32;
    }

    unsafe {
        let process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, 0, pid);
        if process.is_null() {
            return false;
        }
        let mut exit_code = 0_u32;
        let ok = GetExitCodeProcess(process, &mut exit_code) != 0;
        let _ = CloseHandle(process);
        ok && exit_code == STILL_ACTIVE
    }
}

#[cfg(windows)]
fn wait_for_windows_codex_activation(pid: u32) -> bool {
    for _ in 0..12 {
        if windows_process_is_running(pid)
            || windows_store_codex_process_ids_from_tasklist_apps().contains(&pid)
        {
            return true;
        }
        thread::sleep(Duration::from_millis(250));
    }
    false
}

#[cfg(windows)]
struct ProcessSnapshot {
    pid: u32,
    name: String,
    executable_path: PathBuf,
}

#[cfg(windows)]
fn codex_process_snapshots() -> Vec<ProcessSnapshot> {
    let mut system = System::new();
    system.refresh_processes();
    let mut snapshots = system
        .processes()
        .iter()
        .map(|(pid, process)| ProcessSnapshot {
            pid: pid.as_u32(),
            name: process.name().to_string(),
            executable_path: process.exe().map(Path::to_path_buf).unwrap_or_default(),
        })
        .collect::<Vec<_>>();

    // Some Windows/AppContainer combinations are absent from sysinfo's process
    // view. tasklist is an independent OS-provided enumeration path; package
    // family verification below still prevents an unrelated codex.exe from
    // being treated as the Store desktop app.
    for snapshot in windows_tasklist_process_snapshots() {
        if !snapshots
            .iter()
            .any(|existing| existing.pid == snapshot.pid)
        {
            snapshots.push(snapshot);
        }
    }
    snapshots
}

#[cfg(windows)]
fn windows_tasklist_process_snapshots() -> Vec<ProcessSnapshot> {
    let mut cmd = Command::new("tasklist.exe");
    cmd.args(["/FO", "CSV", "/NH"])
        .stdin(Stdio::null())
        .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    let Ok(output) = cmd.output() else {
        return Vec::new();
    };
    if !output.status.success() {
        return Vec::new();
    }
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .filter_map(process_snapshot_from_tasklist_line)
        .collect()
}

#[cfg(windows)]
fn windows_store_codex_process_ids_from_tasklist_apps() -> Vec<u32> {
    let mut cmd = Command::new("tasklist.exe");
    cmd.args(["/APPS", "/FO", "CSV", "/NH"])
        .stdin(Stdio::null())
        .stderr(Stdio::null());
    suppress_command_window(&mut cmd);
    let Ok(output) = cmd.output() else {
        return Vec::new();
    };
    if !output.status.success() {
        return Vec::new();
    }
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .filter_map(windows_store_codex_process_id_from_tasklist_app_line)
        .collect()
}

#[cfg(windows)]
fn windows_store_codex_process_id_from_tasklist_app_line(line: &str) -> Option<u32> {
    let fields = windows_csv_fields(line.trim().trim_start_matches('\u{feff}'));
    let image_name = fields.first()?.trim().to_ascii_lowercase();
    let package_full_name = fields.get(3)?.trim().to_ascii_lowercase();
    let supported_process = image_name.starts_with("chatgpt.exe (")
        || image_name == "chatgpt.exe"
        || image_name.starts_with("codex.exe (")
        || image_name == "codex.exe";
    let official_package = package_full_name.starts_with("openai.codex_")
        && package_full_name.ends_with("__2p2nqsd0c76g0");
    if !supported_process || !official_package {
        return None;
    }
    fields
        .get(1)?
        .trim()
        .parse::<u32>()
        .ok()
        .filter(|pid| *pid != 0)
}

#[cfg(windows)]
fn process_snapshot_from_tasklist_line(line: &str) -> Option<ProcessSnapshot> {
    let fields = windows_csv_fields(line.trim().trim_start_matches('\u{feff}'));
    let name = fields.first()?.trim().to_string();
    let pid = fields.get(1)?.trim().parse::<u32>().ok()?;
    if name.is_empty() || pid == 0 {
        return None;
    }
    Some(ProcessSnapshot {
        pid,
        name,
        executable_path: PathBuf::new(),
    })
}

#[cfg(windows)]
fn windows_csv_fields(line: &str) -> Vec<String> {
    let mut fields = Vec::new();
    let mut field = String::new();
    let mut chars = line.chars().peekable();
    let mut quoted = false;
    while let Some(ch) = chars.next() {
        match ch {
            '"' if quoted && chars.peek() == Some(&'"') => {
                field.push('"');
                chars.next();
            }
            '"' => quoted = !quoted,
            ',' if !quoted => {
                fields.push(field);
                field = String::new();
            }
            _ => field.push(ch),
        }
    }
    fields.push(field);
    fields
}

#[cfg(windows)]
fn is_codex_desktop_process(snapshot: &ProcessSnapshot, app_directory: Option<&Path>) -> bool {
    let process_name = snapshot.name.trim().to_ascii_lowercase();
    let supported_name = matches!(
        process_name.as_str(),
        "chatgpt" | "chatgpt.exe" | "codex" | "codex.exe"
    );
    let path_matches = app_directory
        .is_some_and(|directory| snapshot.executable_path.starts_with(directory))
        || is_official_codex_store_process_path(&snapshot.executable_path);
    let package_family = if supported_name && !path_matches {
        windows_process_package_family_name(snapshot.pid)
    } else {
        None
    };
    is_codex_desktop_process_with_package(snapshot, app_directory, package_family.as_deref())
}

#[cfg(windows)]
fn is_codex_desktop_process_with_package(
    snapshot: &ProcessSnapshot,
    app_directory: Option<&Path>,
    package_family: Option<&str>,
) -> bool {
    let process_name = snapshot.name.trim().to_ascii_lowercase();
    matches!(
        process_name.as_str(),
        "chatgpt" | "chatgpt.exe" | "codex" | "codex.exe"
    ) && (app_directory.is_some_and(|directory| snapshot.executable_path.starts_with(directory))
        || is_official_codex_store_process_path(&snapshot.executable_path)
        || package_family
            .is_some_and(|family| family.eq_ignore_ascii_case(WINDOWS_STORE_CODEX_PACKAGE_FAMILY)))
}

#[cfg(windows)]
fn is_official_codex_store_process_path(path: &Path) -> bool {
    let normalized = path
        .to_string_lossy()
        .replace('/', "\\")
        .to_ascii_lowercase();
    normalized.contains("\\windowsapps\\openai.codex_")
        && normalized.contains("__2p2nqsd0c76g0\\app\\")
}

#[cfg(windows)]
fn windows_process_package_family_name(pid: u32) -> Option<String> {
    const PROCESS_QUERY_LIMITED_INFORMATION: u32 = 0x1000;
    const ERROR_INSUFFICIENT_BUFFER: i32 = 122;

    #[link(name = "kernel32")]
    extern "system" {
        fn OpenProcess(
            desired_access: u32,
            inherit_handle: i32,
            process_id: u32,
        ) -> *mut std::ffi::c_void;
        fn GetPackageFamilyName(
            process: *mut std::ffi::c_void,
            package_family_name_length: *mut u32,
            package_family_name: *mut u16,
        ) -> i32;
        fn CloseHandle(handle: *mut std::ffi::c_void) -> i32;
    }

    unsafe {
        let process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, 0, pid);
        if process.is_null() {
            return None;
        }
        let mut length = 0_u32;
        let first = GetPackageFamilyName(process, &mut length, std::ptr::null_mut());
        if first != ERROR_INSUFFICIENT_BUFFER || length == 0 || length > 512 {
            let _ = CloseHandle(process);
            return None;
        }
        let mut buffer = vec![0_u16; length as usize];
        let result = GetPackageFamilyName(process, &mut length, buffer.as_mut_ptr());
        let _ = CloseHandle(process);
        if result != 0 {
            return None;
        }
        if buffer.last() == Some(&0) {
            buffer.pop();
        }
        String::from_utf16(&buffer).ok()
    }
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

#[cfg(target_os = "macos")]
fn codex_process_running_from_system() -> bool {
    !platform::macos::process_ids().is_empty()
}

#[cfg(target_os = "macos")]
fn codex_process_ids_from_system() -> Vec<u32> {
    platform::macos::process_ids()
}

#[cfg(target_os = "macos")]
fn stop_process_tree(pid: u32) -> Result<(), String> {
    platform::macos::stop_process(pid)
}

#[cfg(not(any(windows, target_os = "macos")))]
fn codex_process_running_from_system() -> bool {
    false
}

#[cfg(not(any(windows, target_os = "macos")))]
fn codex_process_ids_from_system() -> Vec<u32> {
    Vec::new()
}

#[cfg(not(any(windows, target_os = "macos")))]
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

#[cfg(test)]
fn is_codex_process(name: &str, command_line: &str) -> bool {
    let process_name = name.trim().to_ascii_lowercase();
    if process_name == "codex" || process_name == "codex.exe" {
        return true;
    }
    command_line_mentions_codex(command_line)
}

#[cfg(test)]
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

#[cfg(test)]
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
    if cfg!(debug_assertions) {
        let runtime = env::var("CODEXPPP_BACKEND_API_BASE").ok();
        let buildtime = option_env!("CODEXPPP_BACKEND_API_BASE").map(str::to_string);
        return runtime
            .into_iter()
            .chain(buildtime)
            .find_map(|value| normalize_backend_api_base(&value))
            .unwrap_or_else(production_backend_api_base);
    }
    production_backend_api_base()
}

fn production_backend_api_base() -> String {
    DEFAULT_BACKEND_API_BASE.to_string()
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
    let computer = first_env(&["COMPUTERNAME", "HOSTNAME"]).unwrap_or_else(|| {
        if cfg!(target_os = "macos") {
            "Mac"
        } else {
            "Windows"
        }
        .to_string()
    });
    let platform = if cfg!(target_os = "macos") {
        "macOS"
    } else if cfg!(windows) {
        "Windows"
    } else {
        "桌面"
    };
    format!("{platform} 设备 {computer}")
}

fn stable_device_seed() -> String {
    let machine = platform_machine_id()
        .or_else(|| first_env(&["COMPUTERNAME", "HOSTNAME"]))
        .unwrap_or_else(|| "unknown-machine".to_string());
    let user = first_env(&["USERNAME", "USER"]).unwrap_or_else(|| "unknown-user".to_string());
    format!("codexppp-device:{machine}:{user}")
}

fn platform_machine_id() -> Option<String> {
    #[cfg(windows)]
    {
        return windows_machine_guid();
    }
    #[cfg(target_os = "macos")]
    {
        return platform::macos::machine_id();
    }
    #[cfg(not(any(windows, target_os = "macos")))]
    {
        None
    }
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
        assert_eq!(
            provider.get("name").and_then(Item::as_str),
            Some("Codex+++")
        );
        assert_eq!(
            provider.get("wire_api").and_then(Item::as_str),
            Some("responses")
        );
        assert_eq!(
            provider.get("base_url").and_then(Item::as_str),
            Some("http://localhost:8787/api/codex/v1")
        );
        assert_eq!(
            provider.get("requires_openai_auth").and_then(Item::as_bool),
            Some(true)
        );
        assert_eq!(
            doc.get("desktop")
                .and_then(Item::as_table)
                .and_then(|desktop| desktop.get("localeOverride"))
                .and_then(Item::as_str),
            Some(DEFAULT_CODEX_LOCALE)
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
    fn default_locale_is_chinese_without_overriding_an_explicit_choice() {
        let mut missing = "model = \"gpt-5.4\"\n".parse::<DocumentMut>().unwrap();
        ensure_default_codex_locale(&mut missing);
        assert_eq!(
            missing["desktop"]["localeOverride"].as_str(),
            Some(DEFAULT_CODEX_LOCALE)
        );

        let mut explicit = "[desktop]\nlocaleOverride = \"en\"\n"
            .parse::<DocumentMut>()
            .unwrap();
        ensure_default_codex_locale(&mut explicit);
        assert_eq!(explicit["desktop"]["localeOverride"].as_str(), Some("en"));

        let mut inline = "desktop = { localeOverride = \"ja-JP\", notifications = false }\n"
            .parse::<DocumentMut>()
            .unwrap();
        ensure_default_codex_locale(&mut inline);
        assert_eq!(inline["desktop"]["localeOverride"].as_str(), Some("ja-JP"));
    }

    #[test]
    fn prepare_replaces_chatgpt_login_for_pool_use_and_restores_original_state() {
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
        let previous_backup = env::var("CODEXPPP_TEST_CODEX_BACKUP_HOME").ok();
        let backup_home = dir.join("backups");
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);
        env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", &backup_home);

        fs::write(&config, "broken = [").unwrap();
        fs::write(managed.join("codex-provider-token"), "old-token").unwrap();
        let target =
            CodexLaunchTarget::Command(dir.join(if cfg!(windows) { "codex.exe" } else { "codex" }));
        prepare_codex_for_target(
            &target,
            "http://localhost:8787/api",
            "secret-provider-token",
        )
        .unwrap();
        assert!(!managed.exists());
        assert_codexppp_api_key_config(&fs::read_to_string(&user_config).unwrap());
        let managed_auth: Value =
            serde_json::from_str(&fs::read_to_string(user.join("auth.json")).unwrap()).unwrap();
        assert_eq!(
            managed_auth.get("OPENAI_API_KEY").and_then(Value::as_str),
            Some("secret-provider-token")
        );
        restore_codex_user_state().unwrap();
        assert_eq!(
            fs::read_to_string(&user_config).unwrap(),
            "model = \"user-normal\"\n"
        );
        let restored_auth: Value =
            serde_json::from_str(&fs::read_to_string(user.join("auth.json")).unwrap()).unwrap();
        assert_eq!(
            restored_auth
                .pointer("/tokens/account_id")
                .and_then(Value::as_str),
            Some("user-account")
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
        if let Some(value) = previous_backup {
            env::set_var("CODEXPPP_TEST_CODEX_BACKUP_HOME", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_BACKUP_HOME");
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
        prepare_codex_for_target(
            &target,
            "http://localhost:8787/api",
            "secret-provider-token",
        )
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
        assert_codexppp_api_key_config(&written);
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
        restore_codex_user_state().unwrap();
        assert_eq!(fs::read_to_string(&config).unwrap(), original);
        let restored_auth: Value =
            serde_json::from_str(&fs::read_to_string(user.join("auth.json")).unwrap()).unwrap();
        assert_eq!(
            restored_auth
                .pointer("/tokens/account_id")
                .and_then(Value::as_str),
            Some("user-account")
        );

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
    fn prepare_codex_does_not_require_browser_runtime_before_activation() {
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
        let previous_browser = env::var("CODEXPPP_TEST_BROWSER_PLUGIN_ENABLED").ok();
        let previous_version_verified = env::var("CODEXPPP_TEST_CODEX_VERSION_VERIFIED").ok();
        let command_dir = dir.join("bin");
        fs::create_dir_all(&command_dir).unwrap();
        let command = command_dir.join(if cfg!(windows) { "codex.exe" } else { "codex" });
        fs::write(&command, "").unwrap();
        env::set_var("CODEXPPP_TEST_CODEX_HOME", &user_home);
        env::set_var("CODEXPPP_TEST_MANAGED_CODEX_HOME", &managed);
        env::set_var("CODEXPPP_CODEX_COMMAND", &command);
        env::set_var("CODEXPPP_CODEX_RUNNING_FOR_TEST", "0");
        env::set_var("CODEXPPP_TEST_BROWSER_PLUGIN_ENABLED", "0");
        env::set_var("CODEXPPP_TEST_CODEX_VERSION_VERIFIED", "1");

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
        assert_eq!(
            auth.get("auth_mode").and_then(Value::as_str),
            Some("apikey")
        );
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
        if let Some(value) = previous_browser {
            env::set_var("CODEXPPP_TEST_BROWSER_PLUGIN_ENABLED", value);
        } else {
            env::remove_var("CODEXPPP_TEST_BROWSER_PLUGIN_ENABLED");
        }
        if let Some(value) = previous_version_verified {
            env::set_var("CODEXPPP_TEST_CODEX_VERSION_VERIFIED", value);
        } else {
            env::remove_var("CODEXPPP_TEST_CODEX_VERSION_VERIFIED");
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
    fn app_id_prepare_routes_through_pool_and_restores_chatgpt_login() {
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
        prepare_codex_for_target(
            &target,
            "http://localhost:8787/api",
            "secret-provider-token",
        )
        .unwrap();

        let written = fs::read_to_string(user_home.join("config.toml")).unwrap();
        assert!(written.contains("model = \"gpt-5.4\""));
        assert!(written.contains("[features]"));
        assert!(written.contains("[mcp_servers.filesystem]"));
        assert_codexppp_api_key_config(&written);
        let auth: Value =
            serde_json::from_str(&fs::read_to_string(user_home.join("auth.json")).unwrap())
                .unwrap();
        assert_eq!(
            auth.get("OPENAI_API_KEY").and_then(Value::as_str),
            Some("secret-provider-token")
        );
        assert_eq!(
            codex_account_from_auth(),
            Some("secret-pro...oken".to_string())
        );
        assert!(codexppp_config_clean());
        let backup_files = fs::read_dir(&backup_home)
            .unwrap()
            .filter_map(Result::ok)
            .flat_map(|entry| fs::read_dir(entry.path()).unwrap().filter_map(Result::ok))
            .map(|entry| entry.file_name().to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert!(backup_files.iter().any(|name| name == "config.toml"));
        assert!(backup_files.iter().any(|name| name == "auth.json"));
        restore_codex_user_state().unwrap();
        let restored = fs::read_to_string(user_home.join("config.toml")).unwrap();
        assert!(restored.contains("model = \"gpt-5.4\""));
        assert!(restored.contains("http://legacy.invalid"));
        let restored_auth: Value =
            serde_json::from_str(&fs::read_to_string(user_home.join("auth.json")).unwrap())
                .unwrap();
        assert_eq!(
            restored_auth
                .pointer("/tokens/account_id")
                .and_then(Value::as_str),
            Some("user-account")
        );

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
        let desktop_command = app_dir.join("ChatGPT.exe");
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
    fn codex_account_displays_local_api_key_login_when_no_named_account_exists() {
        let managed_key = format!("sk-{}", "8".repeat(64));
        let auth = serde_json::json!({
            "OPENAI_API_KEY": managed_key
        });
        assert_eq!(
            codex_account_from_auth_value(&auth),
            Some(mask_api_key(&managed_key))
        );
        assert_eq!(codex_auth_mode_from_value(&auth), "api_key");

        let external_key = "sk-proj-external-account-visible";
        let auth = serde_json::json!({
            "OPENAI_API_KEY": external_key
        });
        assert_eq!(
            codex_account_from_auth_value(&auth),
            Some(mask_api_key(external_key))
        );
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

    #[cfg(windows)]
    #[test]
    fn windows_running_detection_accepts_only_store_package_processes() {
        let app_directory =
            PathBuf::from(r"C:\Program Files\WindowsApps\OpenAI.Codex_test_x64__2p2nqsd0c76g0\app");
        let desktop = ProcessSnapshot {
            pid: 100,
            name: "ChatGPT.exe".to_string(),
            executable_path: app_directory.join("ChatGPT.exe"),
        };
        let packaged_helper = ProcessSnapshot {
            pid: 101,
            name: "codex.exe".to_string(),
            executable_path: app_directory.join("resources").join("codex.exe"),
        };
        let standalone_cli = ProcessSnapshot {
            pid: 102,
            name: "codex.exe".to_string(),
            executable_path: PathBuf::from(
                r"C:\Users\test\AppData\Local\Programs\OpenAI\Codex\bin\codex.exe",
            ),
        };
        let other_store_version = ProcessSnapshot {
            pid: 103,
            name: "ChatGPT.exe".to_string(),
            executable_path: PathBuf::from(
                r"C:\Program Files\WindowsApps\OpenAI.Codex_26.706.1.0_x64__2p2nqsd0c76g0\app\ChatGPT.exe",
            ),
        };
        let protected_store_process = ProcessSnapshot {
            pid: 104,
            name: "ChatGPT.exe".to_string(),
            executable_path: PathBuf::new(),
        };

        assert!(is_codex_desktop_process_with_package(
            &desktop,
            Some(&app_directory),
            None
        ));
        assert!(is_codex_desktop_process_with_package(
            &packaged_helper,
            Some(&app_directory),
            None
        ));
        assert!(is_codex_desktop_process_with_package(
            &other_store_version,
            Some(&app_directory),
            None
        ));
        assert!(is_codex_desktop_process_with_package(
            &protected_store_process,
            Some(&app_directory),
            Some(WINDOWS_STORE_CODEX_PACKAGE_FAMILY)
        ));
        assert!(is_codex_desktop_process_with_package(
            &protected_store_process,
            None,
            Some(WINDOWS_STORE_CODEX_PACKAGE_FAMILY)
        ));
        assert!(!is_codex_desktop_process_with_package(
            &protected_store_process,
            None,
            Some("OpenAI.ChatGPT_otherpublisher")
        ));
        assert!(!is_codex_desktop_process_with_package(
            &standalone_cli,
            Some(&app_directory),
            None
        ));
    }

    #[cfg(windows)]
    #[test]
    fn windows_tasklist_csv_parses_process_identity_without_a_path() {
        let snapshot = process_snapshot_from_tasklist_line(
            r#""ChatGPT.exe","1892","Console","4","547,484 K""#,
        )
        .expect("tasklist row should parse");
        assert_eq!(snapshot.pid, 1892);
        assert_eq!(snapshot.name, "ChatGPT.exe");
        assert!(snapshot.executable_path.as_os_str().is_empty());
        assert_eq!(
            windows_csv_fields(r#""Codex ""Desktop"".exe","42","Console""#),
            vec!["Codex \"Desktop\".exe", "42", "Console"]
        );
    }

    #[cfg(windows)]
    #[test]
    fn windows_tasklist_apps_requires_the_official_codex_store_package() {
        assert_eq!(
            windows_store_codex_process_id_from_tasklist_app_line(
                r#""ChatGPT.exe (App)","1892","577,644 K","OpenAI.Codex_26.707.9981.0_x64__2p2nqsd0c76g0""#,
            ),
            Some(1892)
        );
        assert_eq!(
            windows_store_codex_process_id_from_tasklist_app_line(
                r#""Codex.exe (App)","42","90,000 K","OpenAI.Codex_26.707.72221.0_x64__2p2nqsd0c76g0""#,
            ),
            Some(42)
        );
        assert_eq!(
            windows_store_codex_process_id_from_tasklist_app_line(
                r#""ChatGPT.exe (App)","1892","577,644 K","Other.ChatGPT_1.0.0.0_x64__otherpublisher""#,
            ),
            None
        );
        assert_eq!(
            windows_store_codex_process_id_from_tasklist_app_line(
                r#""codex-code-mode-host.exe (App)","5640","27,844 K","OpenAI.Codex_26.707.9981.0_x64__2p2nqsd0c76g0""#,
            ),
            None
        );
    }

    #[cfg(windows)]
    #[test]
    fn windows_package_family_lookup_confirms_running_store_codex_when_present() {
        let Some(process) = codex_process_snapshots().into_iter().find(|snapshot| {
            snapshot.name.eq_ignore_ascii_case("ChatGPT.exe")
                || snapshot.name.eq_ignore_ascii_case("ChatGPT")
        }) else {
            return;
        };
        let family = windows_process_package_family_name(process.pid)
            .expect("running ChatGPT Store process should expose a package family");
        assert_eq!(family, WINDOWS_STORE_CODEX_PACKAGE_FAMILY);
    }

    #[cfg(windows)]
    #[test]
    fn windows_activation_pid_can_be_checked_without_process_enumeration() {
        assert!(windows_process_is_running(std::process::id()));
        assert_eq!(
            activate_windows_store_codex("Other.App_publisher!App"),
            Err("codex_appid_invalid".to_string())
        );
    }

    #[test]
    fn desktop_version_reports_package_version() {
        assert_eq!(desktop_version(), env!("CARGO_PKG_VERSION"));
    }

    #[test]
    fn update_download_url_requires_https_executable_without_credentials() {
        assert_eq!(
            validate_update_download_url(" https://example.com/codexppp.exe ").unwrap(),
            "https://example.com/codexppp.exe"
        );
        assert_eq!(
            validate_update_download_url("https://example.com/codexppp.exe?channel=stable")
                .unwrap(),
            "https://example.com/codexppp.exe?channel=stable"
        );
        assert!(validate_update_download_url("http://example.com/codexppp.exe").is_err());
        assert!(validate_update_download_url("file:///tmp/codexppp.exe").is_err());
        assert!(validate_update_download_url("https://user@example.com/codexppp.exe").is_err());
        assert!(validate_update_download_url("https://example.com/codexppp.zip").is_err());
        assert!(validate_update_download_url("https://example.com/\"bad\".exe").is_err());
        assert!(validate_update_download_url("").is_err());
    }

    #[test]
    fn update_sha256_requires_complete_hex_digest() {
        let uppercase = "ABCDEF0123456789".repeat(4);
        assert_eq!(
            normalize_update_sha256(&uppercase).unwrap(),
            uppercase.to_ascii_lowercase()
        );
        assert!(normalize_update_sha256("abc123").is_err());
        assert!(normalize_update_sha256(&"g".repeat(64)).is_err());
    }

    #[test]
    fn update_helper_waits_for_client_and_runs_silent_installer() {
        let installer = PathBuf::from(r"C:\Temp\codexppp-update\CodexPPP-update.exe");
        let update_dir = installer.parent().unwrap();
        let result = build_desktop_update_helper_command(&installer, update_dir);
        if cfg!(windows) {
            let cmd = result.unwrap();
            let program = cmd.get_program().to_string_lossy().to_string();
            let args = cmd
                .get_args()
                .map(|arg| arg.to_string_lossy().to_string())
                .collect::<Vec<_>>();
            assert_eq!(program.to_ascii_lowercase(), "powershell");
            assert!(args.iter().any(|arg| arg == "Hidden"));
            assert!(args.iter().any(|arg| arg == DESKTOP_UPDATE_HELPER_SCRIPT));
            assert!(DESKTOP_UPDATE_HELPER_SCRIPT.contains("-ArgumentList '/S'"));
            assert!(DESKTOP_UPDATE_HELPER_SCRIPT.contains("CODEXPPP_UPDATE_PARENT_PID"));
            assert!(!args
                .join(" ")
                .contains(installer.to_string_lossy().as_ref()));
        } else {
            assert_eq!(result.unwrap_err(), "update_install_failed");
        }
    }

    #[test]
    fn codex_install_command_uses_noninteractive_winget_msstore_install() {
        let winget = PathBuf::from(r"C:\Users\1\AppData\Local\Microsoft\WindowsApps\winget.exe");
        let cmd = build_codex_install_command_with_winget(winget.clone(), false);
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
    fn codex_upgrade_command_uses_store_product_without_interaction() {
        let winget = PathBuf::from(r"C:\Users\1\AppData\Local\Microsoft\WindowsApps\winget.exe");
        let cmd = build_codex_install_command_with_winget(winget, true);
        let args = cmd
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert_eq!(args.first().map(String::as_str), Some("upgrade"));
        assert!(args
            .windows(2)
            .any(|pair| pair == ["--id", WINDOWS_STORE_CODEX_PRODUCT_ID]));
        assert!(args.iter().any(|arg| arg == "--disable-interactivity"));
    }

    #[test]
    fn codex_update_check_uses_live_store_metadata_without_a_fixed_version() {
        let winget = PathBuf::from(r"C:\Users\test\AppData\Local\Microsoft\WindowsApps\winget.exe");
        let source_refresh = build_codex_store_source_refresh_command(winget.clone());
        let source_args = source_refresh
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert_eq!(source_args.first().map(String::as_str), Some("source"));
        assert!(source_args
            .windows(2)
            .any(|pair| pair == ["--name", "msstore"]));

        let command = build_codex_update_check_command(winget);
        let args = command
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();
        assert_eq!(args.first().map(String::as_str), Some("list"));
        assert!(args
            .windows(2)
            .any(|pair| pair == ["--id", WINDOWS_STORE_CODEX_PRODUCT_ID]));
        assert!(!args.iter().any(|arg| arg == "--upgrade-available"));
        assert!(args.iter().any(|arg| arg == "--disable-interactivity"));
        assert!(!args.iter().any(|arg| arg.starts_with("26.")));

        let output = "Name       Id            Version       Available     Source\nChatGPT    9PLM9XGG6VKS 26.707.9981.0 26.708.12000.0 msstore\n";
        assert_eq!(
            winget_codex_update_row(output),
            Some((
                "26.707.9981.0".to_string(),
                Some("26.708.12000.0".to_string())
            ))
        );
        let current = "Name       Id            Version       Source\nChatGPT    9PLM9XGG6VKS 26.707.9981.0 msstore\n";
        assert_eq!(
            winget_codex_update_row(current),
            Some(("26.707.9981.0".to_string(), None))
        );
        assert_eq!(winget_codex_update_row("No installed package found."), None);
        assert!(parse_codex_desktop_package_version("26.707.9981.0").is_some());
        assert!(parse_codex_desktop_package_version("unknown").is_none());
    }

    #[test]
    fn codex_install_fallback_uses_official_microsoft_store_web_installer() {
        let cmd = build_codex_store_web_installer_command().unwrap();
        let program = cmd.get_program().to_string_lossy().to_string();
        let args = cmd
            .get_args()
            .map(|arg| arg.to_string_lossy().to_string())
            .collect::<Vec<_>>();
        let installer_url = cmd
            .get_envs()
            .find_map(|(key, value)| {
                (key == "CODEXPPP_CODEX_DESKTOP_INSTALLER_URL")
                    .then(|| value.map(|value| value.to_string_lossy().to_string()))
                    .flatten()
            })
            .expect("missing official desktop installer URL");

        if cfg!(windows) {
            assert_eq!(program.to_ascii_lowercase(), "powershell");
            assert!(args.iter().any(|arg| arg == CODEX_STORE_WEB_INSTALL_SCRIPT));
            assert_eq!(installer_url, WINDOWS_STORE_CODEX_INSTALLER_URL);
            assert!(installer_url.starts_with("https://get.microsoft.com/"));
            assert!(CODEX_STORE_WEB_INSTALL_SCRIPT.contains("Get-AuthenticodeSignature"));
            assert!(CODEX_STORE_WEB_INSTALL_SCRIPT.contains("Microsoft Corporation"));
            assert!(CODEX_STORE_WEB_INSTALL_SCRIPT.contains("-ArgumentList '--silent'"));
            assert!(!CODEX_STORE_WEB_INSTALL_SCRIPT.contains("github.com/openai/codex"));
        }
    }

    #[test]
    fn codex_install_closes_installer_autolaunch_before_reporting_ready() {
        use std::cell::{Cell, RefCell};
        use std::collections::VecDeque;

        let observations = RefCell::new(VecDeque::from([false, true, true, false, false, false]));
        let stops = Cell::new(0_u32);
        let waits = Cell::new(0_u32);
        observe_codex_install_quiet(
            || observations.borrow_mut().pop_front().unwrap_or(false),
            || {
                stops.set(stops.get() + 1);
                Ok(())
            },
            || waits.set(waits.get() + 1),
            10,
            3,
        )
        .unwrap();

        assert_eq!(stops.get(), 2);
        assert!(waits.get() >= 4);
    }

    #[test]
    fn codex_install_fails_when_installer_autolaunch_cannot_be_stopped() {
        let result = observe_codex_install_quiet(
            || true,
            || Err("codex_stop_failed".to_string()),
            || {},
            3,
            2,
        );
        assert_eq!(
            result,
            Err("codex_install_autolaunch_stop_failed".to_string())
        );
    }

    #[test]
    fn codex_version_is_read_from_command_output() {
        let dir = env::temp_dir().join(format!("codexppp-version-test-{}", now_unix_nanos()));
        fs::create_dir_all(&dir).unwrap();
        let command = dir.join(if cfg!(windows) { "codex.cmd" } else { "codex" });
        if cfg!(windows) {
            fs::write(&command, "@echo off\r\necho codex-cli 0.143.0\r\n").unwrap();
        } else {
            fs::write(&command, "#!/bin/sh\necho codex-cli 0.143.0\n").unwrap();
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let mut permissions = fs::metadata(&command).unwrap().permissions();
                permissions.set_mode(0o755);
                fs::set_permissions(&command, permissions).unwrap();
            }
        }

        assert_eq!(
            codex_version_from_command(&command),
            Some("codex-cli 0.143.0".to_string())
        );

        fs::remove_dir_all(dir).unwrap();
    }

    #[test]
    fn codex_version_sanitizer_drops_control_chars_and_limits_length() {
        let raw = format!("codex-cli 0.143.0\u{0000}{}", "x".repeat(200));
        let sanitized = sanitize_codex_version_line(&raw);
        assert!(!sanitized.contains('\u{0000}'));
        assert!(sanitized.len() <= 96);
    }

    #[test]
    fn browser_plugin_status_requires_installed_and_enabled_official_plugin() {
        let enabled = serde_json::json!({
            "installed": [{
                "pluginId": CODEX_BROWSER_PLUGIN_ID,
                "installed": true,
                "enabled": true
            }]
        });
        assert!(codex_browser_plugin_enabled_from_json(
            serde_json::to_string(&enabled).unwrap().as_bytes()
        ));

        for payload in [
            serde_json::json!({"installed": [{"pluginId": CODEX_BROWSER_PLUGIN_ID, "installed": true, "enabled": false}]}),
            serde_json::json!({"installed": [{"pluginId": "browser@untrusted", "installed": true, "enabled": true}]}),
            serde_json::json!({"available": [{"pluginId": CODEX_BROWSER_PLUGIN_ID, "installed": false, "enabled": false}]}),
        ] {
            assert!(!codex_browser_plugin_enabled_from_json(
                serde_json::to_string(&payload).unwrap().as_bytes()
            ));
        }
        assert!(!codex_browser_plugin_enabled_from_json(b"not json"));
    }

    #[test]
    fn browser_plugin_commands_use_official_codex_plugin_interface() {
        let command =
            PathBuf::from(r"C:\Users\test\AppData\Local\Programs\OpenAI\Codex\bin\codex.exe");
        let list = build_codex_plugin_list_command(&command);
        assert_eq!(list.get_program(), command.as_os_str());
        assert_eq!(
            list.get_args()
                .map(|arg| arg.to_string_lossy().to_string())
                .collect::<Vec<_>>(),
            vec!["plugin", "list", "--json"]
        );

        let install = build_codex_browser_plugin_install_command(&command);
        assert_eq!(install.get_program(), command.as_os_str());
        assert_eq!(
            install
                .get_args()
                .map(|arg| arg.to_string_lossy().to_string())
                .collect::<Vec<_>>(),
            vec!["plugin", "add", CODEX_BROWSER_PLUGIN_ID, "--json"]
        );
    }

    #[cfg(windows)]
    #[test]
    fn browser_plugin_command_detects_official_desktop_runtime_layout() {
        let local_app_data = env::temp_dir().join(format!(
            "codexppp-official-runtime-test-{}",
            now_unix_nanos()
        ));
        let runtime = local_app_data
            .join("OpenAI")
            .join("Codex")
            .join("bin")
            .join("runtime-hash")
            .join("codex.exe");
        fs::create_dir_all(runtime.parent().unwrap()).unwrap();
        fs::write(&runtime, "official desktop runtime").unwrap();

        assert_eq!(
            official_codex_runtime_command(&local_app_data),
            Some(runtime)
        );

        fs::remove_dir_all(local_app_data).unwrap();
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
    fn backend_api_base_is_fixed_for_production_and_normalized_for_development() {
        assert_eq!(DEFAULT_BACKEND_API_BASE, "https://codex.52cx.top/api");
        assert_eq!(production_backend_api_base(), "https://codex.52cx.top/api");
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
        assert!(ui.contains("function codexAccountDisplay()"));
        assert!(ui.contains("state.diagnostics.codexAccount"));
        assert!(ui.contains("state.diagnostics.codexVersion"));
        assert!(!ui.contains("backendCodexAccount"));
        assert!(!ui.contains("prep.provider.account"));
        assert!(!ui.contains("Codex+++ 授权"));
    }

    #[test]
    fn windows_client_bundle_configuration_is_enabled() {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let source = fs::read_to_string(manifest_dir.join("src").join("main.rs")).unwrap();
        let tauri_config = fs::read_to_string(manifest_dir.join("tauri.conf.json")).unwrap();
        let windows_config =
            fs::read_to_string(manifest_dir.join("tauri.windows.conf.json")).unwrap();

        assert!(
            source.contains(
                r#"#![cfg_attr(all(windows, not(debug_assertions)), windows_subsystem = "windows")]"#
            ),
            "Windows release client must use the GUI subsystem so opening the app does not show a console window"
        );
        for required in [
            "\"productName\": \"Codex+++\"",
            "\"identifier\": \"com.codexppp.desktop\"",
            "\"active\": true",
            "\"publisher\": \"Codex+++\"",
        ] {
            assert!(
                tauri_config.contains(required),
                "Windows client release config missing {required}"
            );
        }
        for required in ["\"targets\": [\"nsis\"]", "\"icon\": [\"icons/icon.ico\"]"] {
            assert!(
                windows_config.contains(required),
                "Windows platform config missing {required}"
            );
        }
        assert!(
            !tauri_config.contains("\"active\": false"),
            "Windows client bundle must stay enabled"
        );
    }

    #[test]
    fn macos_client_bundle_configuration_requires_current_platform_boundaries() {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let config = fs::read_to_string(manifest_dir.join("tauri.macos.conf.json")).unwrap();
        let build_script = fs::read_to_string(
            manifest_dir
                .join("..")
                .join("..")
                .join("scripts")
                .join("build-macos-client.sh"),
        )
        .unwrap();
        let test_workflow = fs::read_to_string(
            manifest_dir
                .join("..")
                .join("..")
                .join(".github")
                .join("workflows")
                .join("macos-client-test.yml"),
        )
        .unwrap();
        for required in [
            "\"targets\": [\"app\", \"dmg\"]",
            "\"icon\": [\"icons/icon.icns\"]",
            "\"minimumSystemVersion\": \"14.0\"",
        ] {
            assert!(config.contains(required), "macOS config missing {required}");
        }
        assert!(build_script.contains("APPLE_SIGNING_IDENTITY"));
        assert!(build_script.contains("notarization credentials"));
        assert!(build_script.contains("cargo test"));
        assert!(test_workflow.contains("runs-on: macos-14"));
        assert!(test_workflow.contains("sh scripts/build-macos-client.sh"));
        assert!(test_workflow.contains("ad-hoc test package"));
    }

    #[test]
    fn managed_runtime_survives_window_hiding_and_cleans_up_on_exit() {
        let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        let source = fs::read_to_string(manifest_dir.join("src").join("main.rs")).unwrap();
        let cargo_toml = fs::read_to_string(manifest_dir.join("Cargo.toml")).unwrap();
        let ui = fs::read_to_string(manifest_dir.join("..").join("ui").join("index.html")).unwrap();

        assert!(cargo_toml.contains("\"tray-icon\""));
        assert!(source.contains("WindowEvent::CloseRequested"));
        assert!(source.contains("api.prevent_close()"));
        assert!(source.contains("cleanup_managed_codex_session()"));
        assert!(source.contains("managed_codex_session_active()"));
        assert!(source.contains("__codexpppStopAndExitFromTray"));
        assert!(ui.contains("managedRuntime: true"));
        assert!(ui.contains("if (!state.token || $('app').classList.contains('hidden')) return;"));
        assert!(ui.contains("async function logoutClient()"));
        assert!(ui.contains("async function stopAndExitFromTray()"));
    }
}
