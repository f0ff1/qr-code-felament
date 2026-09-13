package printjob

import (
	"context"
	"path/filepath"
	"strings"

	"filamenttracker/internal/domain/filament"
	printerdomain "filamenttracker/internal/domain/printer"

	"github.com/google/uuid"
)

func normalizeName(raw string) string {
	base := strings.ToLower(strings.TrimSpace(sanitizeUTF8(raw)))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	return strings.Join(strings.Fields(base), " ")
}

func (s *Service) matchResources(ctx context.Context, printer printerdomain.Printer, snap BambuSnapshot) (productID, spoolID uuid.UUID, estimated int, draft bool) {
	draft = true
	products, err := s.productRepo.List(ctx)
	if err == nil {
		fileKey := normalizeName(snap.FileName)
		for _, product := range products {
			nameKey := normalizeName(product.Name)
			if nameKey != "" && (strings.Contains(fileKey, nameKey) || strings.Contains(nameKey, fileKey)) {
				productID = product.ID
				estimated = product.EstimatedWeight
				break
			}
		}
	}

	materialHint, brandHint := parseFilamentHint(snap.MaterialHint, snap.BrandHint)
	colorAliases := colorAliases(snap.ColorHint)

	spools, err := s.spoolRepo.List(ctx)
	if err == nil {
		var bestID uuid.UUID
		bestScore := -1
		for _, spool := range spools {
			if spool.CurrentWeight <= 0 {
				continue
			}
			score := 0
			if materialHint != "" && materialsMatch(string(spool.Material), materialHint) {
				score += 2
			} else if materialHint != "" {
				continue
			}
			if len(colorAliases) > 0 {
				if colorMatches(spool.Color, colorAliases) {
					score += 2
				} else {
					continue
				}
			}
			if brandHint != "" {
				if manufacturersMatch(spool.Manufacturer, brandHint) {
					score += 2
				} else {
					continue
				}
			}
			if printer.DefaultSpoolID != uuid.Nil && spool.ID == printer.DefaultSpoolID {
				score += 1
			}
			if score > bestScore {
				bestScore = score
				bestID = spool.ID
			}
		}
		// Material+color (4) or material+brand (4) is enough; prefer all three (6).
		if bestID != uuid.Nil && bestScore >= 4 {
			spoolID = bestID
		} else if bestID != uuid.Nil && materialHint != "" && len(colorAliases) == 0 && brandHint == "" && bestScore >= 2 {
			spoolID = bestID
		}
	}

	if spoolID == uuid.Nil && printer.DefaultSpoolID != uuid.Nil {
		if spool, err := s.spoolRepo.GetByID(ctx, printer.DefaultSpoolID); err == nil && spool.CurrentWeight > 0 {
			spoolID = printer.DefaultSpoolID
		}
	}

	// Auto-consume when warehouse spool matched, or printer has a default spool.
	if spoolID != uuid.Nil && (materialHint != "" || productID != uuid.Nil || printer.DefaultSpoolID == spoolID) {
		draft = false
	}
	if productID != uuid.Nil && spoolID != uuid.Nil {
		draft = false
	}
	return productID, spoolID, estimated, draft
}

func parseFilamentHint(materialRaw, brandRaw string) (material, brand string) {
	material = normalizeMaterial(materialRaw)
	brand = normalizeBrand(brandRaw)
	if brand == "" {
		brand = extractBrand(materialRaw)
	}
	if brand == "" {
		brand = extractBrand(brandRaw)
	}
	return material, brand
}

func extractBrand(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return ""
	}
	switch {
	case strings.Contains(lower, "bambu lab"), strings.Contains(lower, "bambulab"), strings.HasPrefix(lower, "bambu"):
		return "bambu"
	case strings.Contains(lower, "generic"):
		return "generic"
	case strings.Contains(lower, "polymaker"):
		return "polymaker"
	case strings.Contains(lower, "overture"):
		return "overture"
	case strings.Contains(lower, "esun"):
		return "esun"
	case strings.Contains(lower, "sunlu"):
		return "sunlu"
	default:
		return ""
	}
}

func normalizeBrand(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", " ")
	raw = strings.Join(strings.Fields(raw), " ")
	switch raw {
	case "":
		return ""
	case "bambu lab", "bambulab", "bambu":
		return "bambu"
	case "generic":
		return "generic"
	case "polymaker":
		return "polymaker"
	case "overture":
		return "overture"
	case "esun":
		return "esun"
	case "sunlu":
		return "sunlu"
	default:
		return raw
	}
}

func manufacturersMatch(spoolManufacturer, brandHint string) bool {
	a := normalizeBrand(spoolManufacturer)
	b := normalizeBrand(brandHint)
	if a == "" || b == "" {
		return false
	}
	return a == b
}

func normalizeMaterial(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, "_", " ")
	// Cloud / Handy often send "Generic PLA", "Bambu PLA Matte", etc.
	fields := strings.Fields(raw)
	joined := strings.Join(fields, "")

	switch {
	case strings.Contains(joined, "PETG"):
		return "PETG"
	case strings.Contains(joined, "PLA"):
		return "PLA"
	case strings.Contains(joined, "ABS"):
		return "ABS"
	case strings.Contains(joined, "ASA"):
		return "ASA"
	case strings.Contains(joined, "TPU"):
		return "TPU"
	case strings.Contains(joined, "PC"):
		return "PC"
	case strings.HasPrefix(joined, "PLA"):
		return "PLA"
	default:
		return joined
	}
}

func materialsMatch(spoolMaterial, hint string) bool {
	return strings.EqualFold(normalizeMaterial(spoolMaterial), normalizeMaterial(hint))
}

func colorAliases(raw string) []string {
	return filament.ExpandColorAliases(raw)
}

func colorMatches(spoolColor string, aliases []string) bool {
	spoolColor = strings.ToLower(strings.TrimSpace(spoolColor))
	spoolExpanded := filament.ExpandColorAliases(spoolColor)
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		if spoolColor == alias || strings.Contains(spoolColor, alias) || strings.Contains(alias, spoolColor) {
			return true
		}
		for _, expanded := range spoolExpanded {
			if strings.EqualFold(expanded, alias) {
				return true
			}
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(sanitizeUTF8(v))
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
