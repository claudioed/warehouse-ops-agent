package usecases

import "testing"

// TestSanitizeForLog_StripsCRLF proves the CWE-117 log-injection fix: a
// crafted value containing CR/LF (the classic forged-log-line payload) has
// both stripped, so an untrusted orderId/pathId or an upstream service's
// error text can never be rendered as if it were a second, separate log
// line.
func TestSanitizeForLog_StripsCRLF(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no control chars", "order-abc-123", "order-abc-123"},
		{"embedded newline", "order-abc\n[INFO] forged line", "order-abc[INFO] forged line"},
		{"embedded CRLF", "order-abc\r\n[INFO] forged line", "order-abc[INFO] forged line"},
		{"embedded bare CR", "order-abc\rforged", "order-abcforged"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeForLog(tc.in); got != tc.want {
				t.Fatalf("sanitizeForLog(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
