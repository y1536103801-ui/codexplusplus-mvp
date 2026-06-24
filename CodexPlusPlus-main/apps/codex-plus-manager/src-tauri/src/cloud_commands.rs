use std::fs;
use std::path::Path;

use codex_plus_core::codexplus_cloud::{
    CloudRedeemPayload, CloudRuntimeState, CloudUsagePayload, ManagedProviderApplyResult,
};
use serde::Serialize;

use crate::commands::CommandResult;

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudStatePayload {
    pub state: CloudRuntimeState,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudApplyPayload {
    pub state: CloudRuntimeState,
    pub apply_result: Option<ManagedProviderApplyResult>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudDiagnosticsPayload {
    pub path: String,
    pub text: String,
    pub lines: usize,
}

#[tauri::command]
pub fn codexplus_cloud_load_state() -> CommandResult<CloudStatePayload> {
    ok(
        "账户状态已刷新。",
        CloudStatePayload {
            state: codex_plus_core::codexplus_cloud::load_state(),
        },
    )
}

#[tauri::command]
pub fn codexplus_cloud_configure_endpoint(base_url: String) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::configure_endpoint(&base_url) {
        Ok(state) => ok("客户端已连接。", CloudStatePayload { state }),
        Err(error) => failed_state("客户端连接失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_login(
    base_url: String,
    email: String,
    password: String,
) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::login(&base_url, &email, &password).await {
        Ok(state) => ok("登录成功。", CloudStatePayload { state }),
        Err(error) => failed_state("登录失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_login_2fa(totp_code: String) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::complete_login_2fa(&totp_code).await {
        Ok(state) => ok("验证完成。", CloudStatePayload { state }),
        Err(error) => failed_state("验证失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_start_browser_handoff(
    base_url: String,
) -> CommandResult<CloudStatePayload> {
    let current_state = codex_plus_core::codexplus_cloud::load_state();
    if current_state.connection.pending_browser_handoff {
        return ok(
            "请在浏览器完成登录，完成后回到这里检查状态。",
            CloudStatePayload {
                state: current_state,
            },
        );
    }
    match codex_plus_core::codexplus_cloud::start_browser_handoff(&base_url).await {
        Ok(state) => {
            if let Some(url) = state.connection.browser_handoff_authorize_url.as_deref() {
                if let Err(error) = open_browser_login_url(url) {
                    return failed(
                        &format!("无法打开浏览器，请稍后重试：{}", redact_error(error)),
                        CloudStatePayload { state },
                    );
                }
            }
            ok("已打开浏览器，请完成登录。", CloudStatePayload { state })
        }
        Err(error) => failed_state("浏览器登录失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_poll_browser_handoff() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::poll_browser_handoff().await {
        Ok(state) => ok("已检查登录状态。", CloudStatePayload { state }),
        Err(error) => failed_state("检查登录状态失败。", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_cancel_browser_handoff() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::cancel_browser_handoff() {
        Ok(state) => ok(
            "已取消浏览器登录。",
            CloudStatePayload { state },
        ),
        Err(error) => failed_state("取消浏览器登录失败。", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_logout() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::logout() {
        Ok(state) => ok("已退出登录。", CloudStatePayload { state }),
        Err(error) => failed_state("退出登录失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_refresh_bootstrap() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::refresh_bootstrap().await {
        Ok(state) => ok("状态已刷新。", CloudStatePayload { state }),
        Err(error) => failed_state("刷新状态失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_register_device(
    device_name: Option<String>,
) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::register_device(device_name.as_deref()).await {
        Ok(state) => ok("本机已登记。", CloudStatePayload { state }),
        Err(error) => failed_state("本机登记失败。", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_apply_managed_provider() -> CommandResult<CloudApplyPayload> {
    match codex_plus_core::codexplus_cloud::apply_managed_provider() {
        Ok((state, apply_result)) => ok(
            "Codex 已准备好。",
            CloudApplyPayload {
                state,
                apply_result: Some(apply_result),
            },
        ),
        Err(error) => failed_apply("Codex 准备失败。", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_repair_managed_provider() -> CommandResult<CloudApplyPayload> {
    match codex_plus_core::codexplus_cloud::repair_managed_provider() {
        Ok((state, apply_result)) => ok(
            "Codex 配置已修复。",
            CloudApplyPayload {
                state,
                apply_result: Some(apply_result),
            },
        ),
        Err(error) => failed_apply("Codex 修复失败。", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_redeem(code: String) -> CommandResult<CloudRedeemPayload> {
    match codex_plus_core::codexplus_cloud::redeem(&code).await {
        Ok(payload) => ok("操作成功。", payload),
        Err(error) => failed(
            &format!("账户操作失败：{}", redact_error(error)),
            CloudRedeemPayload {
                result: None,
                state: codex_plus_core::codexplus_cloud::load_state(),
            },
        ),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_load_usage() -> CommandResult<CloudUsagePayload> {
    match codex_plus_core::codexplus_cloud::load_usage().await {
        Ok(payload) => ok("用量已刷新。", payload),
        Err(error) => failed(
            &format!("用量刷新失败：{}", redact_error(error)),
            CloudUsagePayload {
                usage: None,
                state: codex_plus_core::codexplus_cloud::load_state(),
            },
        ),
    }
}

#[tauri::command]
pub fn codexplus_cloud_read_redacted_diagnostics(
    lines: Option<usize>,
) -> CommandResult<CloudDiagnosticsPayload> {
    let path = codex_plus_core::diagnostic_log::diagnostic_log_path();
    let lines = lines
        .unwrap_or_else(default_diagnostic_lines)
        .clamp(1, 1000);
    match read_redacted_tail(&path, lines) {
        Ok(text) => ok(
            "诊断信息已读取。",
            CloudDiagnosticsPayload {
                path: path.to_string_lossy().to_string(),
                text,
                lines,
            },
        ),
        Err(error) => failed(
            &format!(
                "读取诊断信息失败：{}",
                redact_error(anyhow::Error::from(error))
            ),
            CloudDiagnosticsPayload {
                path: path.to_string_lossy().to_string(),
                text: String::new(),
                lines,
            },
        ),
    }
}

fn failed_state(message: &str, error: anyhow::Error) -> CommandResult<CloudStatePayload> {
    failed(
        &format!("{message}: {}", redact_error(error)),
        CloudStatePayload {
            state: codex_plus_core::codexplus_cloud::load_state(),
        },
    )
}

fn failed_apply(message: &str, error: anyhow::Error) -> CommandResult<CloudApplyPayload> {
    failed(
        &format!("{message}: {}", redact_error(error)),
        CloudApplyPayload {
            state: codex_plus_core::codexplus_cloud::load_state(),
            apply_result: None,
        },
    )
}

fn open_browser_login_url(url: &str) -> anyhow::Result<()> {
    if !(url.starts_with("https://") || url.starts_with("http://")) {
        anyhow::bail!("登录地址无效");
    }
    #[cfg(windows)]
    {
        codex_plus_core::windows_open_url(url)
    }
    #[cfg(not(windows))]
    {
        std::process::Command::new("open")
            .arg(url)
            .spawn()
            .map(|_| ())
            .map_err(|error| anyhow::anyhow!("启动系统浏览器失败：{error}"))
    }
}

fn read_redacted_tail(path: &Path, max_lines: usize) -> std::io::Result<String> {
    let contents = fs::read_to_string(path)?;
    let mut lines = contents.lines().rev().take(max_lines).collect::<Vec<_>>();
    lines.reverse();
    Ok(lines
        .into_iter()
        .map(codex_plus_core::codexplus_cloud::redaction::redact_string)
        .collect::<Vec<_>>()
        .join("\n"))
}

fn redact_error(error: anyhow::Error) -> String {
    let text = codex_plus_core::codexplus_cloud::redaction::redact_string(&error.to_string());
    let lower = text.to_ascii_lowercase();
    if lower.contains("please sign in again")
        || lower.contains("session expired")
        || lower.contains("token has expired")
        || lower.contains("access token has expired")
        || lower.contains("invalid token")
    {
        return "登录已失效，请重新登录。".to_string();
    }
    if lower.contains("open_external_url")
        || lower.contains("plugin not found")
        || lower.contains("not allowed")
    {
        return "无法打开浏览器，请重启 Codex++ 或联系管理员。".to_string();
    }
    if lower.contains("backend mode is active")
        || lower.contains("only admin login is allowed")
        || lower.contains("self-service auth flows are disabled")
    {
        return "当前无法自行登录，请联系管理员确认客户端登录已开启。".to_string();
    }
    if lower.contains("connection refused")
        || lower.contains("actively refused")
        || lower.contains("timed out")
        || lower.contains("network error")
        || lower.contains("fetch failed")
    {
        return "无法连接 Codex++，请稍后重试或联系管理员。".to_string();
    }
    text
}

fn ok<T: Serialize>(message: &str, payload: T) -> CommandResult<T> {
    CommandResult {
        status: "ok".to_string(),
        message: message.to_string(),
        payload,
    }
}

fn failed<T: Serialize>(message: &str, payload: T) -> CommandResult<T> {
    CommandResult {
        status: "failed".to_string(),
        message: message.to_string(),
        payload,
    }
}

fn default_diagnostic_lines() -> usize {
    200
}
