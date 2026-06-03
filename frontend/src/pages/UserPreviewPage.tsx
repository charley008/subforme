import { useEffect, useMemo, useState } from "react";
import { getJSON, getText, putJSON } from "../lib/api";
import type { ManagedNode, NodePreview, ProviderAddon, UserSummary } from "../lib/types";

type GroupDef = {
  name: string;
  type: string;
  url?: string;
  interval?: number;
  provider?: string;
};

const modeLabel: Record<UserSummary["mode"], string> = {
  whitelist: "默认直连",
  blacklist: "默认代理",
};

type PreviewState = {
  user: string;
  yaml: string;
  nodes: NodePreview[];
};

export function UserPreviewPage() {
  const [users, setUsers] = useState<UserSummary[]>([]);
  const [managedNodes, setManagedNodes] = useState<ManagedNode[]>([]);
  const [providers, setProviders] = useState<ProviderAddon[]>([]);
  const [groupDefs, setGroupDefs] = useState<GroupDef[]>([]);
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [busyUser, setBusyUser] = useState<string | null>(null);

  // Group editor modal state
  const [editingUser, setEditingUser] = useState<UserSummary | null>(null);
  const [editGroupNodes, setEditGroupNodes] = useState<Record<string, string[]>>({});

  useEffect(() => {
    void refresh();
  }, []);

  const managedNodeMap = useMemo(() => {
    const map = new Map<string, ManagedNode>();
    managedNodes.forEach((node) => map.set(node.id, node));
    return map;
  }, [managedNodes]);

  async function refresh() {
    try {
      const [nextUsers, nextNodes, nextProviders, groupsCfg] = await Promise.all([
        getJSON<UserSummary[]>("/api/users/search"),
        getJSON<ManagedNode[]>("/api/nodes"),
        getJSON<ProviderAddon[]>("/api/providers"),
        getJSON<{ groups: GroupDef[] }>("/api/config/groups").catch(() => ({ groups: [] })),
      ]);
      setUsers(nextUsers);
      setManagedNodes(nextNodes);
      setProviders(nextProviders);
      setGroupDefs(groupsCfg.groups || []);
      setMessage(`已加载 ${nextUsers.length} 个用户。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "加载用户页失败");
    }
  }

  async function updatePolicy(user: UserSummary, patch: Partial<UserSummary> & { group_nodes?: Record<string, string[]> }) {
    const mode = (patch.mode ?? user.mode) as UserSummary["mode"];
    const selectedNodes = patch.selected_nodes ?? (user.selected_nodes || []);
    const selectedProviders = patch.selected_providers ?? (user.selected_providers ?? []);
    setBusyUser(user.email);
    try {
      const body: Record<string, unknown> = {
        user: user.email,
        mode,
        nodes: selectedNodes,
        providers: selectedProviders,
      };
      if (patch.group_nodes) {
        body.group_nodes = patch.group_nodes;
      }
      await putJSON("/api/users/policy", body);
      await refresh();
      setMessage(`${user.email} 已更新。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `更新 ${user.email} 失败`);
    } finally {
      setBusyUser(null);
    }
  }

  async function handlePreview(user: UserSummary) {
    setBusyUser(user.email);
    try {
      const [yaml, nodes] = await Promise.all([
        getText(`/api/sub/preview?user=${encodeURIComponent(user.email)}`),
        getJSON<NodePreview[]>(`/api/users/preview?user=${encodeURIComponent(user.email)}`),
      ]);
      setPreview({ user: user.email, yaml, nodes });
      setMessage(`已生成 ${user.email} 的完整 config.yaml 预览。`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `预览 ${user.email} 失败`);
    } finally {
      setBusyUser(null);
    }
  }

  async function handleCopy(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      setMessage("分享地址已复制到剪贴板。");
    } catch {
      setMessage("复制失败，请手动复制分享地址。");
    }
  }

  function toggleNode(user: UserSummary, nodeID: string) {
    const selected = new Set(user.selected_nodes || []);
    if (selected.has(nodeID)) {
      selected.delete(nodeID);
    } else {
      selected.add(nodeID);
    }
    void updatePolicy(user, { selected_nodes: Array.from(selected) });
  }

  function toggleProvider(user: UserSummary, providerID: string) {
    const selected = new Set(user.selected_providers ?? []);
    if (selected.has(providerID)) {
      selected.delete(providerID);
    } else {
      selected.add(providerID);
    }
    void updatePolicy(user, { selected_providers: Array.from(selected) });
  }

  function openGroupEditor(user: UserSummary) {
    setEditingUser(user);
    setEditGroupNodes(user.group_nodes || {});
  }

  function toggleGroupNode(groupName: string, nodeName: string) {
    setEditGroupNodes((prev) => {
      const current = prev[groupName] || [];
      const set = new Set(current);
      if (set.has(nodeName)) {
        set.delete(nodeName);
      } else {
        set.add(nodeName);
      }
      return { ...prev, [groupName]: Array.from(set) };
    });
  }

  function saveGroupNodes() {
    if (!editingUser) return;
    void updatePolicy(editingUser, { group_nodes: editGroupNodes });
    setEditingUser(null);
  }

  function providerGroupDefs(user: UserSummary): GroupDef[] {
    const selected = new Set(user.selected_providers ?? []);
    return providers
      .filter((provider) => selected.has(provider.id))
      .map((provider) => ({
        name: provider.id,
        type: "url-test",
        url: "http://www.gstatic.com/generate_204",
        interval: 300,
        provider: provider.id,
      }));
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>用户</h1>
        <div className="page-actions">

          <button type="button" className="btn" onClick={() => void refresh()}>刷新列表</button>
        </div>
      </div>

      {users.length === 0 ? (
        <div className="card"><div className="card-body"><div className="empty-state">暂无用户数据，请检查 3x-ui 连接设置。</div></div></div>
      ) : (
        <div className="table-container">
          <table className="modern-table">
            <thead>
              <tr>
                <th style={{ width: 140 }}>用户</th>
                <th style={{ width: 250 }}>UUID / 密码</th>
                <th style={{ width: 420 }}>节点选择</th>
                <th style={{ width: 100 }}>分组管理</th>
                <th style={{ width: 130 }}>第三方订阅</th>
                <th style={{ width: 110 }}>模式</th>
                <th style={{ width: 60 }}>预览</th>
                <th style={{ width: 260 }}>分享</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.email}>
                  <td>
                    <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
                      <strong>{user.email}</strong>
                      <span style={{ fontSize: 12, color: "#94a3b8" }}>{user.protocol || user.remark || ""}</span>
                    </div>
                  </td>
                  <td>
                    <span style={{ fontFamily: "monospace", fontSize: 12, wordBreak: "break-all" }}>{user.uuid || user.password || "-"}</span>
                  </td>
                  <td style={{ width: 420 }}>
                    <div className="chip-group">
                      {managedNodes.length === 0 ? <span style={{ color: "#94a3b8", fontSize: 12 }}>请先去节点页添加节点</span> : null}
                      {managedNodes.map((node) => {
                        const checked = (user.selected_nodes || []).includes(node.id);
                        return (
                          <label key={node.id} className={`chip ${checked ? "checked" : ""}`}>
                            <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                              <input type="checkbox" checked={checked} onChange={() => toggleNode(user, node.id)} disabled={busyUser === user.email} style={{ margin: 0 }} />
                              <span>{node.name}</span>
                            </div>

                          </label>
                        );
                      })}
                    </div>
                  </td>
                  <td>
                    <button className="btn btn-sm" onClick={() => openGroupEditor(user)} disabled={busyUser === user.email}>
                      分组设置
                    </button>
                  </td>
                  <td>
                    {providers.length > 0 ? (
                      <div className="chip-group">
                        {providers.map((provider) => {
                          const checked = (user.selected_providers ?? []).includes(provider.id);
                          return (
                            <label key={provider.id} className={`chip ${checked ? "checked" : ""}`}>
                              <input type="checkbox" checked={checked} onChange={() => toggleProvider(user, provider.id)} disabled={busyUser === user.email} style={{ margin: 0 }} />
                              <span>{provider.name}</span>
                            </label>
                          );
                        })}
                      </div>
                    ) : (
                      <span style={{ color: "#94a3b8", fontSize: 12 }}>无</span>
                    )}
                  </td>
                  <td>
                    <div className="btn-group">
                      <button type="button" className={`btn btn-sm ${user.mode === "whitelist" ? "btn-primary" : ""}`} onClick={() => void updatePolicy(user, { mode: "whitelist" })} disabled={busyUser === user.email}>默认直连</button>
                      <button type="button" className={`btn btn-sm ${user.mode === "blacklist" ? "btn-primary" : ""}`} onClick={() => void updatePolicy(user, { mode: "blacklist" })} disabled={busyUser === user.email}>默认代理</button>
                    </div>
                  </td>
                  <td>
                    <button type="button" className="btn btn-sm" onClick={() => void handlePreview(user)} disabled={busyUser === user.email}>
                      {busyUser === user.email ? "..." : "预览"}
                    </button>
                  </td>
                  <td style={{ width: 260 }}>
                    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                      <button type="button" className="btn btn-sm" onClick={() => void handleCopy(user.share_url)} style={{ alignSelf: "flex-start" }}>复制链接</button>
                      <span style={{ fontSize: 11, color: "#94a3b8", wordBreak: "break-all", maxWidth: 260 }}>{user.share_url}</span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="message" style={{ marginTop: 16 }}>{message}</div>

      {/* Group editor modal */}
      {editingUser ? (
        <div className="modal-mask" onClick={() => setEditingUser(null)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 600 }}>
            <div className="modal-header">
              <h2>{editingUser.email} 的分组节点</h2>
              <div className="btn-group">
                <button className="btn btn-sm btn-primary" onClick={saveGroupNodes}>保存</button>
                <button className="btn btn-sm" onClick={() => setEditingUser(null)}>取消</button>
              </div>
            </div>
            <div style={{ fontSize: 13, color: "#64748b", marginBottom: 16 }}>
              已选节点：{(editingUser.selected_nodes || []).length > 0
                ? managedNodes.filter((n) => (editingUser.selected_nodes || []).includes(n.id)).map((n) => n.name).join("、")
                : "请在用户列表中勾选节点"}
            </div>

            {/* Available groups for this user */}
            {(() => {
              const baseGroups = groupDefs.filter((g) => {
                if (!g.provider) return true;
                return (editingUser.selected_providers ?? []).includes(g.provider);
              });
              const providerGroups = providerGroupDefs(editingUser);
              const seen = new Set<string>();
              const visibleGroups = [...baseGroups, ...providerGroups].filter((g) => {
                if (seen.has(g.name)) return false;
                seen.add(g.name);
                return true;
              });
              if (visibleGroups.length === 0) {
                return <div className="empty-state">暂无可用分组，请先去代理分组页添加。</div>;
              }
              const visibleGroupNames = visibleGroups.map((g) => g.name);
              return (
                <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                  {visibleGroups.map((g) => (
                    <div key={g.name} style={{ border: "1px solid #e2e8f0", borderRadius: 8, padding: 12 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
                        <strong style={{ fontSize: 14 }}>{g.name}</strong>
                        <span className="badge badge-success" style={{ fontSize: 11 }}>{g.type}</span>
                        {g.provider ? <span className="badge badge-warning" style={{ fontSize: 11 }}>use: {g.provider}</span> : null}
                      </div>
                      {g.provider ? (
                        <span style={{ fontSize: 12, color: "#94a3b8" }}>该组使用第三方 provider，无需选择节点</span>
                      ) : (
                        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                          <div>
                            <span style={{ fontSize: 11, color: "#94a3b8" }}>节点：</span>
                            <div className="chip-group" style={{ marginTop: 2 }}>
                              {(() => {
                                const selectedNodes = managedNodes.filter((n) => (editingUser.selected_nodes || []).includes(n.id));
                                return selectedNodes.length === 0 ? (
                                  <span style={{ fontSize: 12, color: "#94a3b8" }}>请在用户列表中勾选节点</span>
                                ) : (
                                  selectedNodes.map((n) => {
                                    const checked = (editGroupNodes[g.name] || []).includes(n.name);
                                    return (
                                      <label key={n.id} className={`chip ${checked ? "checked" : ""}`} style={{ fontSize: 12, padding: "3px 8px" }}>
                                        <input type="checkbox" checked={checked} onChange={() => toggleGroupNode(g.name, n.name)} style={{ margin: 0 }} />
                                        <span>{n.name}</span>
                                      </label>
                                    );
                                  })
                                );
                              })()}
                            </div>
                          </div>
                          <div>
                            <span style={{ fontSize: 11, color: "#94a3b8" }}>其他分组：</span>
                            <div className="chip-group" style={{ marginTop: 2 }}>
                              {visibleGroupNames.filter((n) => n !== g.name).map((gn) => {
                                const checked = (editGroupNodes[g.name] || []).includes(gn);
                                return (
                                  <label key={gn} className={`chip ${checked ? "checked" : ""}`} style={{ fontSize: 12, padding: "3px 8px" }}>
                                    <input type="checkbox" checked={checked} onChange={() => toggleGroupNode(g.name, gn)} style={{ margin: 0 }} />
                                    <span>{gn}</span>
                                  </label>
                                );
                              })}
                            </div>
                          </div>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              );
            })()}
          </div>
        </div>
      ) : null}

      {preview ? (
        <div className="modal-mask" onClick={() => setPreview(null)}>
          <div className="modal-content" onClick={(event) => event.stopPropagation()}>
            <div className="modal-header">
              <h2>{preview.user} 的完整预览</h2>
              <div className="btn-group">
                <button type="button" className="btn btn-sm" onClick={() => { void navigator.clipboard.writeText(preview.yaml); setMessage("配置已复制到剪贴板。"); }}>复制</button>
                <button type="button" className="btn btn-sm" onClick={() => setPreview(null)}>关闭</button>
              </div>
            </div>
            <pre className="yaml-preview">{preview.yaml}</pre>
          </div>
        </div>
      ) : null}
    </div>
  );
}
