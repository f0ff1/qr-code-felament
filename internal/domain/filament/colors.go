package filament

import (
	"strings"
)

// ColorFamily maps Bambu Lab marketing color names / hex codes
// to simple warehouse aliases (RU + EN).
//
// Sources (open):
//   - Bambu Lab PLA Basic / PLA Matte Hex Code tables
//   - BambuStudio resources/profiles/BBL/filament/filaments_color_codes.json
//   - Community palette: github.com/Dadequate/bambu-lab-filament-colors
type ColorFamily struct {
	// Simple names users put on the warehouse spool.
	Aliases []string
	// Official / marketing names from Bambu Cloud / Studio.
	BambuNames []string
	// Hex codes without '#' (6 chars), upper-case.
	Hex []string
}

// Catalog is the matching table used for Cloud ↔ warehouse color sync.
var Catalog = []ColorFamily{
	{
		Aliases:    []string{"чёрный", "черный", "black", "blk"},
		BambuNames: []string{"black", "charcoal", "matte charcoal"},
		Hex:        []string{"000000", "161616", "0A0A0A", "1A1A1A"},
	},
	{
		Aliases:    []string{"белый", "white", "wht"},
		BambuNames: []string{"white", "jade white", "ivory white", "matte ivory white", "bone white", "matte bone white"},
		Hex:        []string{"FFFFFF", "FEFEFE", "F5F5F5", "CBC6B8", "FFFFEE"},
	},
	{
		Aliases:    []string{"красный", "red"},
		BambuNames: []string{"red", "scarlet red", "matte scarlet red", "dark red", "matte dark red", "burgundy red", "brick red", "cherry pink"},
		Hex:        []string{"C12E1F", "DE4343", "BB3D43", "D32941", "B50011", "951E23", "9F332A", "D21B3C"},
	},
	{
		Aliases:    []string{"синий", "blue"},
		BambuNames: []string{"blue", "marine blue", "matte marine blue", "dark blue", "matte dark blue", "sky blue", "matte sky blue", "ice blue", "matte ice blue", "royal blue", "jeans blue", "indigo blue", "navy blue", "azure", "crystal blue"},
		Hex:        []string{"0A2989", "0078BF", "042F56", "56B7E6", "A3D8E1", "2842AD", "6E88BC", "324585", "0C2340", "0A2CA5", "489FDF", "7EB4E1", "0047BB", "40B6E4", "B8CDE9"},
	},
	{
		Aliases:    []string{"зелёный", "зеленый", "green"},
		BambuNames: []string{"bambu green", "mistletoe green", "grass green", "matte grass green", "dark green", "matte dark green", "apple green", "matte apple green", "matcha green", "malachite green", "olive", "teal", "light jade"},
		Hex:        []string{"00AE42", "3F8E43", "61C680", "68724D", "C2E189", "5C9748", "16B08E", "789D4A", "009FA1", "96D8AF", "009BD8"},
	},
	{
		Aliases:    []string{"жёлтый", "желтый", "yellow"},
		BambuNames: []string{"sunflower yellow", "lemon yellow", "matte lemon yellow", "yellow", "tangerine yellow", "mellow yellow", "ochre yellow"},
		Hex:        []string{"FEC600", "F7D959", "F4D53F", "FFC72C", "F5DBAB", "C98935"},
	},
	{
		Aliases:    []string{"оранжевый", "orange"},
		BambuNames: []string{"orange", "pumpkin orange", "mandarin orange", "matte mandarin orange"},
		Hex:        []string{"FF6A13", "FF9016", "F99963", "F74E02", "DC3A27", "FFA500", "FF8C00"},
	},
	{
		Aliases:    []string{"серый", "gray", "grey"},
		BambuNames: []string{"gray", "grey", "light gray", "dark gray", "ash gray", "matte ash gray", "nardo gray", "matte nardo gray", "blue gray", "silver", "lava gray", "titan gray", "quicksilver"},
		Hex:        []string{"8E9089", "D1D3D5", "545454", "9B9EA0", "757575", "5B6579", "A6A9AA", "AFB1AE", "959698", "4D5054", "565656", "9EA2A2", "87909A", "ADB1B2", "515151", "8E8E8E"},
	},
	{
		Aliases:    []string{"розовый", "pink"},
		BambuNames: []string{"hot pink", "sakura pink", "matte sakura pink", "cherry pink", "blaze"},
		Hex:        []string{"F5547C", "E8AFCF", "F5B6CD", "F1AAA8"},
	},
	{
		Aliases:    []string{"фиолетовый", "purple"},
		BambuNames: []string{"indigo purple", "lilac purple", "matte lilac purple", "plum", "matte plum", "iris purple", "violet purple", "lavender", "grape jelly", "mystic magenta"},
		Hex:        []string{"482960", "AE96D4", "950051", "69398E", "583061", "B8ACD6", "D6ABFF", "720062", "8344B0", "AF1685"},
	},
	{
		Aliases:    []string{"коричневый", "brown"},
		BambuNames: []string{"cocoa brown", "latte brown", "matte latte brown", "dark brown", "matte dark brown", "dark chocolate", "matte dark chocolate", "caramel", "matte caramel", "terracotta", "matte terracotta", "desert tan", "matte desert tan", "clay brown", "black walnut", "rosewood", "classic birch", "white oak"},
		Hex:        []string{"6F5034", "D3B7A7", "7D6556", "4D3324", "AE835B", "B15533", "E8DBB7", "5C4738", "995F11", "4F3F24", "4C241C", "918669", "D6CCA3"},
	},
	{
		Aliases:    []string{"голубой", "cyan"},
		BambuNames: []string{"cyan"},
		Hex:        []string{"0086D6", "009BD8"},
	},
	{
		Aliases:    []string{"бежевый", "beige", "cream"},
		BambuNames: []string{"cream", "desert tan", "bone white", "ivory white", "mellow yellow"},
		Hex:        []string{"F9DFB9", "E8DBB7", "CBC6B8", "F5DBAB"},
	},
}

