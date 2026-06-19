use serde::Serialize;
use serde_json::{Map, Value};

const REDACTED: &str = "[REDACTED]";

pub fn redact_json_value(value: &Value) -> Value {
    match value {
        Value::Object(map) => Value::Object(redact_json_object(map)),
        Value::Array(items) => Value::Array(items.iter().map(redact_json_value).collect()),
        Value::String(value) => Value::String(redact_string(value)),
        other => other.clone(),
    }
}

pub fn redact_serialize(value: impl Serialize) -> Value {
    let value = serde_json::to_value(value).unwrap_or_else(|error| {
        serde_json::json!({
            "serialization_error": error.to_string()
        })
    });
    redact_json_value(&value)
}

pub fn redact_string(value: &str) -> String {
    let value = redact_url_query_secrets(value);
    let value = redact_authorization_fragments(&value);
    let value = redact_sk_tokens(&value);
    redact_jwt_like_tokens(&value)
}

pub fn redact_url_query_secrets(value: &str) -> String {
    let Some(question_index) = value.find('?') else {
        return value.to_string();
    };
    let (before_query, query_and_fragment) = value.split_at(question_index + 1);
    let (query, fragment) = match query_and_fragment.find('#') {
        Some(fragment_index) => query_and_fragment.split_at(fragment_index),
        None => (query_and_fragment, ""),
    };
    let redacted_query = query
        .split('&')
        .map(|part| {
            let Some((key, _)) = part.split_once('=') else {
                return part.to_string();
            };
            if is_url_secret_key(key) {
                format!("{key}={REDACTED}")
            } else {
                part.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("&");
    format!("{before_query}{redacted_query}{fragment}")
}

pub fn append_redacted_diagnostic(event: &str, detail: impl Serialize) -> std::io::Result<()> {
    crate::diagnostic_log::append_diagnostic_log(event, redact_serialize(detail))
}

fn redact_json_object(map: &Map<String, Value>) -> Map<String, Value> {
    map.iter()
        .map(|(key, value)| {
            let redacted = if is_sensitive_json_key(key) {
                Value::String(REDACTED.to_string())
            } else {
                redact_json_value(value)
            };
            (key.clone(), redacted)
        })
        .collect()
}

fn is_sensitive_json_key(key: &str) -> bool {
    let normalized = key
        .chars()
        .filter(|ch| *ch != '_' && *ch != '-' && *ch != ' ')
        .flat_map(char::to_lowercase)
        .collect::<String>();
    matches!(
        normalized.as_str(),
        "apikey"
            | "accesstoken"
            | "refreshtoken"
            | "idtoken"
            | "polltoken"
            | "sessiontoken"
            | "authorization"
            | "password"
            | "bearer"
            | "experimentalbearertoken"
            | "openaiapikey"
    )
}

fn is_url_secret_key(key: &str) -> bool {
    let key = key.trim().to_ascii_lowercase();
    matches!(
        key.as_str(),
        "key" | "token" | "access_token" | "api_key" | "poll_token"
    )
}

fn redact_authorization_fragments(value: &str) -> String {
    let lower = value.to_ascii_lowercase();
    if !lower.contains("authorization") && !lower.contains("bearer ") {
        return value.to_string();
    }

    let mut output = Vec::new();
    for token in value.split_whitespace() {
        let normalized = token
            .trim_matches(|ch: char| !ch.is_ascii_alphanumeric() && ch != '_' && ch != '-')
            .to_ascii_lowercase();
        if normalized == "bearer" {
            output.push(token.to_string());
            continue;
        }
        if output
            .last()
            .is_some_and(|previous| previous.eq_ignore_ascii_case("bearer"))
            || normalized.starts_with("authorization:")
        {
            output.push(REDACTED.to_string());
        } else {
            output.push(token.to_string());
        }
    }
    output.join(" ")
}

fn redact_sk_tokens(value: &str) -> String {
    redact_tokens_by_prefix(value, &["sk-", "sk_"])
}

fn redact_tokens_by_prefix(value: &str, prefixes: &[&str]) -> String {
    let mut output = String::with_capacity(value.len());
    let chars = value.chars().collect::<Vec<_>>();
    let mut index = 0;
    while index < chars.len() {
        let remaining = chars[index..].iter().collect::<String>();
        if prefixes.iter().any(|prefix| remaining.starts_with(prefix)) {
            output.push_str(REDACTED);
            index += remaining
                .chars()
                .take_while(|ch| {
                    ch.is_ascii_alphanumeric() || matches!(*ch, '-' | '_' | '.' | ':' | '/' | '+')
                })
                .count()
                .max(1);
        } else {
            output.push(chars[index]);
            index += 1;
        }
    }
    output
}

fn redact_jwt_like_tokens(value: &str) -> String {
    let mut output = String::with_capacity(value.len());
    let mut current = String::new();
    for ch in value.chars() {
        if ch.is_ascii_alphanumeric() || ch == '.' || ch == '_' || ch == '-' {
            current.push(ch);
            continue;
        }
        flush_jwt_candidate(&mut output, &mut current);
        output.push(ch);
    }
    flush_jwt_candidate(&mut output, &mut current);
    output
}

fn flush_jwt_candidate(output: &mut String, current: &mut String) {
    if current.is_empty() {
        return;
    }
    if looks_like_jwt(current) {
        output.push_str(REDACTED);
    } else {
        output.push_str(current);
    }
    current.clear();
}

fn looks_like_jwt(value: &str) -> bool {
    let parts = value.split('.').collect::<Vec<_>>();
    parts.len() == 3
        && parts.iter().all(|part| part.len() >= 8)
        && parts.iter().all(|part| {
            part.chars()
                .all(|ch| ch.is_ascii_alphanumeric() || ch == '_' || ch == '-')
        })
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn redacts_sensitive_json_keys_but_preserves_has_api_key() {
        let value = redact_json_value(&json!({
            "api_key": "secret-token",
            "apiKey": "other-secret-token",
            "poll_token": "desktop-poll-secret",
            "session_token": "pending-session-secret",
            "has_api_key": true,
            "nested": { "Authorization": "Bearer abcdefgh.abcdefgh.abcdefgh" }
        }));

        assert_eq!(value["api_key"], json!("[REDACTED]"));
        assert_eq!(value["apiKey"], json!("[REDACTED]"));
        assert_eq!(value["poll_token"], json!("[REDACTED]"));
        assert_eq!(value["session_token"], json!("[REDACTED]"));
        assert_eq!(value["has_api_key"], json!(true));
        assert_eq!(value["nested"]["Authorization"], json!("[REDACTED]"));
    }

    #[test]
    fn redacts_token_patterns_and_url_query_values() {
        let text = redact_string(
            "Bearer abcdefgh.abcdefgh.abcdefgh https://x.test/v1?api_key=live-token&token=provider-token-123456&poll_token=desktop-secret&ok=1",
        );

        assert!(!text.contains("abcdefgh.abcdefgh.abcdefgh"));
        assert!(!text.contains("live-token"));
        assert!(!text.contains("provider-token-123456"));
        assert!(!text.contains("desktop-secret"));
        assert!(text.contains("ok=1"));
    }
}
