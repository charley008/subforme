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

type PreviewState = {
  user: string;
  yaml: string;
  nodes: NodePreview[];
};

const groupTypeOptions = [
  { value: "select", label: "select" },
  { value: "url-test", label: "url-test" },
  { value: "fallback", label: "fallback" },
  { value: "load-balance", label: "load-balance" },
];

type PolicyPatch = Partial<UserSummary> & {
  group_nodes?: Record<string, string[]>;
  group_modes?: Record<string, string>;
};

export function UserPreviewPage() {
  const [users, setUsers] = useState<UserSummary[]>([]);
  const [managedNodes, setManagedNodes] = useState<ManagedNode[]>([]);
  const [providers, setProviders] = useState<ProviderAddon[]>([]);
  const [groupDefs, setGroupDefs] = useState<GroupDef[]>([]);
  const [message, setMessage] = useState("");
  const [preview, setPreview] = useState<PreviewState | null>(null);
  const [busyUser, setBusyUser] = useState<string | null>(null);

  const [editingNodesUser, setEditingNodesUser] = useState<UserSummary | null>(null);
  const [editSelectedNodes, setEditSelectedNodes] = useState<string[]>([]);

  const [editingGroupsUser, setEditingGroupsUser] = useState<UserSummary | null>(null);
  const [editGroupNodes, setEditGroupNodes] = useState<Record<string, string[]>>({});
  const [editGroupModes, setEditGroupModes] = useState<Record<string, string>>({});

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
      setMessage(error instanceof Error ? error.message : "加载用户页面失败");
    }
  }

  async function updatePolicy(user: UserSummary, patch: PolicyPatch) {
    const mode = (patch.mode ?? user.mode) as UserSummary["mode"];
    const selectedNodes = patch.selected_nodes ?? user.selected_nodes ?? [];
    const selectedProviders = patch.selected_providers ?? user.selected_providers ?? [];
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
      if (patch.group_modes) {
        body.group_modes = patch.group_modes;
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
      setMessage(`已生成 ${user.email} 的完整配置预览。`);
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

  function toggleProvider(user: UserSummary, providerID: string) {
    const selected = new Set(user.selected_providers ?? []);
    if (selected.has(providerID)) {
      selected.delete(providerID);
    } else {
      selected.add(providerID);
    }
    void updatePolicy(user, { selected_providers: Array.from(selected) });
  }

  function openNodeEditor(user: UserSummary) {
    setEditingNodesUser(user);
    setEditSelectedNodes([...(user.selected_nodes || [])]);
  }

  function toggleEditNode(nodeID: string) {
    setEditSelectedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(nodeID)) {
        next.delete(nodeID);
      } else {
        next.add(nodeID);
      }
      return Array.from(next);
    });
  }

  function saveNodeSelection() {
    if (!editingNodesUser) {
      return;
    }
    void updatePolicy(editingNodesUser, { selected_nodes: editSelectedNodes });
    setEditingNodesUser(null);
  }

  function openGroupEditor(user: UserSummary) {
    setEditingGroupsUser(user);
    setEditSelectedNodes([...(user.selected_nodes || [])]);
    setEditGroupNodes({ ...(user.group_nodes || {}) });
    setEditGroupModes({ ...(user.group_modes || {}) });
  }

  function toggleGroupNode(groupName: string, refName: string) {
    setEditGroupNodes((prev) => {
      const current = prev[groupName] || [];
      if (current.includes(refName)) {
        return { ...prev, [groupName]: current.filter((item) => item !== refName) };
      }
      return { ...prev, [groupName]: [...current, refName] };
    });
  }

  function setGroupMode(groupName: string, mode: string) {
    setEditGroupModes((prev) => ({ ...prev, [groupName]: mode }));
  }

  function moveGroupRef(groupName: string, refName: string, direction: "up" | "down") {
    setEditGroupNodes((prev) => {
      const current = [...(prev[groupName] || [])];
      const index = current.indexOf(refName);
      if (index < 0) {
        return prev;
      }
      const nextIndex = direction === "up" ? index - 1 : index + 1;
      if (nextIndex < 0 || nextIndex >= current.length) {
        return prev;
      }
      const swapped = current[nextIndex];
      current[nextIndex] = current[index];
      current[index] = swapped;
      return { ...prev, [groupName]: current };
    });
  }

  function saveGroupSettings() {
    if (!editingGroupsUser) {
      return;
    }
    void updatePolicy(editingGroupsUser, {
      group_nodes: editGroupNodes,
      group_modes: editGroupModes,
    });
    setEditingGroupsUser(null);
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

  function visibleGroupsForUser(user: UserSummary): GroupDef[] {
    const baseGroups = groupDefs.filter((group) => {
      if (!group.provider) {
        return true;
      }
      return (user.selected_providers ?? []).includes(group.provider);
    });
    const merged = [...baseGroups, ...providerGroupDefs(user)];
    const seen = new Set<string>();
    return merged.filter((group) => {
      if (seen.has(group.name)) {
        return false;
      }
      seen.add(group.name);
      return true;
    });
  }

  function selectedNodeNames(nodeIDs: string[]): string[] {
    return nodeIDs
      .map((id) => managedNodeMap.get(id)?.name)
      .filter((name): name is string => Boolean(name));
  }

  function nodeSummary(user: UserSummary): string {
    const names = selectedNodeNames(user.selected_nodes || []);
    if (names.length === 0) {
      return "未选择节点";
    }
    if (names.length <= 5) {
      return names.join(", ");
    }
    return `${names.slice(0, 5).join(", ")} 等 ${names.length} 个`;
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>用户</h1>
        <div className="page-actions">
          <button type="button" className="btn" onClick={() => void refresh()}>
            刷新列表
          </button>
        </div>
      </div>

      {users.length === 0 ? (
        <div className="card">
          <div className="card-body">
            <div className="empty-state">暂无用户数据，请检查 3x-ui 连接设置。</div>
          </div>
        </div>
      ) : (
        <div className="table-container">
          <table className="modern-table">
            <thead>
              <tr>
                <th style={{ width: 140 }}>用户</th>
                <th style={{ width: 220 }}>UUID / 密码</th>
                <th style={{ width: 220 }}>节点选择</th>
                <th style={{ width: 120 }}>分组管理</th>
                <th style={{ width: 150 }}>第三方订阅</th>
                <th style={{ width: 110 }}>模式</th>
                <th style={{ width: 70 }}>预览</th>
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
                    <span style={{ fontFamily: "monospace", fontSize: 12, wordBreak: "break-all" }}>
                      {user.uuid || user.password || "-"}
                    </span>
                  </td>
                  <td>
                    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                      <button
                        type="button"
                        className="btn btn-sm"
                        onClick={() => openNodeEditor(user)}
                        disabled={busyUser === user.email}
                      >
                        节点设置
                      </button>
                      <span style={{ fontSize: 12, color: "#94a3b8", lineHeight: 1.5 }}>{nodeSummary(user)}</span>
                    </div>
                  </td>
                  <td>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => openGroupEditor(user)}
                      disabled={busyUser === user.email}
                    >
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
                              <input
                                type="checkbox"
                                checked={checked}
                                onChange={() => toggleProvider(user, provider.id)}
                                disabled={busyUser === user.email}
                                style={{ margin: 0 }}
                              />
                              <span>{provider.name || provider.id}</span>
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
                      <button
                        type="button"
                        className={`btn btn-sm ${user.mode === "whitelist" ? "btn-primary" : ""}`}
                        onClick={() => void updatePolicy(user, { mode: "whitelist" })}
                        disabled={busyUser === user.email}
                      >
                        默认直连
                      </button>
                      <button
                        type="button"
                        className={`btn btn-sm ${user.mode === "blacklist" ? "btn-primary" : ""}`}
                        onClick={() => void updatePolicy(user, { mode: "blacklist" })}
                        disabled={busyUser === user.email}
                      >
                        默认代理
                      </button>
                    </div>
                  </td>
                  <td>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={() => void handlePreview(user)}
                      disabled={busyUser === user.email}
                    >
                      {busyUser === user.email ? "..." : "预览"}
                    </button>
                  </td>
                  <td style={{ width: 260 }}>
                    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                      <button
                        type="button"
                        className="btn btn-sm"
                        onClick={() => void handleCopy(user.share_url)}
                        style={{ alignSelf: "flex-start" }}
                      >
                        复制链接
                      </button>
                      <span style={{ fontSize: 11, color: "#94a3b8", wordBreak: "break-all", maxWidth: 260 }}>
                        {user.share_url}
                      </span>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="message" style={{ marginTop: 16 }}>
        {message}
      </div>

      {editingNodesUser ? (
        <div className="modal-mask" onClick={() => setEditingNodesUser(null)}>
          <div className="modal-content" onClick={(event) => event.stopPropagation()} style={{ maxWidth: 720 }}>
            <div className="modal-header">
              <h2>{editingNodesUser.email} 的节点设置</h2>
              <div className="btn-group">
                <button type="button" className="btn btn-sm btn-primary" onClick={saveNodeSelection}>
                  保存
                </button>
                <button type="button" className="btn btn-sm" onClick={() => setEditingNodesUser(null)}>
                  取消
                </button>
              </div>
            </div>
            <div style={{ fontSize: 13, color: "#94a3b8", marginBottom: 16 }}>
              已选 {editSelectedNodes.length} 个节点
            </div>
            {managedNodes.length === 0 ? (
              <div className="empty-state">请先去节点页面添加节点。</div>
            ) : (
              <div className="chip-group">
                {managedNodes.map((node) => {
                  const checked = editSelectedNodes.includes(node.id);
                  return (
                    <label key={node.id} className={`chip ${checked ? "checked" : ""}`}>
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => toggleEditNode(node.id)}
                        style={{ margin: 0 }}
                      />
                      <span>{node.name}</span>
                    </label>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      ) : null}

      {editingGroupsUser ? (
        <div className="modal-mask" onClick={() => setEditingGroupsUser(null)}>
          <div className="modal-content" onClick={(event) => event.stopPropagation()} style={{ maxWidth: 760 }}>
            <div className="modal-header">
              <h2>{editingGroupsUser.email} 的分组设置</h2>
              <div className="btn-group">
                <button type="button" className="btn btn-sm btn-primary" onClick={saveGroupSettings}>
                  保存
                </button>
                <button type="button" className="btn btn-sm" onClick={() => setEditingGroupsUser(null)}>
                  取消
                </button>
              </div>
            </div>
            <div style={{ fontSize: 13, color: "#94a3b8", marginBottom: 16 }}>
              已选节点：{selectedNodeNames(editSelectedNodes).join(", ") || "未选择节点"}
            </div>

            {visibleGroupsForUser(editingGroupsUser).length === 0 ? (
              <div className="empty-state">暂无可用分组，请先去代理分组页面添加。</div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                {visibleGroupsForUser(editingGroupsUser).map((group) => {
                  const selectedNodeRefs = selectedNodeNames(editSelectedNodes);
                  const currentRefs = editGroupNodes[group.name] || [];
                  const currentMode = editGroupModes[group.name] || group.type || "select";
                  const siblingGroups = visibleGroupsForUser(editingGroupsUser)
                    .map((item) => item.name)
                    .filter((name) => name !== group.name);
                  const orderedRefs = currentRefs.filter((ref) => selectedNodeRefs.includes(ref) || siblingGroups.includes(ref));

                  return (
                    <div
                      key={group.name}
                      style={{
                        border: "1px solid var(--gray-200)",
                        borderRadius: 8,
                        padding: 12,
                        background: "var(--gray-50)",
                      }}
                    >
                      <div
                        style={{
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "space-between",
                          gap: 12,
                          marginBottom: 10,
                          flexWrap: "wrap",
                        }}
                      >
                        <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                          <strong style={{ fontSize: 14 }}>{group.name}</strong>
                          {group.provider ? (
                            <span className="badge badge-warning" style={{ fontSize: 11 }}>
                              use: {group.provider}
                            </span>
                          ) : null}
                        </div>
                        <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, color: "var(--gray-500)" }}>
                          <span>模式</span>
                          <select
                            value={currentMode}
                            onChange={(event) => setGroupMode(group.name, event.target.value)}
                            style={{
                              minWidth: 140,
                              height: 32,
                              borderRadius: 8,
                              border: "1px solid var(--gray-300)",
                              background: "var(--white)",
                              color: "var(--gray-900)",
                              padding: "0 10px",
                            }}
                          >
                            {groupTypeOptions.map((option) => (
                              <option key={option.value} value={option.value}>
                                {option.label}
                              </option>
                            ))}
                          </select>
                        </label>
                      </div>

                      {group.provider ? (
                        <div style={{ fontSize: 12, color: "var(--gray-500)" }}>该分组引用第三方订阅源，不需要手动选择节点。</div>
                      ) : (
                        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                          <div>
                            <div style={{ fontSize: 12, color: "var(--gray-500)", marginBottom: 6 }}>节点引用</div>
                            {selectedNodeRefs.length === 0 ? (
                              <div style={{ fontSize: 12, color: "var(--gray-500)" }}>请先在“节点设置”里选择节点。</div>
                            ) : (
                              <div className="chip-group">
                                {selectedNodeRefs.map((nodeName) => {
                                  const checked = currentRefs.includes(nodeName);
                                  return (
                                    <label key={nodeName} className={`chip ${checked ? "checked" : ""}`}>
                                      <input
                                        type="checkbox"
                                        checked={checked}
                                        onChange={() => toggleGroupNode(group.name, nodeName)}
                                        style={{ margin: 0 }}
                                      />
                                      <span>{nodeName}</span>
                                    </label>
                                  );
                                })}
                              </div>
                            )}
                          </div>

                          <div>
                            <div style={{ fontSize: 12, color: "var(--gray-500)", marginBottom: 6 }}>其他分组引用</div>
                            {siblingGroups.length === 0 ? (
                              <div style={{ fontSize: 12, color: "var(--gray-500)" }}>暂无其他分组可引用。</div>
                            ) : (
                              <div className="chip-group">
                                {siblingGroups.map((groupName) => {
                                  const checked = currentRefs.includes(groupName);
                                  return (
                                    <label key={groupName} className={`chip ${checked ? "checked" : ""}`}>
                                      <input
                                        type="checkbox"
                                        checked={checked}
                                        onChange={() => toggleGroupNode(group.name, groupName)}
                                        style={{ margin: 0 }}
                                      />
                                      <span>{groupName}</span>
                                    </label>
                                  );
                                })}
                              </div>
                            )}
                          </div>

                          {currentMode === "fallback" ? (
                            <div>
                              <div style={{ fontSize: 12, color: "var(--gray-500)", marginBottom: 6 }}>fallback 顺序</div>
                              {orderedRefs.length === 0 ? (
                                <div style={{ fontSize: 12, color: "var(--gray-500)" }}>请先勾选要参与 fallback 的节点或分组。</div>
                              ) : (
                                <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                                  {orderedRefs.map((refName, index) => (
                                    <div
                                      key={refName}
                                      style={{
                                        display: "flex",
                                        alignItems: "center",
                                        justifyContent: "space-between",
                                        gap: 12,
                                        padding: "8px 10px",
                                        border: "1px solid var(--gray-200)",
                                        borderRadius: 8,
                                        background: "var(--white)",
                                      }}
                                    >
                                      <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0 }}>
                                        <span
                                          style={{
                                            width: 22,
                                            height: 22,
                                            borderRadius: 999,
                                            background: "var(--primary-bg)",
                                            color: "var(--primary-dark)",
                                            display: "inline-flex",
                                            alignItems: "center",
                                            justifyContent: "center",
                                            fontSize: 12,
                                            fontWeight: 600,
                                            flexShrink: 0,
                                          }}
                                        >
                                          {index + 1}
                                        </span>
                                        <span style={{ fontSize: 13, color: "var(--gray-800)", wordBreak: "break-all" }}>{refName}</span>
                                      </div>
                                      <div className="btn-group" style={{ flexWrap: "nowrap" }}>
                                        <button
                                          type="button"
                                          className="btn btn-sm"
                                          onClick={() => moveGroupRef(group.name, refName, "up")}
                                          disabled={index === 0}
                                        >
                                          上移
                                        </button>
                                        <button
                                          type="button"
                                          className="btn btn-sm"
                                          onClick={() => moveGroupRef(group.name, refName, "down")}
                                          disabled={index === orderedRefs.length - 1}
                                        >
                                          下移
                                        </button>
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              )}
                            </div>
                          ) : null}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      ) : null}

      {preview ? (
        <div className="modal-mask" onClick={() => setPreview(null)}>
          <div className="modal-content" onClick={(event) => event.stopPropagation()}>
            <div className="modal-header">
              <h2>{preview.user} 的完整预览</h2>
              <div className="btn-group">
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => {
                    void navigator.clipboard.writeText(preview.yaml);
                    setMessage("配置已复制到剪贴板。");
                  }}
                >
                  复制
                </button>
                <button type="button" className="btn btn-sm" onClick={() => setPreview(null)}>
                  关闭
                </button>
              </div>
            </div>
            <pre className="yaml-preview">{preview.yaml}</pre>
          </div>
        </div>
      ) : null}
    </div>
  );
}
