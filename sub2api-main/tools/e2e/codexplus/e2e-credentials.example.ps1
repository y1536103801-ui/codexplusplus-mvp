# Codex++ 07 E2E credentials example.
#
# This file is intentionally placeholders only. Copy it to a local uncommitted
# env file before filling real values. Do not commit filled copies and do not
# paste real values into chat.
#
# The E2E env-file parser accepts comments, blank lines, and single-quoted
# assignments shaped exactly like:
#   $env:NAME = 'value'

# Non-secret, environment target. Use local/dev/test/staging/sandbox/qa unless -AllowProduction is explicitly approved.
$env:CODEXPLUS_07_E2E_BACKEND_BASE_URL = '<fill-backend-base-url>'
$env:CODEXPLUS_07_E2E_ADMIN_BASE_URL = '<fill-admin-base-url>'
$env:CODEXPLUS_07_E2E_GATEWAY_BASE_URL = '<fill-gateway-base-url>'

# Non-secret local path. Must exist.
$env:CODEXPLUS_07_E2E_MANAGER_BUILD_PATH = 'TODO_MANAGER_BUILD_PATH'

# Secret admin token.
$env:CODEXPLUS_07_E2E_ADMIN_TOKEN = '<fill-admin-jwt-or-test-admin-token>'

# Secret user client tokens.
$env:CODEXPLUS_07_E2E_USER_ACTIVE_TOKEN = '<fill-active-user-client-token>'
$env:CODEXPLUS_07_E2E_USER_NOT_PURCHASED_TOKEN = '<fill-not-purchased-user-client-token>'
$env:CODEXPLUS_07_E2E_USER_EXPIRED_TOKEN = '<fill-expired-user-client-token>'
$env:CODEXPLUS_07_E2E_USER_LOW_BALANCE_TOKEN = '<fill-low-balance-user-client-token>'
$env:CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_TOKEN = '<fill-device-revoked-user-client-token>'
$env:CODEXPLUS_07_E2E_USER_MODEL_DENIED_TOKEN = '<fill-model-denied-user-client-token>'

# Non-secret internal numeric user IDs for admin audit correlation.
$env:CODEXPLUS_07_E2E_USER_ACTIVE_ID = '<fill-active-user-numeric-id>'
$env:CODEXPLUS_07_E2E_USER_NOT_PURCHASED_ID = '<fill-not-purchased-user-numeric-id>'
$env:CODEXPLUS_07_E2E_USER_EXPIRED_ID = '<fill-expired-user-numeric-id>'
$env:CODEXPLUS_07_E2E_USER_LOW_BALANCE_ID = '<fill-low-balance-user-numeric-id>'
$env:CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_ID = '<fill-device-revoked-user-numeric-id>'
$env:CODEXPLUS_07_E2E_USER_MODEL_DENIED_ID = '<fill-model-denied-user-numeric-id>'

# Non-secret stable test device ID.
$env:CODEXPLUS_07_E2E_TEST_DEVICE_ID = 'codexplus-07-e2e-device'

# Non-secret model policy inputs. These values must differ.
$env:CODEXPLUS_07_E2E_ALLOWED_TEST_MODEL = '<fill-allowed-model>'
$env:CODEXPLUS_07_E2E_DENIED_TEST_MODEL = '<fill-denied-model>'

# Secret user-side gateway keys.
$env:CODEXPLUS_07_E2E_USER_ACTIVE_GATEWAY_KEY = '<fill-active-user-gateway-key>'
$env:CODEXPLUS_07_E2E_USER_NOT_PURCHASED_GATEWAY_KEY = '<fill-not-purchased-user-gateway-key>'
$env:CODEXPLUS_07_E2E_USER_EXPIRED_GATEWAY_KEY = '<fill-expired-user-gateway-key>'
$env:CODEXPLUS_07_E2E_USER_LOW_BALANCE_GATEWAY_KEY = '<fill-low-balance-user-gateway-key>'
$env:CODEXPLUS_07_E2E_USER_DEVICE_REVOKED_GATEWAY_KEY = '<fill-device-revoked-user-gateway-key>'
$env:CODEXPLUS_07_E2E_USER_MODEL_DENIED_GATEWAY_KEY = '<fill-model-denied-user-gateway-key>'

# Secret and browser-session scoped. Required only with -AllowBrowserComplete.
$env:CODEXPLUS_07_E2E_BROWSER_AUTH_TOKEN = '<fill-browser-auth-token-only-when-authorized>'

# Secret or sensitive mutating input. Required only with -AllowRedeem.
$env:CODEXPLUS_07_E2E_REDEEM_CODE = '<optional-test-redeem-code-only-when-authorized>'
