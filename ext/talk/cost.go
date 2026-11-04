package talk

import (
	"maps"
	"slices"
	"strings"
	"writer/locales"
)

func costText(costs map[string]float64) string {
	var out []string
	for _, cur := range slices.Sorted(maps.Keys(costs)) {
		if v := costs[cur]; v < 0.01 {
			out = append(out, locales.AiCostUnder(cur+"0.01"))
		} else {
			if cur == "$" {
				out = append(out, locales.SettingsSpentCost(v))
			} else {
				out = append(out, locales.SettingsCostCurrency(cur, v))
			}
		}
	}
	return strings.Join(out, " + ")
}
