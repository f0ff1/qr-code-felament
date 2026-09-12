package filament

import "testing"

func TestExpandColorAliasesCharcoalToBlack(t *testing.T) {
	aliases := ExpandColorAliases("Charcoal")
	if !containsFold(aliases, "чёрный") && !containsFold(aliases, "черный") {
		t.Fatalf("Charcoal should map to чёрный, got %v", aliases)
	}
	if !containsFold(aliases, "black") {
		t.Fatalf("Charcoal should map to black, got %v", aliases)
	}
}

func TestExpandColorAliasesHexBlack(t *testing.T) {
	aliases := ExpandColorAliases("000000FF")
	if !containsFold(aliases, "charcoal") || !containsFold(aliases, "black") {
		t.Fatalf("000000FF should expand to charcoal/black, got %v", aliases)
	}
}

func TestExpandColorAliasesScarletRed(t *testing.T) {
	aliases := ExpandColorAliases("Scarlet Red")
	if !containsFold(aliases, "красный") || !containsFold(aliases, "red") {
		t.Fatalf("Scarlet Red should map to красный/red, got %v", aliases)
	}
}

func TestExpandColorAliasesJadeWhite(t *testing.T) {
	aliases := ExpandColorAliases("Jade White")
	if !containsFold(aliases, "белый") || !containsFold(aliases, "white") {
		t.Fatalf("Jade White should map to белый/white, got %v", aliases)
	}
}

func TestWarehouseBlackMatchesCharcoal(t *testing.T) {
	cloud := ExpandColorAliases("Matte Charcoal")
	if !containsFold(cloud, "чёрный") && !containsFold(cloud, "черный") {
		t.Fatalf("warehouse «чёрный» should match Matte Charcoal via aliases %v", cloud)
	}
}

func containsFold(list []string, want string) bool {
	for _, item := range list {
		if equalFoldASCII(item, want) {
			return true
		}
	}
	return false
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		// still allow unicode via simple lower compare
		return normalizeColorToken(a) == normalizeColorToken(b)
	}
	return normalizeColorToken(a) == normalizeColorToken(b)
}
