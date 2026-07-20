use serde::{Deserialize, Serialize};
use std::{
    env, fs,
    path::{Path, PathBuf},
    process::{Command, Stdio},
    thread,
    time::Duration,
};

pub(crate) const OFFICIAL_CODEX_INSTALLER_URL: &str =
    "https://persistent.oaistatic.com/codex-app-prod/ChatGPT.dmg";

const MACOS_CODEX_INSTALL_SCRIPT: &str = r#"
set -eu
url="$CODEXPPP_CODEX_DESKTOP_INSTALLER_URL"
case "$url" in
  https://persistent.oaistatic.com/codex-app-prod/ChatGPT.dmg) ;;
  *) exit 20 ;;
esac
tmp="$(/usr/bin/mktemp -d "${TMPDIR:-/tmp}/codexppp-codex-install.XXXXXX")"
mount="$tmp/mount"
dmg="$tmp/ChatGPT.dmg"
mkdir -p "$mount"
mounted=0
cleanup() {
  if [ "$mounted" -eq 1 ]; then /usr/bin/hdiutil detach "$mount" -quiet >/dev/null 2>&1 || true; fi
  rm -rf -- "$tmp"
}
trap cleanup EXIT HUP INT TERM
/usr/bin/curl --fail --location --retry 2 --connect-timeout 20 --max-time 1200 --silent --show-error "$url" --output "$dmg"
/usr/bin/hdiutil attach -nobrowse -readonly -mountpoint "$mount" "$dmg" >/dev/null
mounted=1
source_app="$mount/ChatGPT.app"
test -d "$source_app"
test -x "$source_app/Contents/MacOS/ChatGPT"
/usr/bin/codesign --verify --deep --strict "$source_app"
/usr/sbin/spctl --assess --type execute "$source_app"
signature="$(/usr/bin/codesign -dv --verbose=4 "$source_app" 2>&1)"
printf '%s\n' "$signature" | /usr/bin/grep -Eq '^Identifier=com\.openai\.'
printf '%s\n' "$signature" | /usr/bin/grep -Eq '^TeamIdentifier=.+$'
applications="$HOME/Applications"
test -n "$HOME"
mkdir -p "$applications"
dest="$applications/ChatGPT.app"
case "$dest" in
  "$HOME"/Applications/ChatGPT.app) ;;
  *) exit 21 ;;
esac
staging="$applications/.ChatGPT.codexppp-new-$$.app"
rm -rf -- "$staging"
/usr/bin/ditto "$source_app" "$staging"
/usr/bin/codesign --verify --deep --strict "$staging"
/usr/sbin/spctl --assess --type execute "$staging"
rm -rf -- "$dest"
mv "$staging" "$dest"
"#;

const MACOS_DESKTOP_UPDATE_HELPER_SCRIPT: &str = r#"
set -eu
parent_pid="$CODEXPPP_UPDATE_PARENT_PID"
dmg="$CODEXPPP_UPDATE_INSTALLER"
current_app="$CODEXPPP_UPDATE_CURRENT_APP"
temp_dir="$CODEXPPP_UPDATE_TEMP_DIR"
case "$current_app" in
  /Applications/Codex+++.app|"$HOME"/Applications/Codex+++.app) ;;
  *) exit 30 ;;
esac
for _ in $(/usr/bin/seq 1 180); do
  if ! /bin/kill -0 "$parent_pid" >/dev/null 2>&1; then break; fi
  /bin/sleep 0.5
done
if /bin/kill -0 "$parent_pid" >/dev/null 2>&1; then exit 31; fi
mount="$temp_dir/mount"
mkdir -p "$mount"
mounted=0
cleanup() {
  if [ "$mounted" -eq 1 ]; then /usr/bin/hdiutil detach "$mount" -quiet >/dev/null 2>&1 || true; fi
  rm -rf -- "$temp_dir"
}
trap cleanup EXIT HUP INT TERM
/usr/bin/hdiutil attach -nobrowse -readonly -mountpoint "$mount" "$dmg" >/dev/null
mounted=1
source_app="$mount/Codex+++.app"
test -d "$source_app"
/usr/bin/codesign --verify --deep --strict "$source_app"
/usr/sbin/spctl --assess --type execute "$source_app"
signature="$(/usr/bin/codesign -dv --verbose=4 "$source_app" 2>&1)"
printf '%s\n' "$signature" | /usr/bin/grep -Eq '^Identifier=com\.codexppp\.desktop$'
printf '%s\n' "$signature" | /usr/bin/grep -Eq '^TeamIdentifier=.+$'
parent="$(/usr/bin/dirname "$current_app")"
test -w "$parent"
staging="$parent/.CodexPPP-update-$$.app"
rm -rf -- "$staging"
/usr/bin/ditto "$source_app" "$staging"
/usr/bin/codesign --verify --deep --strict "$staging"
/usr/sbin/spctl --assess --type execute "$staging"
rm -rf -- "$current_app"
mv "$staging" "$current_app"
/usr/bin/open "$current_app"
"#;

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct InstallerMarker {
    app_version: String,
    installer_identity: String,
}

