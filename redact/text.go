package redact

import "regexp"

var (
	sensitiveTextFieldPattern = regexp.MustCompile(`(?i)(["']?)(password|old_pwd|new_pwd|pwd|client_secret|crypto_key|encrypted_key|ciphertext|access_token|refresh_token|captcha_code|captcha_token|verification_code|sms_code|email_code|otp|recovery_code|private_key|authorization|cookie|signature|setup_ticket|mfa_challenge_id|mfa_setup_ticket|webauthn_options_json|otpauth_uri)(["']?)\s*([=:])\s*("[^"]*"|'[^']*'|[^\s,;]+)`)
	textPhonePattern          = regexp.MustCompile(`1[3-9][0-9]{9}`)
	textEmailPattern          = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

// SanitizeText 脱敏自由文本中的常见手机号、邮箱和凭据键值。
func SanitizeText(value string) string {
	if value == "" {
		return value
	}
	value = sensitiveTextFieldPattern.ReplaceAllString(value, `${1}${2}${3}${4}"[REDACTED]"`)
	value = textEmailPattern.ReplaceAllStringFunc(value, func(match string) string {
		return Email(match, 2, false, "*")
	})
	return textPhonePattern.ReplaceAllStringFunc(value, func(match string) string {
		return Mask(match, 3, 4, "*")
	})
}
