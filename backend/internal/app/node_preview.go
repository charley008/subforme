package app

import (
	"context"
	"fmt"
	"strings"

	"subforme/backend/internal/config"
	"subforme/backend/internal/generator"
	"subforme/backend/internal/xui"
	"subforme/backend/pkg/proxyopts"
)

// PreviewManagedNode previews an unsaved draft without changing node or user data.
func (s Service) PreviewManagedNode(user string, draft config.ManagedNode) ([]byte, error) {
	if strings.TrimSpace(user) == "" {
		return nil, fmt.Errorf("请选择用于预览的用户")
	}
	if _, err := proxyopts.Parse(draft.MihomoOptions); err != nil {
		return nil, err
	}
	bundle, err := s.loadBundle()
	if err != nil {
		return nil, err
	}
	var nodes []xui.Node
	if s.DB != nil {
		nodes, err = s.dbResolveUserNodes(user)
	}
	if len(nodes) == 0 {
		nodes, err = s.resolver(bundle.App).ResolveUserNodes(context.Background(), user)
	}
	if err != nil {
		return nil, err
	}
	draft = normalizeManagedNodes([]config.ManagedNode{draft})[0]
	node, ok := managedTemplateNode(nodes, draft)
	if !ok {
		return nil, fmt.Errorf("该用户没有可用于此节点的入站配置，请先在用户页完成入站分配")
	}
	node.Name = draft.Name
	node.Server = draft.Address
	if draft.Port > 0 {
		node.Port = draft.Port
	}
	node.MihomoOptions = draft.MihomoOptions
	return generator.BuildProxyYAML(node)
}
