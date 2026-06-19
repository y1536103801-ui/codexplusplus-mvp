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
        "Cloud state loaded.",
        CloudStatePayload {
            state: codex_plus_core::codexplus_cloud::load_state(),
        },
    )
}

#[tauri::command]
pub fn codexplus_cloud_configure_endpoint(base_url: String) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::configure_endpoint(&base_url) {
        Ok(state) => ok("Cloud endpoint configured.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud endpoint configuration failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_login(
    base_url: String,
    email: String,
    password: String,
) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::login(&base_url, &email, &password).await {
        Ok(state) => ok("Cloud login completed.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud login failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_login_2fa(totp_code: String) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::complete_login_2fa(&totp_code).await {
        Ok(state) => ok("Cloud 2FA login completed.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud 2FA login failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_start_browser_handoff(
    base_url: String,
) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::start_browser_handoff(&base_url).await {
        Ok(state) => ok("Cloud browser login started.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud browser login failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_poll_browser_handoff() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::poll_browser_handoff().await {
        Ok(state) => ok("Cloud browser login checked.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud browser login check failed.", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_cancel_browser_handoff() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::cancel_browser_handoff() {
        Ok(state) => ok(
            "Cloud browser login cancelled.",
            CloudStatePayload { state },
        ),
        Err(error) => failed_state("Cloud browser login cancel failed.", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_logout() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::logout() {
        Ok(state) => ok("Cloud session cleared.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud logout failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_refresh_bootstrap() -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::refresh_bootstrap().await {
        Ok(state) => ok("Cloud bootstrap refreshed.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud bootstrap refresh failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_register_device(
    device_name: Option<String>,
) -> CommandResult<CloudStatePayload> {
    match codex_plus_core::codexplus_cloud::register_device(device_name.as_deref()).await {
        Ok(state) => ok("Cloud device registered.", CloudStatePayload { state }),
        Err(error) => failed_state("Cloud device registration failed.", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_apply_managed_provider() -> CommandResult<CloudApplyPayload> {
    match codex_plus_core::codexplus_cloud::apply_managed_provider() {
        Ok((state, apply_result)) => ok(
            "Codex++ Cloud provider applied.",
            CloudApplyPayload {
                state,
                apply_result: Some(apply_result),
            },
        ),
        Err(error) => failed_apply("Codex++ Cloud provider apply failed.", error),
    }
}

#[tauri::command]
pub fn codexplus_cloud_repair_managed_provider() -> CommandResult<CloudApplyPayload> {
    match codex_plus_core::codexplus_cloud::repair_managed_provider() {
        Ok((state, apply_result)) => ok(
            "Codex++ Cloud provider repaired.",
            CloudApplyPayload {
                state,
                apply_result: Some(apply_result),
            },
        ),
        Err(error) => failed_apply("Codex++ Cloud provider repair failed.", error),
    }
}

#[tauri::command]
pub async fn codexplus_cloud_redeem(code: String) -> CommandResult<CloudRedeemPayload> {
    match codex_plus_core::codexplus_cloud::redeem(&code).await {
        Ok(payload) => ok("Cloud redeem completed.", payload),
        Err(error) => failed(
            &format!("Cloud redeem failed: {}", redact_error(error)),
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
        Ok(payload) => ok("Cloud usage loaded.", payload),
        Err(error) => failed(
            &format!("Cloud usage load failed: {}", redact_error(error)),
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
            "Cloud diagnostics loaded.",
            CloudDiagnosticsPayload {
                path: path.to_string_lossy().to_string(),
                text,
                lines,
            },
        ),
        Err(error) => failed(
            &format!(
                "Cloud diagnostics load failed: {}",
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
    codex_plus_core::codexplus_cloud::redaction::redact_string(&error.to_string())
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
