package filament

import "testing"

func TestIsKnownBrandAndMaterial(t *testing.T) {
	if !IsKnownBrand("Bambu Lab") || !IsKnownBrand("Bambu") || !IsKnownBrand("Generic") {
		t.Fatal("expected common brands")
	}
	if IsKnownBrand("UnknownCorp") {
		t.Fatal("unknown brand must fail")
	}
	if !IsKnownMaterial("PLA") || !IsKnownMaterial("PETG HF") || !IsKnownMaterial("PLA-CF") {
		t.Fatal("expected materials")
	}
	if IsKnownMaterial("Banana") {
		t.Fatal("unknown material must fail")
	}
	if !IsKnownColor("чёрный") || !IsKnownColor("Charcoal") || !IsKnownColor("#000000") {
		t.Fatal("expected colors")
	}
	if IsKnownColor("несуществующийцветxyz") {
		t.Fatal("unknown color must fail")
	}
}
