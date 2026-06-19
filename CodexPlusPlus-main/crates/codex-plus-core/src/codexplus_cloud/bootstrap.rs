use anyhow::Context;
use serde::Serialize;

use super::api::{self, BootstrapSnapshot, Envelope, RedeemResult, UsageSnapshot};
use super::local_state::{
    CloudLocalStore, CloudPendingHandoff, CloudRuntimeState, device_register_request,
    mark_device_registered, now_utc_string, session_from_login, session_from_pending_2fa,
    session_from_pending_handoff, state_with_usage,
};
use super::provider_writer::{self, ManagedProviderApplyResult};
use super::redaction::{append_redacted_diagnostic, redact_string};

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudRedeemPayload {
    pub result: Option<RedeemResult>,
    pub state: CloudRuntimeState,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudUsagePayload {
    pub usage: Option<UsageSnapshot>,
    pub state: CloudRuntimeState,
}

pub fn load_state() -> CloudRuntimeState {
    let settings = crate::settings::SettingsStore::default()
        .load()
        .unwrap_or_default();
    CloudLocalStore::default().state(&settings)
}

pub fn configure_endpoint(base_url: &str) -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    let mut session = store.load_session()?.unwrap_or_default();
    session.base_url = api::normalize_base_url(base_url);
    store.save_session(&session)?;
    append_redacted_diagnostic(
        "codexplus_cloud.configure_endpoint",
        serde_json::json!({
            "base_url": session.base_url,
            "state_dir": store.root().to_string_lossy(),
        }),
    )
    .ok();
    Ok(load_state())
}

pub async fn login(
    base_url: &str,
    email: &str,
    password: &str,
) -> anyhow::Result<CloudRuntimeState> {
    let base_url = api::normalize_base_url(base_url);
    let envelope = api::login(&base_url, email, password).await?;
    if !envelope.is_success() {
        anyhow::bail!(
            "{}",
            redact_string(&format!(
                "{} ({})",
                envelope.message,
                envelope.error_code.as_deref().unwrap_or("unknown")
            ))
        );
    }
    let data = envelope
        .data
        .context("cloud login response did not include session data")?;
    if data.requires_2fa {
        let temp_token = data
            .temp_token
            .as_deref()
            .map(str::trim)
            .filter(|token| !token.is_empty())
            .context("cloud login response requires 2FA but did not include a temp token")?;
        let store = CloudLocalStore::default();
        let pending_label = data
            .user_email_masked
            .clone()
            .or_else(|| data.user_label.clone())
            .or_else(|| Some(email.trim().to_string()));
        let session = session_from_pending_2fa(&base_url, pending_label, temp_token);
        store.save_session(&session)?;
        store.ensure_device(None)?;
        append_redacted_diagnostic(
            "codexplus_cloud.login.2fa_required",
            serde_json::json!({
                "base_url": base_url,
                "has_temp_token": true,
            }),
        )
        .ok();
        return Ok(load_state());
    }
    if data.access_token.trim().is_empty() {
        anyhow::bail!("cloud login response did not include an access token");
    }

    let store = CloudLocalStore::default();
    let user_id = login_user_id(&data, email);
    let user_label = login_user_label(&data, email);
    let session = session_from_login(
        &base_url,
        &user_id,
        Some(user_label),
        &data.access_token,
        data.expires_at,
    );
    store.save_session(&session)?;
    store.ensure_device(None)?;
    append_redacted_diagnostic(
        "codexplus_cloud.login.ok",
        serde_json::json!({
            "base_url": base_url,
            "user_id": user_id,
            "has_access_token": true,
        }),
    )
    .ok();
    Ok(load_state())
}

