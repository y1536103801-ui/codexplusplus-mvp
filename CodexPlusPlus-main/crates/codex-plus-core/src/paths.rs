use std::path::PathBuf;
use std::sync::{Mutex, OnceLock};

const APP_STATE_DIR: &str = ".codex-session-delete";
const SETTINGS_FILE: &str = "settings.json";
const LATEST_STATUS_FILE: &str = "latest-status.json";
const DIAGNOSTIC_LOG_FILE: &str = "codex-plus.log";

pub fn default_app_state_dir() -> PathBuf {
    if let Some(state_dir) = std::env::var_os("CODEX_PLUS_STATE_DIR").map(PathBuf::from) {
        return state_dir;
    }

    if let Some(home_dir) = std::env::var_os("USERPROFILE")
        .or_else(|| std::env::var_os("HOME"))
        .map(PathBuf::from)
    {
        return home_dir.join(APP_STATE_DIR);
    }

    if let Some(home_dir) = directories::BaseDirs::new().map(|dirs| dirs.home_dir().to_path_buf()) {
        return home_dir.join(APP_STATE_DIR);
    }

    PathBuf::from(APP_STATE_DIR)
}

pub fn default_settings_path() -> PathBuf {
    if let Some(path) = settings_path_for_tests() {
        return path;
    }
    default_app_state_dir().join(SETTINGS_FILE)
}

pub fn default_latest_status_path() -> PathBuf {
    default_app_state_dir().join(LATEST_STATUS_FILE)
}

pub fn default_diagnostic_log_path() -> PathBuf {
    default_app_state_dir().join(DIAGNOSTIC_LOG_FILE)
}

fn settings_path_for_tests() -> Option<PathBuf> {
    SETTINGS_PATH_FOR_TESTS
        .get_or_init(|| Mutex::new(None))
        .lock()
        .ok()
        .and_then(|path| path.clone())
}

static SETTINGS_PATH_FOR_TESTS: OnceLock<Mutex<Option<PathBuf>>> = OnceLock::new();

pub fn set_settings_path_for_tests(path: Option<PathBuf>) -> Option<PathBuf> {
    SETTINGS_PATH_FOR_TESTS
        .get_or_init(|| Mutex::new(None))
        .lock()
        .ok()
        .and_then(|mut current| std::mem::replace(&mut *current, path))
}

#[cfg(test)]
mod tests {
    use super::*;

    fn restore_env(name: &str, previous: Option<std::ffi::OsString>) {
        match previous {
            Some(value) => set_env(name, value),
            None => remove_env(name),
        }
    }

    fn set_env<K: AsRef<std::ffi::OsStr>, V: AsRef<std::ffi::OsStr>>(key: K, value: V) {
        unsafe { std::env::set_var(key, value) };
    }

    fn remove_env<K: AsRef<std::ffi::OsStr>>(key: K) {
        unsafe { std::env::remove_var(key) };
    }

    #[test]
    fn default_settings_path_uses_app_state_directory() {
        let path = default_settings_path();

        assert!(path.ends_with(".codex-session-delete/settings.json"));
    }

    #[test]
    fn default_latest_status_path_uses_app_state_directory() {
        let path = default_latest_status_path();

        assert!(path.ends_with(".codex-session-delete/latest-status.json"));
    }

    #[test]
    fn default_diagnostic_log_path_uses_app_state_directory() {
        let path = default_diagnostic_log_path();

        assert!(path.ends_with(".codex-session-delete/codex-plus.log"));
    }

    #[test]
    fn default_app_state_dir_honors_explicit_state_override() {
        let previous_state = std::env::var_os("CODEX_PLUS_STATE_DIR");
        let previous_userprofile = std::env::var_os("USERPROFILE");
        let previous_home = std::env::var_os("HOME");
        let temp = tempfile::tempdir().unwrap();
        let state_dir = temp.path().join("state");

        set_env("CODEX_PLUS_STATE_DIR", &state_dir);
        set_env("USERPROFILE", temp.path().join("profile"));
        set_env("HOME", temp.path().join("home"));
        let resolved = default_app_state_dir();

        restore_env("CODEX_PLUS_STATE_DIR", previous_state);
        restore_env("USERPROFILE", previous_userprofile);
        restore_env("HOME", previous_home);

        assert_eq!(resolved, state_dir);
    }

    #[test]
    fn default_app_state_dir_honors_isolated_userprofile() {
        let previous_state = std::env::var_os("CODEX_PLUS_STATE_DIR");
        let previous_userprofile = std::env::var_os("USERPROFILE");
        let previous_home = std::env::var_os("HOME");
        let temp = tempfile::tempdir().unwrap();
        let profile = temp.path().join("isolated-profile");

        remove_env("CODEX_PLUS_STATE_DIR");
        set_env("USERPROFILE", &profile);
        remove_env("HOME");
        let resolved = default_app_state_dir();

        restore_env("CODEX_PLUS_STATE_DIR", previous_state);
        restore_env("USERPROFILE", previous_userprofile);
        restore_env("HOME", previous_home);

        assert_eq!(resolved, profile.join(".codex-session-delete"));
    }
}