pub(crate) struct UpdateCheck {
    pub(crate) installed_version: String,
    pub(crate) update_available: bool,
}

pub(crate) fn application_support_root() -> Result<PathBuf, Box<dyn std::error::Error>> {
    Ok(home_dir()?.join("Library").join("Application Support"))
}

pub(crate) fn machine_id() -> Option<String> {
    let output = Command::new("/usr/sbin/ioreg")
        .args(["-rd1", "-c", "IOPlatformExpertDevice"])
        .stdin(Stdio::null())
        .stderr(Stdio::null())
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .find_map(|line| {
            if !line.contains("IOPlatformUUID") {
                return None;
            }
            let (_, value) = line.split_once('=')?;
            let value = value.trim().trim_matches('"');
            (!value.is_empty()).then(|| value.to_string())
        })
        .filter(|value| !value.is_empty())
}

pub(crate) fn installed_app() -> Option<PathBuf> {
    let mut candidates = Vec::new();
    if let Ok(home) = home_dir() {
        candidates.push(home.join("Applications").join("ChatGPT.app"));
    }
    candidates.push(PathBuf::from("/Applications/ChatGPT.app"));
    candidates.into_iter().find(|path| app_layout_valid(path))
}

pub(crate) fn app_ready() -> bool {
    installed_app().is_some_and(|path| app_trusted(&path))
}

pub(crate) fn app_version(path: &Path) -> Option<String> {
    let plist = path.join("Contents").join("Info.plist");
    for key in ["CFBundleVersion", "CFBundleShortVersionString"] {
        let output = Command::new("/usr/libexec/PlistBuddy")
            .args(["-c", &format!("Print :{key}")])
            .arg(&plist)
            .stdin(Stdio::null())
            .stderr(Stdio::null())
            .output()
            .ok()?;
        if !output.status.success() {
            continue;
        }
        let version = sanitize_line(&String::from_utf8_lossy(&output.stdout));
        if version_is_readable(&version) {
            return Some(version);
        }
    }
    None
}

pub(crate) fn check_update() -> Result<UpdateCheck, String> {
    let app = installed_app().ok_or_else(|| "codex_not_detected".to_string())?;
    if !app_trusted(&app) {
        return Err("codex_signature_invalid".to_string());
    }
    let installed_version =
        app_version(&app).ok_or_else(|| "codex_version_unreadable".to_string())?;
    let installer_identity =
        remote_installer_identity().ok_or_else(|| "codex_version_check_failed".to_string())?;
    let marker = read_installer_marker();
    let update_available = marker.is_none_or(|marker| {
        marker.app_version != installed_version || marker.installer_identity != installer_identity
    });
    Ok(UpdateCheck {
        installed_version,
        update_available,
    })
}

pub(crate) fn install_or_update() -> Result<(), String> {
    ensure_supported_system()?;
    let status = Command::new("/bin/sh")
        .args(["-c", MACOS_CODEX_INSTALL_SCRIPT])
        .env(
            "CODEXPPP_CODEX_DESKTOP_INSTALLER_URL",
            OFFICIAL_CODEX_INSTALLER_URL,
        )
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map_err(|_| "codex_install_failed".to_string())?;
    if !status.success() || !app_ready() {
        return Err(if installed_app().is_some() {
            "codex_update_failed".to_string()
        } else {
            "codex_install_failed".to_string()
        });
    }
    record_current_install()
}

pub(crate) fn launch(path: &Path) -> Result<(), String> {
    if !app_trusted(path) {
        return Err("codex_signature_invalid".to_string());
    }
    let status = Command::new("/usr/bin/open")
        .arg(path)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map_err(|_| "codex_command_launch_failed".to_string())?;
    if !status.success() {
        return Err("codex_command_launch_failed".to_string());
    }
    for _ in 0..20 {
        if !process_ids().is_empty() {
            return Ok(());
        }
        thread::sleep(Duration::from_millis(250));
    }
    Err("codex_activation_process_exited".to_string())
}

