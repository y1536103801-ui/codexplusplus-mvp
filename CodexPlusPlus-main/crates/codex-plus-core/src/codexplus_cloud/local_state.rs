use std::collections::hash_map::DefaultHasher;
use std::fs;
use std::hash::{Hash, Hasher};
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

use super::api::{
    Announcement, BootstrapSnapshot, DeviceRegisterRequest, Envelope, FeatureFlagSnapshot,
    ModelSummary, UsageSnapshot, VersionPolicy, normalize_base_url,
};
use super::redaction::{redact_string, redact_url_query_secrets};
use super::status::{CloudRuntimeCategory, runtime_category_from_service_status};
use crate::settings::BackendSettings;

const CLOUD_STATE_DIR: &str = "codexplus-cloud";
const SESSION_FILE: &str = "session.json";
const DEVICE_FILE: &str = "device.json";
const BOOTSTRAP_SNAPSHOT_FILE: &str = "bootstrap_snapshot.json";

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct CloudSession {
    pub base_url: String,
    pub user_id: String,
    pub user_label: Option<String>,
    pub access_token: String,
    pub expires_at: Option<String>,
    pub last_login_at: String,
    #[serde(default)]
    pub pending_2fa_token: Option<String>,
    #[serde(default)]
    pub pending_2fa_user_label: Option<String>,
    #[serde(default)]
    pub pending_handoff: Option<CloudPendingHandoff>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CloudPendingHandoff {
    pub session_token: String,
    pub poll_token: String,
    pub authorize_url: String,
    pub verification_code: Option<String>,
    pub expires_at: String,
    pub poll_interval_seconds: u64,
    pub started_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CloudDevice {
    pub device_id: String,
    pub device_name: String,
    pub platform: String,
    pub first_seen_at: String,
    #[serde(default)]
    pub last_registered_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CloudBootstrapSnapshotFile {
    pub snapshot_version: String,
    pub config_version: String,
    pub fetched_at: String,
    pub envelope: Envelope<BootstrapSnapshot>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudRuntimeState {
    pub connection: CloudConnectionState,
    pub device: CloudDeviceState,
    pub entitlement: CloudEntitlementState,
    pub usage: CloudUsageState,
    pub provider: CloudProviderState,
    pub diagnostics: CloudDiagnosticsState,
    pub models: Vec<ModelSummary>,
    pub feature_flags: Option<FeatureFlagSnapshot>,
    pub announcements: Vec<Announcement>,
    pub version_policy: Option<VersionPolicy>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudConnectionState {
    pub base_url: String,
    pub authenticated: bool,
    pub user_label: Option<String>,
    pub token_expires_at: Option<String>,
    pub pending_two_factor: bool,
    pub pending_two_factor_user_label: Option<String>,
    pub pending_browser_handoff: bool,
    pub browser_handoff_authorize_url: Option<String>,
    pub browser_handoff_verification_code: Option<String>,
    pub browser_handoff_expires_at: Option<String>,
    pub browser_handoff_poll_interval_seconds: Option<u64>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudDeviceState {
    pub device_id: Option<String>,
    pub device_name: Option<String>,
    pub status: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudEntitlementState {
    pub status: String,
    pub service_status: String,
    pub message: String,
    pub message_key: Option<String>,
    pub action_hint: String,
    pub action_type: Option<String>,
    pub action_label: Option<String>,
    pub action_copy_key: Option<String>,
    pub action_url: Option<String>,
    pub retryable: bool,
    pub plan_name: Option<String>,
    pub expires_at: Option<String>,
    pub renewal_hint: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudUsageState {
    pub period: Option<String>,
    pub used: Option<String>,
    pub limit: Option<String>,
    pub unit: Option<String>,
    pub balance_display: Option<String>,
    pub period_usage_display: Option<String>,
    pub rate_limit_state: Option<String>,
    pub action_type: Option<String>,
    pub action_label: Option<String>,
    pub action_copy_key: Option<String>,
    pub action_url: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudProviderState {
    pub managed_provider_id: String,
    pub display_name: String,
    pub configured: bool,
    pub active: bool,
    pub base_url: Option<String>,
    pub default_model: Option<String>,
    pub has_api_key: bool,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CloudDiagnosticsState {
    pub last_error_code: Option<String>,
    pub last_error_message: Option<String>,
    pub last_bootstrap_at: Option<String>,
    pub config_version: Option<String>,
    pub snapshot_version: Option<String>,
}

#[derive(Debug, Clone)]
pub struct CloudLocalStore {
    root: PathBuf,
}

impl Default for CloudLocalStore {
    fn default() -> Self {
        Self::new(default_cloud_state_dir())
    }
}

impl CloudLocalStore {
    pub fn new(root: PathBuf) -> Self {
        Self { root }
    }

    pub fn root(&self) -> &Path {
        &self.root
    }

    pub fn load_session(&self) -> anyhow::Result<Option<CloudSession>> {
        read_optional_json(&self.root.join(SESSION_FILE))
    }

    pub fn save_session(&self, session: &CloudSession) -> anyhow::Result<()> {
        write_json(&self.root.join(SESSION_FILE), session)
    }

    pub fn clear_session(&self) -> anyhow::Result<()> {
        match fs::remove_file(self.root.join(SESSION_FILE)) {
            Ok(()) => Ok(()),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(error) => Err(error.into()),
        }
    }

    pub fn load_device(&self) -> anyhow::Result<Option<CloudDevice>> {
        read_optional_json(&self.root.join(DEVICE_FILE))
    }

    pub fn save_device(&self, device: &CloudDevice) -> anyhow::Result<()> {
        write_json(&self.root.join(DEVICE_FILE), device)
    }

    pub fn ensure_device(&self, preferred_name: Option<&str>) -> anyhow::Result<CloudDevice> {
        if let Some(mut device) = self.load_device()? {
            if let Some(name) = preferred_name
                .map(str::trim)
                .filter(|name| !name.is_empty())
            {
                device.device_name = name.to_string();
                self.save_device(&device)?;
            }
            return Ok(device);
        }
        let device = CloudDevice {
            device_id: generate_device_id(),
            device_name: preferred_name
                .map(str::trim)
                .filter(|name| !name.is_empty())
                .unwrap_or("Codex++ Desktop")
                .to_string(),
            platform: platform_name().to_string(),
            first_seen_at: now_utc_string(),
            last_registered_at: None,
        };
        self.save_device(&device)?;
        Ok(device)
    }

    pub fn save_snapshot(
        &self,
        envelope: &Envelope<BootstrapSnapshot>,
    ) -> anyhow::Result<CloudBootstrapSnapshotFile> {
        let snapshot = envelope
            .data
            .as_ref()
            .map(|data| CloudBootstrapSnapshotFile {
                snapshot_version: data.version_policy.snapshot_version.clone(),
                config_version: data.version_policy.config_version.clone(),
                fetched_at: now_utc_string(),
                envelope: envelope.clone(),
            })
            .unwrap_or_else(|| CloudBootstrapSnapshotFile {
                snapshot_version: String::new(),
                config_version: String::new(),
                fetched_at: now_utc_string(),
                envelope: envelope.clone(),
            });
        write_json(&self.root.join(BOOTSTRAP_SNAPSHOT_FILE), &snapshot)?;
        Ok(snapshot)
    }

    pub fn load_snapshot(&self) -> anyhow::Result<Option<CloudBootstrapSnapshotFile>> {
        read_optional_json(&self.root.join(BOOTSTRAP_SNAPSHOT_FILE))
    }

    pub fn state(&self, settings: &BackendSettings) -> CloudRuntimeState {
        let session = self.load_session().ok().flatten();
        let device = self.load_device().ok().flatten();
        let snapshot = self.load_snapshot().ok().flatten();
        state_from_parts(
            session.as_ref(),
            device.as_ref(),
            snapshot.as_ref(),
            settings,
        )
    }
}

pub fn default_cloud_state_dir() -> PathBuf {
    crate::paths::default_app_state_dir().join(CLOUD_STATE_DIR)
}

pub fn session_from_login(
    base_url: &str,
    user_id: &str,
    user_label: Option<String>,
    access_token: &str,
    expires_at: Option<String>,
) -> CloudSession {
    CloudSession {
        base_url: normalize_base_url(base_url),
        user_id: user_id.to_string(),
        user_label,
        access_token: access_token.to_string(),
        expires_at,
        last_login_at: now_utc_string(),
        pending_2fa_token: None,
        pending_2fa_user_label: None,
        pending_handoff: None,
    }
}

pub fn session_from_pending_2fa(
    base_url: &str,
    user_label: Option<String>,
    temp_token: &str,
) -> CloudSession {
    CloudSession {
        base_url: normalize_base_url(base_url),
        user_id: String::new(),
        user_label: user_label.clone(),
        access_token: String::new(),
        expires_at: None,
        last_login_at: now_utc_string(),
        pending_2fa_token: Some(temp_token.to_string()),
        pending_2fa_user_label: user_label,
        pending_handoff: None,
    }
}

pub fn session_from_pending_handoff(base_url: &str, handoff: CloudPendingHandoff) -> CloudSession {
    CloudSession {
        base_url: normalize_base_url(base_url),
        user_id: String::new(),
        user_label: None,
        access_token: String::new(),
        expires_at: None,
        last_login_at: now_utc_string(),
        pending_2fa_token: None,
        pending_2fa_user_label: None,
        pending_handoff: Some(handoff),
    }
}

pub fn device_register_request(device: &CloudDevice) -> DeviceRegisterRequest {
    DeviceRegisterRequest {
        device_id: device.device_id.clone(),
        platform: device.platform.clone(),
        app_version: crate::version::VERSION.to_string(),
        codex_version: None,
        last_seen_at: now_utc_string(),
    }
}

pub fn mark_device_registered(device: &mut CloudDevice) {
    device.last_registered_at = Some(now_utc_string());
}

pub fn state_with_usage(
    base_state: CloudRuntimeState,
    usage: Option<&Envelope<UsageSnapshot>>,
) -> CloudRuntimeState {
    let Some(usage) = usage.and_then(|envelope| envelope.data.as_ref()) else {
        return base_state;
    };
    CloudRuntimeState {
        usage: CloudUsageState {
            period: usage
                .period_usage
                .get("period")
                .and_then(|value| value.as_str())
                .map(ToString::to_string),
            used: usage
                .period_usage
                .get("used")
                .map(|value| value.to_string()),
            limit: usage
                .period_usage
                .get("limit")
                .map(|value| value.to_string()),
            unit: usage
                .period_usage
                .get("unit")
                .and_then(|value| value.as_str())
                .map(ToString::to_string),
            balance_display: usage
                .balance_summary
                .as_object()
                .and_then(|object| json_string_from_keys(object, &["balance_display", "display"])),
            period_usage_display: usage
                .period_usage
                .as_object()
                .and_then(|object| json_string_from_keys(object, &["usage_display", "display"])),
            rate_limit_state: Some(usage.rate_limit_state.clone()),
            action_type: usage
                .renew_action
                .get("action_type")
                .and_then(|value| value.as_str())
                .map(ToString::to_string),
            action_label: usage
                .renew_action
                .get("label")
                .and_then(|value| value.as_str())
                .map(ToString::to_string),
            action_copy_key: usage
                .renew_action
                .get("action_copy_key")
                .and_then(|value| value.as_str())
                .map(ToString::to_string),
            action_url: usage
                .renew_action
                .get("url")
                .and_then(|value| value.as_str())
                .map(redact_string),
        },
        ..base_state
    }
}

fn json_string_from_keys(
    object: &serde_json::Map<String, serde_json::Value>,
    keys: &[&str],
) -> Option<String> {
    keys.iter()
        .find_map(|key| object.get(*key).and_then(|value| value.as_str()))
        .map(ToString::to_string)
}

fn state_from_parts(
    session: Option<&CloudSession>,
    device: Option<&CloudDevice>,
    snapshot: Option<&CloudBootstrapSnapshotFile>,
    settings: &BackendSettings,
) -> CloudRuntimeState {
    let active_profile = settings
        .relay_profiles
        .iter()
        .find(|profile| profile.id == super::provider_writer::MANAGED_PROVIDER_ID);
    let snapshot_data = snapshot.and_then(|snapshot| snapshot.envelope.data.as_ref());
    let authenticated = session.is_some_and(|session| !session.access_token.trim().is_empty());
    let pending_two_factor = session
        .and_then(|session| session.pending_2fa_token.as_ref())
        .is_some_and(|token| !token.trim().is_empty());
    let pending_browser_handoff = session
        .and_then(|session| session.pending_handoff.as_ref())
        .is_some_and(|handoff| {
            !handoff.session_token.trim().is_empty() && !handoff.poll_token.trim().is_empty()
        });
    let category = if !authenticated {
        CloudRuntimeCategory::NeedsLogin
    } else {
        snapshot_data
            .map(|data| runtime_category_from_service_status(&data.service.status))
            .unwrap_or_else(|| {
                let snapshot_auth_error = snapshot
                    .and_then(|snapshot| snapshot.envelope.error_code.as_deref())
                    .is_some_and(|code| {
                        matches!(
                            code,
                            "CLIENT_AUTH_NOT_AUTHENTICATED" | "CLIENT_AUTH_TOKEN_EXPIRED"
                        )
                    });
                if snapshot_auth_error {
                    CloudRuntimeCategory::NeedsLogin
                } else {
                    CloudRuntimeCategory::Unknown
                }
            })
    };
    let service_status = if authenticated {
        snapshot_data
            .map(|data| data.service.status.clone())
            .unwrap_or_else(|| category.as_str().to_string())
    } else {
        category.as_str().to_string()
    };
    let primary_action = snapshot_data.and_then(|data| {
        data.plan
            .commerce_action
            .as_ref()
            .or(Some(&data.usage.renew_action))
    });

    CloudRuntimeState {
        connection: CloudConnectionState {
            base_url: session
                .map(|session| redact_url_query_secrets(&session.base_url))
                .unwrap_or_default(),
            authenticated,
            user_label: session.and_then(|session| session.user_label.clone()),
            token_expires_at: session.and_then(|session| session.expires_at.clone()),
            pending_two_factor,
            pending_two_factor_user_label: session
                .and_then(|session| session.pending_2fa_user_label.clone()),
            pending_browser_handoff,
            browser_handoff_authorize_url: session
                .and_then(|session| session.pending_handoff.as_ref())
                .map(|handoff| redact_url_query_secrets(&handoff.authorize_url)),
            browser_handoff_verification_code: session
                .and_then(|session| session.pending_handoff.as_ref())
                .and_then(|handoff| handoff.verification_code.clone()),
            browser_handoff_expires_at: session
                .and_then(|session| session.pending_handoff.as_ref())
                .map(|handoff| handoff.expires_at.clone()),
            browser_handoff_poll_interval_seconds: session
                .and_then(|session| session.pending_handoff.as_ref())
                .map(|handoff| handoff.poll_interval_seconds),
        },
        device: CloudDeviceState {
            device_id: device.map(|device| device.device_id.clone()),
            device_name: device.map(|device| device.device_name.clone()),
            status: snapshot_data
                .map(|data| data.device.status.clone())
                .unwrap_or_else(|| "unknown".to_string()),
        },
        entitlement: CloudEntitlementState {
            status: category.as_str().to_string(),
            service_status,
            message: snapshot_data
                .map(|data| data.service.message.clone())
                .unwrap_or_else(|| {
                    if pending_two_factor {
                        "Enter your 2FA code to finish signing in.".to_string()
                    } else if pending_browser_handoff {
                        "Complete browser authorization to finish signing in.".to_string()
                    } else {
                        "Please sign in to Codex++ Cloud.".to_string()
                    }
                }),
            message_key: snapshot_data.and_then(|data| data.service.message_key.clone()),
            action_hint: snapshot_data
                .map(|data| data.service.action_hint.clone())
                .unwrap_or_else(|| "sign_in".to_string()),
            action_type: primary_action.map(|action| action.action_type.clone()),
            action_label: primary_action.map(|action| action.label.clone()),
            action_copy_key: primary_action.and_then(|action| action.action_copy_key.clone()),
            action_url: primary_action
                .and_then(|action| action.url.clone())
                .map(|url| redact_string(&url)),
            retryable: snapshot_data
                .map(|data| data.service.retryable)
                .unwrap_or(true),
            plan_name: snapshot_data.map(|data| data.plan.name.clone()),
            expires_at: snapshot_data.and_then(|data| data.plan.expires_at.clone()),
            renewal_hint: snapshot_data
                .and_then(|data| data.plan.renew_url.clone())
                .map(|url| redact_string(&url)),
        },
        usage: CloudUsageState {
            period: None,
            used: None,
            limit: None,
            unit: None,
            balance_display: snapshot_data.map(|data| data.usage.balance_display.clone()),
            period_usage_display: snapshot_data.map(|data| data.usage.period_usage_display.clone()),
            rate_limit_state: snapshot_data.map(|data| data.usage.rate_limit_state.clone()),
            action_type: snapshot_data.map(|data| data.usage.renew_action.action_type.clone()),
            action_label: snapshot_data.map(|data| data.usage.renew_action.label.clone()),
            action_copy_key: snapshot_data
                .and_then(|data| data.usage.renew_action.action_copy_key.clone()),
            action_url: snapshot_data
                .and_then(|data| data.usage.renew_action.url.clone())
                .map(|url| redact_string(&url)),
        },
        provider: CloudProviderState {
            managed_provider_id: super::provider_writer::MANAGED_PROVIDER_ID.to_string(),
            display_name: super::provider_writer::MANAGED_PROVIDER_NAME.to_string(),
            configured: active_profile.is_some(),
            active: settings.active_relay_id == super::provider_writer::MANAGED_PROVIDER_ID,
            base_url: snapshot_data
                .map(|data| redact_url_query_secrets(&data.provider.gateway_base_url)),
            default_model: snapshot_data.map(|data| data.provider.default_model.clone()),
            has_api_key: snapshot_data
                .and_then(|data| data.provider.api_key.as_ref())
                .is_some_and(|key| !key.trim().is_empty())
                || active_profile.is_some_and(profile_has_api_key),
        },
        diagnostics: CloudDiagnosticsState {
            last_error_code: snapshot_data.and_then(|data| data.service.error_code.clone()),
            last_error_message: snapshot.and_then(|snapshot| {
                (snapshot.envelope.status == "error").then(|| snapshot.envelope.message.clone())
            }),
            last_bootstrap_at: snapshot.map(|snapshot| snapshot.fetched_at.clone()),
            config_version: snapshot.map(|snapshot| snapshot.config_version.clone()),
            snapshot_version: snapshot.map(|snapshot| snapshot.snapshot_version.clone()),
        },
        models: snapshot_data
            .map(|data| data.models.clone())
            .unwrap_or_default(),
        feature_flags: snapshot_data.map(|data| data.feature_flags.clone()),
        announcements: snapshot_data
            .map(|data| data.announcements.clone())
            .unwrap_or_default(),
        version_policy: snapshot_data.map(|data| data.version_policy.clone()),
    }
}

fn profile_has_api_key(profile: &crate::settings::RelayProfile) -> bool {
    !profile.api_key.trim().is_empty() || !profile.auth_contents.trim().is_empty()
}

fn read_optional_json<T>(path: &Path) -> anyhow::Result<Option<T>>
where
    T: for<'de> Deserialize<'de>,
{
    let contents = match fs::read_to_string(path) {
        Ok(contents) => contents,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error.into()),
    };
    match serde_json::from_str(&contents) {
        Ok(parsed) => Ok(Some(parsed)),
        Err(error) => {
            super::redaction::append_redacted_diagnostic(
                "codexplus_cloud.local_state.parse_failed",
                serde_json::json!({
                    "path": path.to_string_lossy(),
                    "error": error.to_string(),
                }),
            )
            .ok();
            Ok(None)
        }
    }
}

fn write_json(path: &Path, value: impl Serialize) -> anyhow::Result<()> {
    let bytes = serde_json::to_vec_pretty(&value)?;
    crate::settings::atomic_write(path, &bytes)
}

fn generate_device_id() -> String {
    let now = now_millis();
    let stack_value = 0usize;
    let mut hasher = DefaultHasher::new();
    now.hash(&mut hasher);
    std::process::id().hash(&mut hasher);
    (&stack_value as *const usize as usize).hash(&mut hasher);
    format!("dev_{now:x}_{:016x}", hasher.finish())
}

fn platform_name() -> &'static str {
    if cfg!(target_os = "windows") {
        "windows"
    } else if cfg!(target_os = "macos") {
        "macos"
    } else if cfg!(target_os = "linux") {
        "linux"
    } else {
        "unknown"
    }
}

pub fn now_utc_string() -> String {
    let seconds = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as i64;
    format_unix_seconds_utc(seconds)
}

fn now_millis() -> u128 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis()
}

fn format_unix_seconds_utc(seconds: i64) -> String {
    let days = seconds.div_euclid(86_400);
    let seconds_of_day = seconds.rem_euclid(86_400);
    let (year, month, day) = civil_from_days(days);
    let hour = seconds_of_day / 3_600;
    let minute = (seconds_of_day % 3_600) / 60;
    let second = seconds_of_day % 60;
    format!("{year:04}-{month:02}-{day:02}T{hour:02}:{minute:02}:{second:02}Z")
}

fn civil_from_days(days_since_epoch: i64) -> (i64, i64, i64) {
    let z = days_since_epoch + 719_468;
    let era = if z >= 0 { z } else { z - 146_096 } / 146_097;
    let doe = z - era * 146_097;
    let yoe = (doe - doe / 1_460 + doe / 36_524 - doe / 146_096) / 365;
    let y = yoe + era * 400;
    let doy = doe - (365 * yoe + yoe / 4 - yoe / 100);
    let mp = (5 * doy + 2) / 153;
    let day = doy - (153 * mp + 2) / 5 + 1;
    let month = mp + if mp < 10 { 3 } else { -9 };
    let year = y + if month <= 2 { 1 } else { 0 };
    (year, month, day)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::settings::{RelayProfile, SettingsStore};

    #[test]
    fn local_state_missing_files_returns_safe_defaults() {
        let temp = tempfile::tempdir().unwrap();
        let store = CloudLocalStore::new(temp.path().join("cloud"));
        let settings = BackendSettings::default();
        let state = store.state(&settings);

        assert!(!state.connection.authenticated);
        assert_eq!(state.entitlement.status, "needs_login");
        assert!(!state.provider.has_api_key);
    }

    #[test]
    fn device_id_is_stable_after_first_generation() {
        let temp = tempfile::tempdir().unwrap();
        let store = CloudLocalStore::new(temp.path().join("cloud"));
        let first = store.ensure_device(Some("A")).unwrap();
        let second = store.ensure_device(Some("B")).unwrap();

        assert_eq!(first.device_id, second.device_id);
        assert_eq!(second.device_name, "B");
    }

    #[test]
    fn cloud_state_exposes_has_api_key_without_key_text() {
        let temp = tempfile::tempdir().unwrap();
        let settings_path = temp.path().join("settings.json");
        let store = SettingsStore::new(settings_path);
        let settings = BackendSettings {
            relay_profiles: vec![RelayProfile {
                id: super::super::provider_writer::MANAGED_PROVIDER_ID.to_string(),
                relay_mode: crate::settings::RelayMode::PureApi,
                auth_contents: r#"{"OPENAI_API_KEY":"test-secret"}"#.to_string(),
                ..RelayProfile::default()
            }],
            active_relay_id: super::super::provider_writer::MANAGED_PROVIDER_ID.to_string(),
            ..BackendSettings::default()
        };
        store.save(&settings).unwrap();
        let cloud = CloudLocalStore::new(temp.path().join("cloud"));
        let state = cloud.state(&store.load().unwrap());
        let text = serde_json::to_string(&state).unwrap();

        assert!(state.provider.has_api_key);
        assert!(!text.contains("test-secret"));
    }

    #[test]
    fn pending_2fa_session_exposes_pending_state_without_temp_token() {
        let temp = tempfile::tempdir().unwrap();
        let cloud = CloudLocalStore::new(temp.path().join("cloud"));
        let session = session_from_pending_2fa(
            "https://api.example.test",
            Some("u***@example.test".to_string()),
            "temporary-login-token",
        );
        cloud.save_session(&session).unwrap();

        let state = cloud.state(&BackendSettings::default());
        let text = serde_json::to_string(&state).unwrap();

        assert!(!state.connection.authenticated);
        assert!(state.connection.pending_two_factor);
        assert!(!state.connection.pending_browser_handoff);
        assert_eq!(
            state.connection.pending_two_factor_user_label.as_deref(),
            Some("u***@example.test")
        );
        assert_eq!(state.entitlement.status, "needs_login");
        assert!(!text.contains("temporary-login-token"));
    }

    #[test]
    fn pending_handoff_session_exposes_only_browser_safe_fields() {
        let temp = tempfile::tempdir().unwrap();
        let cloud = CloudLocalStore::new(temp.path().join("cloud"));
        let session = session_from_pending_handoff(
            "https://api.example.test",
            CloudPendingHandoff {
                session_token: "session-secret".to_string(),
                poll_token: "poll-secret".to_string(),
                authorize_url: "https://app.example.test/auth/desktop/authorize?session_token=session-secret&verification_code=123456".to_string(),
                verification_code: Some("123456".to_string()),
                expires_at: "2026-06-16T00:15:00Z".to_string(),
                poll_interval_seconds: 2,
                started_at: "2026-06-16T00:00:00Z".to_string(),
            },
        );
        cloud.save_session(&session).unwrap();

        let state = cloud.state(&BackendSettings::default());
        let text = serde_json::to_string(&state).unwrap();

        assert!(!state.connection.authenticated);
        assert!(state.connection.pending_browser_handoff);
        assert_eq!(
            state
                .connection
                .browser_handoff_verification_code
                .as_deref(),
            Some("123456")
        );
        assert!(
            state
                .connection
                .browser_handoff_authorize_url
                .as_deref()
                .unwrap_or_default()
                .contains("session_token=session-secret")
        );
        assert!(
            !state
                .connection
                .browser_handoff_authorize_url
                .as_deref()
                .unwrap_or_default()
                .contains("poll-secret")
        );
        assert!(!text.contains("poll-secret"));
    }

    #[test]
    fn state_projects_backend_action_fields_without_provider_key() {
        let temp = tempfile::tempdir().unwrap();
        let cloud = CloudLocalStore::new(temp.path().join("cloud"));
        let session = session_from_login(
            "https://api.example.test",
            "42",
            Some("buyer@example.test".to_string()),
            "jwt-secret-token",
            None,
        );
        cloud.save_session(&session).unwrap();
        let envelope: Envelope<BootstrapSnapshot> = serde_json::from_str(
            r#"{
              "code": 0,
              "status": "success",
              "message": "ok",
              "data": {
                "service": {"status":"available","message":"Ready","message_key":"service.available","action_hint":"none","retryable":true,"support_url":null,"error_code":null},
                "provider": {"provider_id":"codex-plus-cloud","display_name":"Codex++ Cloud","gateway_base_url":"https://api.example/v1","auth_mode":"user_side_api_key","api_key":"sk-live-secret","key_summary":{"key_id":"k","masked_key":"sk-...secret","created_at":"2026-06-16T00:00:00Z","last_used_at":null},"default_model":"gpt-5-codex"},
                "plan": {"plan_id":"starter","name":"Starter","status":"active","expires_at":null,"renew_url":null,"commerce_action":{"action_type":"manage_plan","message_key":"plan.starter.active","action_copy_key":"action.manage_plan","label":"Manage plan","url":"https://billing.example.test?token=secret"}},
                "models": [],
                "usage": {"balance_display":"100 credits remaining","low_balance":false,"period_usage_display":"0 credits used today","rate_limit_state":"normal","renew_action":{"action_type":"manage_plan","message_key":"usage.manage_plan","action_copy_key":"action.manage_plan","label":"Manage","url":null}},
                "feature_flags": {"advanced_provider_config":false,"install_assistant":true,"new_user_tutorial":true,"model_selector":true,"diagnostic_export":true,"announcements":true,"force_update_prompt":false,"strict_device_enforcement":true},
                "announcements": [],
                "version_policy": {"config_version":"cfg","snapshot_version":"snap","refresh_ttl_seconds":300,"force_refresh":false,"minimum_client_version":"0.1.0"},
                "device": {"device_id":"dev","status":"active","message":"ok","snapshot_version":"snap"}
              }
            }"#,
        )
        .unwrap();
        cloud.save_snapshot(&envelope).unwrap();

        let state = cloud.state(&BackendSettings::default());
        let text = serde_json::to_string(&state).unwrap();

        assert_eq!(
            state.entitlement.message_key.as_deref(),
            Some("service.available")
        );
        assert_eq!(
            state.entitlement.action_type.as_deref(),
            Some("manage_plan")
        );
        assert_eq!(
            state.entitlement.action_copy_key.as_deref(),
            Some("action.manage_plan")
        );
        assert_eq!(
            state.entitlement.action_url.as_deref(),
            Some("https://billing.example.test?token=[REDACTED]")
        );
        assert_eq!(state.usage.action_label.as_deref(), Some("Manage"));
        assert!(
            state
                .feature_flags
                .as_ref()
                .unwrap()
                .strict_device_enforcement
        );
        assert!(state.provider.has_api_key);
        assert!(!text.contains("sk-live-secret"));
        assert!(!text.contains("jwt-secret-token"));
    }

    #[test]
    fn unix_epoch_formats_as_utc() {
        assert_eq!(format_unix_seconds_utc(0), "1970-01-01T00:00:00Z");
    }
}
