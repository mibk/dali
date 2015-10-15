package dali

import "testing"

func TestToUnderscore(t *testing.T) {
	tests := []struct {
		val      string
		expected string
	}{
		{"ID", "id"},
		{"userID", "user_id"},
		{"OldBookID", "old_book_id"},
		{"Old_BookID", "old_book_id"},
		{"ŽlutáČára", "žlutá_čára"},
		{"UIDCode", "uid_code"},
	}

	for _, test := range tests {
		if got := ToUnderscore(test.val); got != test.expected {
			t.Errorf("got %v, want %v", got, test.expected)
		}
	}
}
