package filament

// Brands lists manufacturer names as shown in Bambu Studio / Cloud filament presets.
var Brands = []string{
	"Bambu Lab",
	"Generic",
	"PolyLite",
	"PolyTerra",
	"eSUN",
	"Overture",
	"Fiberon",
	"Polymaker",
	"Sunlu",
	"Elegoo",
	"Creality",
	"Prusament",
}

// Materials lists filament types as shown in Bambu Studio / Cloud presets.
var Materials = []string{
	"PLA",
	"PLA Basic",
	"PLA Matte",
	"PLA Silk",
	"PLA-CF",
	"PLA Tough",
	"PETG",
	"PETG HF",
	"PETG-CF",
	"PETG Translucent",
	"ABS",
	"ABS-GF",
	"ASA",
	"ASA-CF",
	"TPU",
	"TPU 95A",
	"TPU 95A HF",
	"PC",
	"PC-CF",
	"PA",
	"PA-CF",
	"PA6-CF",
	"PAHT-CF",
	"PVA",
	"Support",
	"HIPS",
}

// ColorOptions are warehouse color choices (RU primary aliases from Catalog).
func ColorOptions() []string {
	out := make([]string, 0, len(Catalog))
	for _, family := range Catalog {
		if len(family.Aliases) > 0 {
			out = append(out, family.Aliases[0])
		}
	}
	return out
}

func IsKnownBrand(raw string) bool {
	n := normalizeBrandToken(raw)
	if n == "" {
		return false
	}
	if n == "bambu" {
		n = "bambu lab"
	}
	for _, b := range Brands {
		if normalizeBrandToken(b) == n {
			return true
		}
	}
	return false
}

func IsKnownMaterial(raw string) bool {
	n := normalizeMaterialToken(raw)
	if n == "" {
		return false
	}
	for _, m := range Materials {
		if normalizeMaterialToken(m) == n {
			return true
		}
	}
	// Allow short base types that match catalog prefixes (PLA, PETG, …).
	bases := []string{"pla", "petg", "abs", "asa", "tpu", "pc", "pa", "pva", "hips", "support"}
	for _, b := range bases {
		if n == b || hasPrefixToken(n, b) {
			return true
		}
	}
	return false
}

func IsKnownColor(raw string) bool {
	return len(ExpandColorAliases(raw)) > 0 && familyMatchesAny(raw)
}

func familyMatchesAny(raw string) bool {
	token := normalizeColorToken(raw)
	if token == "" {
		return false
	}
	for _, family := range Catalog {
		if familyMatches(family, token) {
			return true
		}
	}
	return false
}

func normalizeBrandToken(raw string) string {
	return normalizeColorToken(raw) // reuse lower/trim/fields
}

func normalizeMaterialToken(raw string) string {
	return normalizeColorToken(raw)
}

func hasPrefixToken(value, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	if value[:len(prefix)] != prefix {
		return false
	}
	if len(value) == len(prefix) {
		return true
	}
	next := value[len(prefix)]
	return next == ' ' || next == '-' || next == '_'
}