pub async fn complete_login_2fa(totp_code: &str) -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    let session = store
        .load_session()?
        .context("Codex++ Cloud 2FA session is missing; sign in again")?;
    let temp_token = session
        .pending_2fa_token
        .as_deref()
        .map(str::trim)
        .filter(|token| !token.is_empty())
        .context("Codex++ Cloud 2FA session is missing; sign in again")?;
    let envelope = api::login_2fa(&session.base_url, temp_token, totp_code).await?;
    if !envelope.is_success() {
        anyhow::bail!(
            "{}",
            redact_string(&format!(
                "{} ({})",
                envelope.message,
                envelope.error_code.as_deref().unwrap_or("unknown")
            ))
        );
    }
    let data = envelope
        .data
        .context("cloud 2FA login response did not include session data")?;
    if data.access_token.trim().is_empty() {
        anyhow::bail!("cloud 2FA login response did not include an access token");
    }
    let user_id = login_user_id(&data, session.user_label.as_deref().unwrap_or_default());
    let user_label = login_user_label(&data, session.user_label.as_deref().unwrap_or_default());
    let completed = session_from_login(
        &session.base_url,
        &user_id,
        Some(user_label),
        &data.access_token,
        data.expires_at,
    );
    store.save_session(&completed)?;
    store.ensure_device(None)?;
    append_redacted_diagnostic(
        "codexplus_cloud.login.2fa_ok",
        serde_json::json!({
            "base_url": session.base_url,
            "user_id": user_id,
            "has_access_token": true,
        }),
    )
    .ok();
    Ok(load_state())
}

pub async fn start_browser_handoff(base_url: &str) -> anyhow::Result<CloudRuntimeState> {
    let base_url = api::normalize_base_url(base_url);
    let store = CloudLocalStore::default();
    let device = store.ensure_device(None)?;
    let envelope =
        api::start_browser_handoff(&base_url, &device.device_id, &device.device_name).await?;
    if !envelope.is_success() {
        anyhow::bail!(
            "{}",
            redact_string(&format!(
                "{} ({})",
                envelope.message,
                envelope.error_code.as_deref().unwrap_or("unknown")
            ))
        );
    }
    let data = envelope
        .data
        .context("cloud browser handoff response did not include session data")?;
    if data.session_token.trim().is_empty()
        || data.poll_token.trim().is_empty()
        || data.authorize_url.trim().is_empty()
    {
        anyhow::bail!("cloud browser handoff response is incomplete");
    }

    let handoff = CloudPendingHandoff {
        session_token: data.session_token,
        poll_token: data.poll_token,
        authorize_url: data.authorize_url,
        verification_code: data.verification_code,
        expires_at: data.expires_at,
        poll_interval_seconds: data.poll_interval_seconds.max(1),
        started_at: now_utc_string(),
    };
    store.save_session(&session_from_pending_handoff(&base_url, handoff))?;
    append_redacted_diagnostic(
        "codexplus_cloud.browser_handoff.started",
        serde_json::json!({
            "base_url": base_url,
            "has_authorize_url": true,
            "has_poll_token": true,
        }),
    )
    .ok();
    Ok(load_state())
}

pub async fn poll_browser_handoff() -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    let session = store
        .load_session()?
        .context("Codex++ Cloud browser login is not pending; start browser login again")?;
    let handoff = session
        .pending_handoff
        .clone()
        .context("Codex++ Cloud browser login is not pending; start browser login again")?;
    let envelope = api::poll_browser_handoff(
        &session.base_url,
        &handoff.session_token,
        &handoff.poll_token,
    )
    .await?;
    if !envelope.is_success() {
        anyhow::bail!(
            "{}",
            redact_string(&format!(
                "{} ({})",
                envelope.message,
                envelope.error_code.as_deref().unwrap_or("unknown")
            ))
        );
    }
    let data = envelope
        .data
        .context("cloud browser handoff response did not include session data")?;
    if !matches!(data.status.as_deref(), Some("completed")) {
        append_redacted_diagnostic(
            "codexplus_cloud.browser_handoff.pending",
            serde_json::json!({
                "base_url": session.base_url,
                "status": data.status,
            }),
        )
        .ok();
        return Ok(load_state());
    }
    if data.access_token.trim().is_empty() {
        anyhow::bail!("cloud browser handoff completed without an access token");
    }

    let user_id = login_user_id(&data, "browser-handoff");
    let user_label = login_user_label(&data, "browser-handoff");
    let completed = session_from_login(
        &session.base_url,
        &user_id,
        Some(user_label),
        &data.access_token,
        data.expires_at,
    );
    store.save_session(&completed)?;
    store.ensure_device(None)?;
    append_redacted_diagnostic(
        "codexplus_cloud.browser_handoff.completed",
        serde_json::json!({
            "base_url": session.base_url,
            "user_id": user_id,
            "has_access_token": true,
        }),
    )
    .ok();
    Ok(refresh_bootstrap().await.unwrap_or_else(|_| load_state()))
}

