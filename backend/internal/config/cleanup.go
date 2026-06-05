package config

import "path/filepath"

type AppCleanupState struct {
	Users          []string
	UsersKnown     bool
	NodeIDs        []string
	NodeIDsKnown   bool
	NodeNames      []string
	NodeNamesKnown bool
	Providers      []string
	ProvidersKnown bool
	Groups         []string
	GroupsKnown    bool
}

func CleanupAppConfigFile(dir string, state AppCleanupState) error {
	var app AppConfig
	path := filepath.Join(dir, "app.yaml")
	if err := readYAML(path, &app); err != nil {
		return err
	}
	cleaned := CleanupAppConfig(app, state)
	return SaveAppConfig(dir, cleaned)
}

func CleanupAppConfig(app AppConfig, state AppCleanupState) AppConfig {
	userSet := stringSet(state.Users)
	nodeIDSet := stringSet(state.NodeIDs)
	nodeNameSet := stringSet(state.NodeNames)
	providerSet := stringSet(state.Providers)
	groupSet := stringSet(state.Groups)
	for _, builtin := range []string{"DIRECT", "REJECT", "GLOBAL", "PASS"} {
		groupSet[builtin] = struct{}{}
	}

	if state.UsersKnown {
		for user := range app.UserModes {
			if _, ok := userSet[user]; !ok {
				delete(app.UserModes, user)
			}
		}
		for user := range app.UserNodes {
			if _, ok := userSet[user]; !ok {
				delete(app.UserNodes, user)
			}
		}
		for user := range app.UserProviders {
			if _, ok := userSet[user]; !ok {
				delete(app.UserProviders, user)
			}
		}
		for user := range app.UserGroupNodes {
			if _, ok := userSet[user]; !ok {
				delete(app.UserGroupNodes, user)
			}
		}
		for user := range app.UserGroupModes {
			if _, ok := userSet[user]; !ok {
				delete(app.UserGroupModes, user)
			}
		}
	}

	for user, nodes := range app.UserNodes {
		filtered := filterKnown(nodes, nodeIDSet, state.NodeIDsKnown)
		if len(filtered) == 0 {
			delete(app.UserNodes, user)
		} else {
			app.UserNodes[user] = filtered
		}
	}
	for user, providers := range app.UserProviders {
		filtered := filterKnown(providers, providerSet, state.ProvidersKnown)
		if len(filtered) == 0 {
			delete(app.UserProviders, user)
		} else {
			app.UserProviders[user] = filtered
		}
	}
	for user, groups := range app.UserGroupNodes {
		for groupName, refs := range groups {
			if _, ok := groupSet[groupName]; !ok && state.GroupsKnown {
				delete(groups, groupName)
				continue
			}
			filtered := refs[:0]
			for _, ref := range refs {
				if keepGroupRef(ref, nodeNameSet, providerSet, groupSet, state) {
					filtered = append(filtered, ref)
				}
			}
			if len(filtered) == 0 {
				delete(groups, groupName)
			} else {
				groups[groupName] = filtered
			}
		}
		if len(groups) == 0 {
			delete(app.UserGroupNodes, user)
		}
	}
	for user, groupModes := range app.UserGroupModes {
		for groupName := range groupModes {
			if _, ok := groupSet[groupName]; !ok && state.GroupsKnown {
				delete(groupModes, groupName)
			}
		}
		if len(groupModes) == 0 {
			delete(app.UserGroupModes, user)
		}
	}
	return app
}

func keepGroupRef(ref string, nodeNames, providers, groups map[string]struct{}, state AppCleanupState) bool {
	checked := false
	if state.NodeNamesKnown {
		checked = true
		if _, ok := nodeNames[ref]; ok {
			return true
		}
	}
	if state.ProvidersKnown {
		checked = true
		if _, ok := providers[ref]; ok {
			return true
		}
	}
	if state.GroupsKnown {
		checked = true
		if _, ok := groups[ref]; ok {
			return true
		}
	}
	return !checked
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func filterKnown(values []string, known map[string]struct{}, authoritative bool) []string {
	if !authoritative {
		return values
	}
	filtered := values[:0]
	for _, value := range values {
		if _, ok := known[value]; ok {
			filtered = append(filtered, value)
		}
	}
	return filtered
}