pub(crate) fn process_ids() -> Vec<u32> {
    let Ok(output) = Command::new("/bin/ps")
        .args(["-axo", "pid=,command="])
        .stdin(Stdio::null())
        .stderr(Stdio::null())
        .output()
    else {
        return Vec::new();
    };
    if !output.status.success() {
        return Vec::new();
    }
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .filter_map(process_id_from_ps_line)
        .collect()
}

pub(crate) fn stop_process(pid: u32) -> Result<(), String> {
    if pid == 0 {
        return Ok(());
    }
    let status = Command::new("/bin/kill")
        .args(["-TERM", &pid.to_string()])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map_err(|_| "codex_stop_failed".to_string())?;
    if !status.success() {
        return Err("codex_stop_failed".to_string());
    }
    for _ in 0..20 {
        if !process_ids().contains(&pid) {
            return Ok(());
        }
        thread::sleep(Duration::from_millis(100));
    }
    let status = Command::new("/bin/kill")
        .args(["-KILL", &pid.to_string()])
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .map_err(|_| "codex_stop_failed".to_string())?;
    status
        .success()
        .then_some(())
        .ok_or_else(|| "codex_stop_failed".to_string())
}

pub(crate) fn common_codex_command() -> Option<PathBuf> {
    let home = home_dir().ok()?;
    [
        home.join(".local").join("bin").join("codex"),
        home.join("bin").join("codex"),
    ]
    .into_iter()
    .find(|path| path.is_file())
}

pub(crate) fn desktop_update_helper(
    installer: &Path,
    update_dir: &Path,
    parent_pid: u32,
) -> Result<Command, String> {
    let current_exe = env::current_exe().map_err(|_| "update_install_failed".to_string())?;
    let current_app = current_exe
        .ancestors()
        .find(|path| path.file_name().is_some_and(|name| name == "Codex+++.app"))
        .ok_or_else(|| "update_install_failed".to_string())?;
    let mut helper = Command::new("/bin/sh");
    helper
        .args(["-c", MACOS_DESKTOP_UPDATE_HELPER_SCRIPT])
        .env("CODEXPPP_UPDATE_PARENT_PID", parent_pid.to_string())
        .env("CODEXPPP_UPDATE_INSTALLER", installer)
        .env("CODEXPPP_UPDATE_CURRENT_APP", current_app)
        .env("CODEXPPP_UPDATE_TEMP_DIR", update_dir)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null());
    Ok(helper)
}

pub(crate) fn version_is_readable(value: &str) -> bool {
    let trimmed = value.trim();
    !trimmed.is_empty()
        && trimmed.len() <= 96
        && trimmed
            .chars()
            .all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '.' | '-' | '_' | '+'))
}

fn home_dir() -> Result<PathBuf, Box<dyn std::error::Error>> {
    let home = env::var_os("HOME").ok_or("HOME is not set")?;
    let path = PathBuf::from(home);
    if !path.is_absolute() {
        return Err("HOME must be absolute".into());
    }
    Ok(path)
}

fn ensure_supported_system() -> Result<(), String> {
    let output = Command::new("/usr/bin/sw_vers")
        .arg("-productVersion")
        .stdin(Stdio::null())
        .stderr(Stdio::null())
        .output()
        .map_err(|_| "codex_install_unavailable".to_string())?;
    let major = String::from_utf8_lossy(&output.stdout)
        .trim()
        .split('.')
        .next()
        .and_then(|part| part.parse::<u32>().ok())
        .unwrap_or_default();
    if !output.status.success() || major < 14 {
        return Err("codex_macos_unsupported".to_string());
    }
    Ok(())
}

fn app_layout_valid(path: &Path) -> bool {
    path.is_dir()
        && path.join("Contents").join("Info.plist").is_file()
        && path
            .join("Contents")
            .join("MacOS")
            .join("ChatGPT")
            .is_file()
}

fn app_trusted(path: &Path) -> bool {
    if !app_layout_valid(path) || app_version(path).is_none() {
        return false;
    }
    let codesign = Command::new("/usr/bin/codesign")
        .args(["--verify", "--deep", "--strict"])
        .arg(path)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .is_ok_and(|status| status.success());
    let gatekeeper = Command::new("/usr/sbin/spctl")
        .args(["--assess", "--type", "execute"])
        .arg(path)
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status()
        .is_ok_and(|status| status.success());
    let identity = Command::new("/usr/bin/codesign")
        .args(["-dv", "--verbose=4"])
        .arg(path)
        .stdin(Stdio::null())
        .output()
        .ok()
        .map(|output| String::from_utf8_lossy(&output.stderr).into_owned())
        .is_some_and(|details| {
            details
                .lines()
                .any(|line| line.starts_with("Identifier=com.openai."))
                && details
                    .lines()
                    .any(|line| line.starts_with("TeamIdentifier=") && line.len() > 15)
        });
    codesign && gatekeeper && identity
}

