package util

import "testing"

func TestChunks(t *testing.T) {
	got := Chunks([]int{1, 2, 3, 4, 5}, 2)
	if len(got) != 3 {
		t.Fatalf("Chunks([1..5], 2) = %d chunks, want 3", len(got))
	}
	if len(got[0]) != 2 || got[0][0] != 1 || got[0][1] != 2 {
		t.Errorf("chunk[0] = %v, want [1,2]", got[0])
	}
	if len(got[2]) != 1 || got[2][0] != 5 {
		t.Errorf("chunk[2] = %v, want [5]", got[2])
	}

	// Edge cases
	if res := Chunks([]int{}, 3); len(res) != 0 {
		t.Errorf("Chunks(empty, 3) = %v, want empty", res)
	}
	if res := Chunks([]int{1}, 0); res != nil {
		t.Errorf("Chunks([1], 0) = %v, want nil", res)
	}
}