pub fn cancel_browser_handoff() -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    let base_url = store
        .load_session()?
        .map(|session| session.base_url)
        .unwrap_or_default();
    if base_url.trim().is_empty() {
        store.clear_session()?;
    } else {
        let mut session = super::local_state::CloudSession::default();
        session.base_url = base_url;
        store.save_session(&session)?;
    }
    append_redacted_diagnostic(
        "codexplus_cloud.browser_handoff.cancelled",
        serde_json::json!({}),
    )
    .ok();
    Ok(load_state())
}

fn login_user_id(data: &api::LoginData, fallback: &str) -> String {
    if !data.user_id.trim().is_empty() {
        return data.user_id.trim().to_string();
    }
    if let Some(user) = &data.user {
        if user.id > 0 {
            return user.id.to_string();
        }
        if !user.email.trim().is_empty() {
            return user.email.trim().to_string();
        }
        if !user.username.trim().is_empty() {
            return user.username.trim().to_string();
        }
    }
    fallback.trim().to_string()
}

fn login_user_label(data: &api::LoginData, fallback: &str) -> String {
    if let Some(label) = data
        .user_label
        .as_ref()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
    {
        return label.to_string();
    }
    if let Some(user) = &data.user {
        if !user.email.trim().is_empty() {
            return user.email.trim().to_string();
        }
        if !user.username.trim().is_empty() {
            return user.username.trim().to_string();
        }
    }
    fallback.trim().to_string()
}

pub fn logout() -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    store.clear_session()?;
    append_redacted_diagnostic("codexplus_cloud.logout", serde_json::json!({})).ok();
    Ok(load_state())
}

pub async fn register_device(device_name: Option<&str>) -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    let session = require_session(&store)?;
    let mut device = store.ensure_device(device_name)?;
    let request = device_register_request(&device);
    let envelope = api::register_device(&session.base_url, &session.access_token, &request).await?;
    if !envelope.is_success() {
        anyhow::bail!("{}", envelope.message);
    }
    mark_device_registered(&mut device);
    store.save_device(&device)?;
    append_redacted_diagnostic(
        "codexplus_cloud.device_registered",
        serde_json::json!({
            "base_url": session.base_url,
            "device_id": device.device_id,
            "device_status": envelope.data.as_ref().map(|device| device.status.clone()),
        }),
    )
    .ok();
    Ok(load_state())
}

pub async fn refresh_bootstrap() -> anyhow::Result<CloudRuntimeState> {
    let store = CloudLocalStore::default();
    let session = require_session(&store)?;
    let mut device = store.ensure_device(None)?;
    let request = device_register_request(&device);
    let _ = api::register_device(&session.base_url, &session.access_token, &request).await;
    mark_device_registered(&mut device);
    store.save_device(&device)?;

    match api::get_bootstrap(&session.base_url, &session.access_token, &device.device_id).await {
        Ok(envelope) => {
            if is_auth_error(&envelope) {
                let _ = store.clear_session();
            }
            let snapshot = store.save_snapshot(&envelope)?;
            append_redacted_diagnostic(
                "codexplus_cloud.bootstrap.ok",
                serde_json::json!({
                    "base_url": session.base_url,
                    "snapshot_version": snapshot.snapshot_version,
                    "config_version": snapshot.config_version,
                    "status": envelope.data.as_ref().map(|data| data.service.status.clone()),
                    "has_api_key": envelope.data.as_ref().and_then(|data| data.provider.api_key.as_ref()).is_some(),
                }),
            )
            .ok();
            Ok(load_state())
        }
        Err(error) => {
            append_redacted_diagnostic(
                "codexplus_cloud.bootstrap.failed",
                serde_json::json!({
                    "base_url": session.base_url,
                    "error": error.to_string(),
                }),
            )
            .ok();
            if store.load_snapshot()?.is_some() {
                return Ok(state_with_stale_snapshot(load_state(), error.to_string()));
            }
            Err(error)
        }
    }
}