// ExpandColorAliases turns a Cloud hint (Bambu name or hex) into all matching
// warehouse aliases + bambu names for that color family.
func ExpandColorAliases(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	normalized := normalizeColorToken(raw)
	out := []string{normalized}

	for _, family := range Catalog {
		if familyMatches(family, normalized) {
			out = append(out, family.Aliases...)
			out = append(out, family.BambuNames...)
			out = append(out, family.Hex...)
		}
	}
	return uniqueFold(out)
}

func familyMatches(family ColorFamily, token string) bool {
	for _, a := range family.Aliases {
		if token == normalizeColorToken(a) {
			return true
		}
	}
	for _, n := range family.BambuNames {
		nt := normalizeColorToken(n)
		if token == nt || strings.Contains(token, nt) || strings.Contains(nt, token) {
			return true
		}
	}
	hex := stripHex(token)
	if len(hex) >= 6 {
		hex6 := hex[:6]
		for _, h := range family.Hex {
			if hex6 == strings.ToUpper(h) {
				return true
			}
		}
	}
	return false
}

func normalizeColorToken(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	raw = strings.TrimPrefix(raw, "#")
	raw = strings.ReplaceAll(raw, "_", " ")
	raw = strings.ReplaceAll(raw, "-", " ")
	// Drop common prefixes from RFID / Studio labels.
	raw = strings.TrimPrefix(raw, "matte ")
	raw = strings.TrimPrefix(raw, "pla ")
	raw = strings.TrimPrefix(raw, "petg ")
	raw = strings.TrimPrefix(raw, "abs ")
	raw = strings.Join(strings.Fields(raw), " ")
	if hex := stripHex(raw); len(hex) == 6 || len(hex) == 8 {
		return strings.ToUpper(hex)
	}
	return raw
}

func stripHex(raw string) string {
	raw = strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(raw)), "#")
	for _, r := range raw {
		if (r < '0' || r > '9') && (r < 'A' || r > 'F') {
			return ""
		}
	}
	if len(raw) == 8 {
		return raw[:6]
	}
	return raw
}

func uniqueFold(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}
