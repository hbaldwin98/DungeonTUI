package dice

import "testing"

func TestEvaluateDiceWithModifier(t *testing.T) {
	result, err := Evaluate("d20+5", FixedRNG(17))
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 22 {
		t.Fatalf("total=%d want 22", result.Total)
	}
	if result.Expression != "d20+5" {
		t.Fatalf("expression=%q", result.Expression)
	}
	if got := result.Detail; got != "[17]+5 = 22" {
		t.Fatalf("detail=%q", got)
	}
	if len(result.Rolls) != 1 || result.Rolls[0] != 17 {
		t.Fatalf("rolls=%v", result.Rolls)
	}
}

func TestEvaluateMultipleDice(t *testing.T) {
	result, err := Evaluate("2d6+3", FixedRNG(4, 5))
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 12 {
		t.Fatalf("total=%d want 12", result.Total)
	}
	if result.Detail != "[4+5]+3 = 12" {
		t.Fatalf("detail=%q", result.Detail)
	}
}

func TestEvaluateArithmeticPrecedence(t *testing.T) {
	result, err := Evaluate("10+2*3", FixedRNG())
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 16 {
		t.Fatalf("total=%d want 16", result.Total)
	}
}

func TestExtractSkipsSessionCommands(t *testing.T) {
	found := Extract("look around #location Crypt #random npc #d20+5 #damage 2d6+3")
	if len(found) != 2 {
		t.Fatalf("found=%v", found)
	}
	if found[0].Expression != "d20+5" || found[0].Label != "" {
		t.Fatalf("first=%+v", found[0])
	}
	if found[1].Label != "damage" || found[1].Expression != "2d6+3" {
		t.Fatalf("second=%+v", found[1])
	}
}

func TestEvaluateRejectsJunk(t *testing.T) {
	if _, err := Evaluate("d", FixedRNG(1)); err == nil {
		t.Fatal("expected error")
	}
}