pub fn apply_managed_provider() -> anyhow::Result<(CloudRuntimeState, ManagedProviderApplyResult)> {
    let store = CloudLocalStore::default();
    let snapshot = require_snapshot(&store)?;
    let data = require_bootstrap_data(&snapshot.envelope)?;
    let result = provider_writer::apply_managed_provider_from_bootstrap(data)?;
    Ok((load_state(), result))
}

pub fn repair_managed_provider() -> anyhow::Result<(CloudRuntimeState, ManagedProviderApplyResult)>
{
    apply_managed_provider()
}

pub async fn redeem(code: &str) -> anyhow::Result<CloudRedeemPayload> {
    let store = CloudLocalStore::default();
    let session = require_session(&store)?;
    let device = store.ensure_device(None)?;
    let envelope = api::redeem(
        &session.base_url,
        &session.access_token,
        &device.device_id,
        code,
    )
    .await?;
    if !envelope.is_success() {
        anyhow::bail!("{}", envelope.message);
    }
    let state = refresh_bootstrap().await.unwrap_or_else(|_| load_state());
    Ok(CloudRedeemPayload {
        result: envelope.data,
        state,
    })
}

pub async fn load_usage() -> anyhow::Result<CloudUsagePayload> {
    let store = CloudLocalStore::default();
    let session = require_session(&store)?;
    let device = store.ensure_device(None)?;
    let envelope =
        api::get_usage(&session.base_url, &session.access_token, &device.device_id).await?;
    if !envelope.is_success() {
        anyhow::bail!("{}", envelope.message);
    }
    let state = state_with_usage(load_state(), Some(&envelope));
    Ok(CloudUsagePayload {
        usage: envelope.data,
        state,
    })
}

fn require_session(store: &CloudLocalStore) -> anyhow::Result<super::local_state::CloudSession> {
    let session = store
        .load_session()?
        .context("Codex++ Cloud session is missing; sign in first")?;
    if session.base_url.trim().is_empty() || session.access_token.trim().is_empty() {
        anyhow::bail!("Codex++ Cloud session is incomplete; sign in first");
    }
    Ok(session)
}

fn require_snapshot(
    store: &CloudLocalStore,
) -> anyhow::Result<super::local_state::CloudBootstrapSnapshotFile> {
    store
        .load_snapshot()?
        .context("Codex++ Cloud bootstrap snapshot is missing; refresh first")
}

fn require_bootstrap_data(
    envelope: &Envelope<BootstrapSnapshot>,
) -> anyhow::Result<&BootstrapSnapshot> {
    envelope
        .data
        .as_ref()
        .context("Codex++ Cloud bootstrap snapshot has no provider data")
}

fn is_auth_error<T>(envelope: &Envelope<T>) -> bool {
    !envelope.is_success()
        && (envelope.code == Some(401)
            || envelope.error_code.as_deref().is_some_and(|code| {
                matches!(
                    code,
                    "CLIENT_AUTH_NOT_AUTHENTICATED" | "CLIENT_AUTH_TOKEN_EXPIRED"
                )
            }))
}

fn state_with_stale_snapshot(
    mut state: CloudRuntimeState,
    error_message: String,
) -> CloudRuntimeState {
    state.entitlement.status = super::status::CloudRuntimeCategory::StaleSnapshot
        .as_str()
        .to_string();
    state.entitlement.message =
        "Cached Codex++ Cloud state is stale; retry bootstrap to confirm entitlement.".to_string();
    state.entitlement.action_hint = "retry".to_string();
    state.diagnostics.last_error_code = Some("CLIENT_STALE_BOOTSTRAP_SNAPSHOT".to_string());
    state.diagnostics.last_error_message = Some(redact_string(&error_message));
    state
}
