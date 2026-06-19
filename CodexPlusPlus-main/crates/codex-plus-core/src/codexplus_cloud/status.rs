use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CloudRuntimeCategory {
    Ready,
    NeedsLogin,
    NotPurchased,
    Expired,
    Limited,
    Disabled,
    DeviceRevoked,
    ModelUnavailable,
    RateLimited,
    GatewayUnhealthy,
    LocalCodexMissing,
    LocalConfigFailed,
    StaleSnapshot,
    Unknown,
}

impl CloudRuntimeCategory {
    pub fn as_str(&self) -> &'static str {
        match self {
            Self::Ready => "ready",
            Self::NeedsLogin => "needs_login",
            Self::NotPurchased => "not_purchased",
            Self::Expired => "expired",
            Self::Limited => "limited",
            Self::Disabled => "disabled",
            Self::DeviceRevoked => "device_revoked",
            Self::ModelUnavailable => "model_unavailable",
            Self::RateLimited => "rate_limited",
            Self::GatewayUnhealthy => "gateway_unhealthy",
            Self::LocalCodexMissing => "local_codex_missing",
            Self::LocalConfigFailed => "local_config_failed",
            Self::StaleSnapshot => "stale_snapshot",
            Self::Unknown => "unknown",
        }
    }
}

pub fn runtime_category_from_service_status(status: &str) -> CloudRuntimeCategory {
    match status {
        "available" => CloudRuntimeCategory::Ready,
        "not_authenticated" => CloudRuntimeCategory::NeedsLogin,
        "not_purchased" => CloudRuntimeCategory::NotPurchased,
        "expired" => CloudRuntimeCategory::Expired,
        "low_balance" => CloudRuntimeCategory::Limited,
        "disabled" => CloudRuntimeCategory::Disabled,
        "device_revoked" => CloudRuntimeCategory::DeviceRevoked,
        "model_unavailable" => CloudRuntimeCategory::ModelUnavailable,
        "rate_limited" => CloudRuntimeCategory::RateLimited,
        "gateway_unhealthy" => CloudRuntimeCategory::GatewayUnhealthy,
        "local_codex_missing" => CloudRuntimeCategory::LocalCodexMissing,
        "local_config_failed" => CloudRuntimeCategory::LocalConfigFailed,
        _ => CloudRuntimeCategory::Unknown,
    }
}

pub fn service_status_is_applyable(status: &str) -> bool {
    matches!(status, "available" | "low_balance" | "rate_limited")
}
