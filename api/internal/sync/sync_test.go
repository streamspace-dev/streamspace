package sync

import "testing"

func TestShouldRecloneRepository(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "invalid git repo",
			err:  testError("git fetch failed: exit status 128\nOutput: fatal: not a git repository"),
			want: true,
		},
		{
			name: "missing remote repo",
			err:  testError("git clone failed: repository not found"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRecloneRepository(tt.err); got != tt.want {
				t.Fatalf("expected %t, got %t", tt.want, got)
			}
		})
	}
}

type testError string

func (e testError) Error() string {
	return string(e)
}