fn remote_installer_identity() -> Option<String> {
    let output = Command::new("/usr/bin/curl")
        .args([
            "--fail",
            "--location",
            "--head",
            "--connect-timeout",
            "15",
            "--max-time",
            "30",
            "--silent",
            "--show-error",
            OFFICIAL_CODEX_INSTALLER_URL,
        ])
        .stdin(Stdio::null())
        .stderr(Stdio::null())
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    installer_identity_from_headers(&String::from_utf8_lossy(&output.stdout))
}

fn installer_identity_from_headers(headers: &str) -> Option<String> {
    let etag = headers
        .lines()
        .filter_map(|line| line.split_once(':'))
        .filter(|(name, _)| name.trim().eq_ignore_ascii_case("etag"))
        .map(|(_, value)| sanitize_line(value))
        .filter(|value| !value.is_empty())
        .next_back();
    if let Some(etag) = etag {
        return Some(format!("etag:{etag}"));
    }
    let last_modified = last_header_value(headers, "last-modified")?;
    let content_length = last_header_value(headers, "content-length")?;
    Some(format!("modified:{last_modified};length:{content_length}"))
}

fn last_header_value(headers: &str, name: &str) -> Option<String> {
    headers
        .lines()
        .filter_map(|line| line.split_once(':'))
        .filter(|(header, _)| header.trim().eq_ignore_ascii_case(name))
        .map(|(_, value)| sanitize_line(value))
        .filter(|value| !value.is_empty())
        .next_back()
}

fn marker_path() -> Option<PathBuf> {
    Some(
        application_support_root()
            .ok()?
            .join("Codex+++")
            .join("codex-official-installer.json"),
    )
}

fn read_installer_marker() -> Option<InstallerMarker> {
    let content = fs::read_to_string(marker_path()?).ok()?;
    serde_json::from_str(&content).ok()
}

fn record_current_install() -> Result<(), String> {
    let app = installed_app().ok_or_else(|| "codex_install_failed".to_string())?;
    let app_version = app_version(&app).ok_or_else(|| "codex_version_unreadable".to_string())?;
    let installer_identity =
        remote_installer_identity().ok_or_else(|| "codex_version_check_failed".to_string())?;
    let path = marker_path().ok_or_else(|| "codex_version_check_failed".to_string())?;
    let parent = path
        .parent()
        .ok_or_else(|| "codex_version_check_failed".to_string())?;
    fs::create_dir_all(parent).map_err(|_| "codex_version_check_failed".to_string())?;
    let marker = InstallerMarker {
        app_version,
        installer_identity,
    };
    fs::write(
        path,
        format!(
            "{}\n",
            serde_json::to_string_pretty(&marker)
                .map_err(|_| "codex_version_check_failed".to_string())?
        ),
    )
    .map_err(|_| "codex_version_check_failed".to_string())
}

fn process_id_from_ps_line(line: &str) -> Option<u32> {
    let trimmed = line.trim();
    let split = trimmed.find(char::is_whitespace)?;
    let pid = trimmed[..split].parse::<u32>().ok()?;
    let command = trimmed[split..].trim();
    (pid != 0
        && command.contains("/ChatGPT.app/Contents/MacOS/ChatGPT")
        && !command.contains("/ChatGPT Classic.app/"))
    .then_some(pid)
}

fn sanitize_line(value: &str) -> String {
    value
        .chars()
        .filter(|ch| !ch.is_control())
        .collect::<String>()
        .trim()
        .chars()
        .take(256)
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_remote_installer_identity_without_hardcoded_version() {
        let headers = "HTTP/2 200\r\ncontent-length: 42\r\netag: 0xabc\r\nlast-modified: now\r\n";
        assert_eq!(
            installer_identity_from_headers(headers).as_deref(),
            Some("etag:0xabc")
        );
        let fallback = "HTTP/2 200\r\ncontent-length: 42\r\nlast-modified: now\r\n";
        assert_eq!(
            installer_identity_from_headers(fallback).as_deref(),
            Some("modified:now;length:42")
        );
    }

    #[test]
    fn only_matches_the_new_chatgpt_desktop_main_process() {
        assert_eq!(
            process_id_from_ps_line(
                "123 /Users/test/Applications/ChatGPT.app/Contents/MacOS/ChatGPT"
            ),
            Some(123)
        );
        assert_eq!(
            process_id_from_ps_line("124 /Applications/ChatGPT Classic.app/Contents/MacOS/ChatGPT"),
            None
        );
    }
}
