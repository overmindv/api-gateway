package httpx

import "testing"

// TestClampLimit проверяет нормализацию limit пагинации.
func TestClampLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "negative", input: -5, want: DefaultLimit},
		{name: "zero", input: 0, want: DefaultLimit},
		{name: "default", input: DefaultLimit, want: DefaultLimit},
		{name: "lower bound", input: MinLimit, want: MinLimit},
		{name: "typical", input: 15, want: 15},
		{name: "upper bound", input: MaxLimit, want: MaxLimit},
		{name: "over limit", input: MaxLimit + 1, want: MaxLimit},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := ClampLimit(test.input); got != test.want {
				t.Fatalf("ClampLimit(%d) = %d, want %d", test.input, got, test.want)
			}
		})
	}
}
