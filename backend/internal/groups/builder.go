package groups

import (
	"subforme/backend/internal/config"
	"subforme/backend/internal/xui"
)

const (
	defaultHealthcheckURL      = "https://www.gstatic.com/generate_204"
	defaultHealthcheckInterval = 300
)

type ProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies,omitempty"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
	Use      []string `yaml:"use,omitempty"`
}

func Build(cfg config.GroupConfig, nodes []xui.Node, groupNodes map[string][]string, groupModes map[string]string, selectedProviders []string) []ProxyGroup {
	defs := cfg.Groups
	if len(defs) == 0 {
		defs = defaultGroups(cfg)
	}

	providerSet := map[string]bool{}
	for _, p := range selectedProviders {
		providerSet[p] = true
	}

	out := make([]ProxyGroup, 0, len(defs))
	for _, g := range defs {
		if g.Provider != "" && !providerSet[g.Provider] {
			continue
		}
		groupType := g.Type
		if override := groupModes[g.Name]; override != "" {
			groupType = override
		}
		pg := ProxyGroup{Name: g.Name, Type: groupType}
		if g.URL != "" {
			pg.URL = g.URL
		}
		if g.Interval > 0 {
			pg.Interval = g.Interval
		}
		if needsHealthcheck(groupType) {
			if pg.URL == "" {
				if cfg.Healthcheck.URL != "" {
					pg.URL = cfg.Healthcheck.URL
				} else {
					pg.URL = defaultHealthcheckURL
				}
			}
			if pg.Interval <= 0 {
				if cfg.Healthcheck.IntervalSeconds > 0 {
					pg.Interval = cfg.Healthcheck.IntervalSeconds
				} else {
					pg.Interval = defaultHealthcheckInterval
				}
			}
		}
		if g.Provider != "" {
			pg.Use = []string{g.Provider}
		} else {
			pg.Proxies = groupNodes[g.Name]
		}
		out = append(out, pg)
	}
	return out
}

func needsHealthcheck(groupType string) bool {
	switch groupType {
	case "url-test", "fallback", "load-balance":
		return true
	default:
		return false
	}
}

func defaultGroups(cfg config.GroupConfig) []config.GroupDef {
	defs := []config.GroupDef{
		{Name: cfg.GroupNames.Proxy, Type: "select"},
		{Name: cfg.GroupNames.Auto, Type: "url-test", URL: cfg.Healthcheck.URL, Interval: cfg.Healthcheck.IntervalSeconds},
	}
	for region := range cfg.Regions {
		defs = append(defs, config.GroupDef{Name: region, Type: "select"})
	}
	if cfg.GroupNames.Other != "" {
		defs = append(defs, config.GroupDef{Name: cfg.GroupNames.Other, Type: "select"})
	}
	return defs
}
