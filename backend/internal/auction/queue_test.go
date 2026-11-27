package auction

import "testing"

func TestQueueOptions(t *testing.T) {
	defaults, err := ValidateQueueOptions(QueueOptions{})
	if err != nil || len(defaults.CategoryOrder) != 10 {
		t.Fatalf("defaults: %v %v", defaults, err)
	}
	reordered := append([]string{}, defaults.CategoryOrder...)
	reordered[0], reordered[1] = reordered[1], reordered[0]
	got, err := ValidateQueueOptions(QueueOptions{CategoryOrder: reordered, ShuffleCategories: []string{" capped_batter "}})
	if err != nil || got.CategoryOrder[0] != "CAPPED_BATTER" || got.ShuffleCategories[0] != "CAPPED_BATTER" {
		t.Fatalf("custom order: %v %v", got, err)
	}
	for _, c := range []QueueOptions{{CategoryOrder: []string{"MARQUEE"}}, {ShuffleCategories: []string{"REFERENCE_BATTER"}}, {ShuffleCategories: []string{"MARQUEE", "MARQUEE"}}} {
		if _, err := ValidateQueueOptions(c); err == nil {
			t.Fatalf("accepted invalid options: %v", c)
		}
	}
	reordered[0] = reordered[1]
	if _, err := ValidateQueueOptions(QueueOptions{CategoryOrder: reordered}); err == nil {
		t.Fatal("accepted duplicate order")
	}
}
