package cli

import "testing"

func TestExtractNormalizedCommands(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		expects []string
	}{
		{
			name:    "single command with path and number",
			input:   "go test ./internal/store -run '^TestExpandPath$'",
			expects: []string{"go test <path> -run ^TestExpandPath$"},
		},
		{
			name:    "chained commands split",
			input:   "git status && go test ./...",
			expects: []string{"git status", "go test <path>"},
		},
		{
			name:    "dynamic values normalized",
			input:   "git show 9f7e8a1 && cat /tmp/out.log",
			expects: []string{"git show <id>", "cat <path>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNormalizedCommands(tt.input)
			if len(got) != len(tt.expects) {
				t.Fatalf("expected %d commands, got %d (%v)", len(tt.expects), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expects[i] {
					t.Fatalf("command[%d]: expected %q, got %q", i, tt.expects[i], got[i])
				}
			}
		})
	}
}

func TestEstimateOutputTokens(t *testing.T) {
	if estimateOutputTokens("") != 0 {
		t.Fatalf("expected empty output to estimate as 0")
	}

	if estimateOutputTokens("abcd") != 1 {
		t.Fatalf("expected 4 chars to estimate as 1 token")
	}

	if estimateOutputTokens("abcdefgh") != 2 {
		t.Fatalf("expected 8 chars to estimate as 2 tokens")
	}
}
